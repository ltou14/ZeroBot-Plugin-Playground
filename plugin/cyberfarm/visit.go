package cyberfarm

import (
    "fmt"
    "strconv"
    "time"

    "github.com/FloatTech/zbputils/ctxext"   // 新增导入
    zero "github.com/wdvxdr1123/ZeroBot"
    "github.com/wdvxdr1123/ZeroBot/message"
)

func init() {
    // 拜访指令：拜访 [@某人] [田地编号]
    engine.OnRegex(`^拜访.*?(\d)?$`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
        victimID := extractFirstAt(ctx)
        if victimID == 0 {
            ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("请 @ 你要拜访的人"))
            return
        }
        fieldIndex := -1
        // 如果正则匹配到了数字，则作为田地编号
        if len(ctx.State["regex_matched"].([]string)) > 1 && ctx.State["regex_matched"].([]string)[1] != "" {
            fieldIndex, _ = strconv.Atoi(ctx.State["regex_matched"].([]string)[1])
            fieldIndex-- // 转为0索引
            if fieldIndex < 0 || fieldIndex >= 4 {
                ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("田地编号必须为1-4"))
                return
            }
        }

        // 获取对方农田数据
        fields, err := farmdb.getFieldsByUser(victimID)
        if err != nil {
            ctx.SendChain(message.Text("[ERROR]:", err))
            return
        }
        // 如果对方没有农田，初始化默认
        if fields[0] == nil {
            defaultFields := [4]*singleField{
                {Crop: "", Water: 50, Fertilizer: 50},
                {Crop: "", Water: 50, Fertilizer: 50},
                {Crop: "", Water: 50, Fertilizer: 50},
                {Crop: "", Water: 50, Fertilizer: 50},
            }
            rowToSave := farmdb.buildFullRow(victimID, defaultFields)
            if err = farmdb.insertOrUpdateField(rowToSave); err != nil {
                ctx.SendChain(message.Text("[ERROR]: 初始化对方农田失败:", err))
                return
            }
            fields, _ = farmdb.getFieldsByUser(victimID)
        }

        if fieldIndex == -1 {
            // 显示对方完整农场图片
            showFullFarm(ctx, victimID, fields)
        } else {
            // 显示指定田块的详细信息（文本）
            f := fields[fieldIndex]
            if f == nil || f.Crop == "" {
                ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("对方这块田没有种植作物"))
                return
            }
            msg := fmt.Sprintf("【%s 的田地%d】\n作物：%s\n水分：%.1f\n肥度：%.1f\n可收获次数：%d/%d",
                ctx.CardOrNickName(victimID), fieldIndex+1, f.Crop, f.Water, f.Fertilizer, f.HarvestCount, f.TotalHarvests)
            // 计算剩余时间
            now := time.Now().Unix()
            growthTime := f.GrowthDuration * int64(f.HarvestCount)
            elapsed := now - f.PlantTime - growthTime
            remain := f.GrowthDuration - elapsed
            if remain < 0 {
                remain = 0
            }
            if f.HarvestCount >= f.TotalHarvests {
                msg += "\n状态：已枯竭"
            } else if elapsed >= f.GrowthDuration {
                msg += "\n状态：可收获"
            } else {
                msg += "\n剩余时间：" + formatDuration(remain)
            }
            ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(msg))
        }
    })
}

// showFullFarm 显示对方完整农场图片
func showFullFarm(ctx *zero.Ctx, victimID int64, fields [4]*singleField) {
    userName := ctx.CardOrNickName(victimID)
    imgBase64, err := generateFarmImage(victimID, userName, fields)
    if err != nil {
        ctx.SendChain(message.Text("[ERROR]: 生成图片失败", err))
        return
    }
    ctx.SendChain(message.Image("base64://" + imgBase64))
}