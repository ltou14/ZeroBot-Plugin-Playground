package cyberfarm

import (
	"math/rand"
	"strconv"
	"time"

    sqlite "github.com/FloatTech/sqlite"
    "github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

func init() {
    engine.OnRegex(`^帮忙浇水.*?(\d)$`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
        ownerID := extractFirstAt(ctx)
        if ownerID == 0 {
            ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("请 @ 你要帮忙的人"))
            return
        }
        fieldIndex, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[1])
        fieldIndex-- // 转为0索引
        if fieldIndex < 0 || fieldIndex >= 4 {
            ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("田地编号必须为1-4"))
            return
        }
        handleHelp(ctx, "water", ownerID, fieldIndex)
    })

    // 帮忙施肥
    engine.OnRegex(`^帮忙施肥.*?(\d)$`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
        ownerID := extractFirstAt(ctx)
        if ownerID == 0 {
            ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("请 @ 你要帮忙的人"))
            return
        }
        fieldIndex, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[1])
        fieldIndex-- // 转为0索引
        if fieldIndex < 0 || fieldIndex >= 4 {
            ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("田地编号必须为1-4"))
            return
        }
        handleHelp(ctx, "fertilizer", ownerID, fieldIndex)
    })
}

// 修改 handleHelp 函数签名，增加 ownerID 和 fieldIndex 参数
func handleHelp(ctx *zero.Ctx, helpType string, ownerID int64, fieldIndex int) {
    helperID := ctx.Event.UserID

    // 移除原先从 ctx.State 解析 ownerID 和 fieldIndex 的代码，现在直接使用参数

    // 不能帮自己
    if helperID == ownerID {
        ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("不能帮自己的田"))
        return
    }

    // 获取对方田地数据
    fields, err := farmdb.getFieldsByUser(ownerID)
    if err != nil {
        ctx.SendChain(message.Text("[ERROR]:", err))
        return
    }
    if fields[0] == nil {
        ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("对方还没有农田"))
        return
    }
    f := fields[fieldIndex]
    if f == nil || f.Crop == "" {
        ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("对方这块田没有种植作物"))
        return
    }

    // 检查是否已帮忙过（每人每日对同一块田只能帮忙一次）
    startOfDay := time.Now().Truncate(24 * time.Hour).Unix()
    var count struct {
        Count int
    }
    err = farmdb.Sqlite.Query(`
        SELECT COUNT(*) as count FROM farm_help_log
        WHERE helper_id = ? AND owner_id = ? AND field_index = ? AND help_type = ? AND help_time >= ?`,
        &count, helperID, ownerID, fieldIndex+1, helpType, startOfDay)
    if err == nil && count.Count > 0 {
        ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("你今天已经帮过这块田了"))
        return
    }
    if err != nil && err != sqlite.ErrNullResult {
        ctx.SendChain(message.Text("[ERROR]:", err))
        return
    }

    // 执行浇水或施肥
    var inc float64
    if helpType == "water" {
        inc = 5 + rand.Float64()*10
        f.Water += inc
        if f.Water > 100 {
            f.Water = 100
        }
    } else {
        inc = 3 + rand.Float64()*8
        f.Fertilizer += inc
        if f.Fertilizer > 100 {
            f.Fertilizer = 100
        }
    }

   // 保存田地状态
    rowToSave := farmdb.buildFullRow(ownerID, fields)
    if err = farmdb.insertOrUpdateField(rowToSave); err != nil {
        ctx.SendChain(message.Text("[ERROR]: 保存失败:", err))
        return
    }

    // 插入帮忙记录
    _, err = farmdb.Exec(`
        INSERT INTO farm_help_log (helper_id, owner_id, field_index, help_type, help_time, rewarded)
        VALUES (?, ?, ?, ?, ?, 0)`,
        helperID, ownerID, fieldIndex+1, helpType, time.Now().Unix())
    if err != nil {
        ctx.SendChain(message.Text("[ERROR]: 记录帮忙失败:", err))
        return
    }

    // 发送提示
    action := "浇水"
    if helpType == "fertilizer" {
        action = "施肥"
    }
    ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("帮忙", action, "成功！", f.Crop, "的", action, "增加了", strconv.FormatFloat(inc, 'f', 1, 64)))
}