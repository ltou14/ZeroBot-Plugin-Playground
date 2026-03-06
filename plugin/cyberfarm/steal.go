package cyberfarm

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/FloatTech/zbputils/ctxext" // 确保已导入
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

func init() {
	// 偷菜指令：偷菜 [@某人] [田地编号]
	engine.OnRegex(`^偷菜.*?(\d)$`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		thiefID := ctx.Event.UserID
		victimID := extractFirstAt(ctx)
		if victimID == 0 {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("请 @ 你要偷的人"))
			return
		}
		fieldIndex, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[1])
		fieldIndex-- // 转为0索引
		if fieldIndex < 0 || fieldIndex >= 4 {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("田地编号必须为1-4"))
			return
		}

		// 不能偷自己
		if thiefID == victimID {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("不能偷自己的田"))
			return
		}

		// 检查偷菜次数
		count, err := farmdb.getTodayStealCount(thiefID)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if count >= MaxStealPerDay {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("你今天已经偷了", MaxStealPerDay, "次，不能再偷了！"))
			return
		}

		// 获取对方田地数据
		fields, err := farmdb.getFieldsByUser(victimID)
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
		// 幻昙花不可被偷
		if f.Crop == "幻昙花" {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("幻昙花是奇迹之花，不可被偷取"))
			return
		}

		// 检查是否可收获（成熟）
		now := time.Now().Unix()
		growthTime := f.GrowthDuration * int64(f.HarvestCount)
		if now-f.PlantTime-growthTime < f.GrowthDuration {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("作物还未成熟，不能偷"))
			return
		}
		if f.HarvestCount >= f.TotalHarvests {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("这块田已经枯竭，不能偷"))
			return
		}

		// 随机偷取数量
		cfg := GetCropConfig(f.Crop)
		maxSteal := int(float64(cfg.Yield) * 0.3)
		if maxSteal < 1 {
			maxSteal = 1
		}
		stealAmount := rand.Intn(maxSteal) + 1

		// 事务处理 - 由于原始包没有提供 Begin 方法，我们直接执行操作
		// 注意：这样会失去事务的原子性，但在这个场景下影响不大

		// 增加小偷（直接从田里偷，不扣除主人背包）
		if err = farmdb.updateCrop(thiefID, f.Crop, stealAmount); err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		// 插入偷菜记录
		log := &farmStealLog{
			ThiefID:    thiefID,
			VictimID:   victimID,
			FieldIndex: fieldIndex + 1,
			Crop:       f.Crop,
			Amount:     stealAmount,
			StealTime:  now,
			Discovered: false,
			Punished:   false,
		}
		if err = farmdb.insertStealLog(log); err != nil {
			// 回滚
			farmdb.updateCrop(thiefID, f.Crop, -stealAmount)
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}

		// 发送提示
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("偷窃成功！你获得了", stealAmount, "个", f.Crop))
		// 私聊通知主人
		ctx.SendPrivateMessage(victimID, message.Text("你的", f.Crop, "被", ctx.CardOrNickName(thiefID), "偷走了", stealAmount, "个，10分钟内可发现小偷"))
	})
}
