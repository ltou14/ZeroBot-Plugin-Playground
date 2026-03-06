// Package cyberfarm 农场插件 - 批量操作
package cyberfarm

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

func init() {
	// 全部浇水
	engine.OnFullMatch("全部浇水", zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		uid := ctx.Event.UserID
		fields, err := farmdb.getFieldsByUser(uid)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if fields[0] == nil {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("你还没有开垦农田，请先种植吧"))
			return
		}

		modified := false
		msg := "浇水结果：\n"
		for i, f := range fields {
			if f == nil || f.Crop == "" {
				msg += "田地" + strconv.Itoa(i+1) + "：没有种植作物\n"
				continue
			}
			inc := 5 + rand.Float64()*10
			f.Water += inc
			if f.Water > 100 {
				f.Water = 100
			}
			modified = true
			msg += "田地" + strconv.Itoa(i+1) + "：" + f.Crop + " 水分+" + strconv.FormatFloat(inc, 'f', 1, 64) + "\n"
		}

		if modified {
			rowToSave := farmdb.buildFullRow(uid, fields)
			if err = farmdb.insertOrUpdateField(rowToSave); err != nil {
				ctx.SendChain(message.Text("[ERROR]: 保存失败:", err))
				return
			}
		}
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(msg))
	})

	// 全部施肥
	engine.OnFullMatch("全部施肥", zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		uid := ctx.Event.UserID
		fields, err := farmdb.getFieldsByUser(uid)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if fields[0] == nil {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("你还没有开垦农田，请先种植吧"))
			return
		}

		modified := false
		msg := "施肥结果：\n"
		for i, f := range fields {
			if f == nil || f.Crop == "" {
				msg += "田地" + strconv.Itoa(i+1) + "：没有种植作物\n"
				continue
			}
			inc := 3 + rand.Float64()*8
			f.Fertilizer += inc
			if f.Fertilizer > 100 {
				f.Fertilizer = 100
			}
			modified = true
			msg += "田地" + strconv.Itoa(i+1) + "：" + f.Crop + " 肥度+" + strconv.FormatFloat(inc, 'f', 1, 64) + "\n"
		}

		if modified {
			rowToSave := farmdb.buildFullRow(uid, fields)
			if err = farmdb.insertOrUpdateField(rowToSave); err != nil {
				ctx.SendChain(message.Text("[ERROR]: 保存失败:", err))
				return
			}
		}
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(msg))
	})

	// 全部收获
	engine.OnFullMatch("全部收获", zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		uid := ctx.Event.UserID
		fields, err := farmdb.getFieldsByUser(uid)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if fields[0] == nil {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("你还没有开垦农田，请先种植吧"))
			return
		}

		totalHarvested := 0
		msg := "收获结果：\n"
		modified := false
		var rewardInfo string // 新增：用于收集奖励信息
		for i, f := range fields {
			if f == nil || f.Crop == "" {
				msg += "田地" + strconv.Itoa(i+1) + "：没有种植作物\n"
				continue
			}
			cfg := GetCropConfig(f.Crop)
			if cfg == nil {
				msg += "田地" + strconv.Itoa(i+1) + "：未知作物\n"
				continue
			}
			now := time.Now().Unix()
			growthTime := f.GrowthDuration * int64(f.HarvestCount)
			if now-f.PlantTime-growthTime < f.GrowthDuration {
				msg += "田地" + strconv.Itoa(i+1) + "：" + f.Crop + " 还未成熟\n"
				continue
			}
			if f.HarvestCount >= f.TotalHarvests {
				msg += "田地" + strconv.Itoa(i+1) + "：" + f.Crop + " 已枯竭\n"
				continue
			}

			// 计算产量
			yield := cfg.Yield
			if f.Water > 70 {
				yield = int(float64(yield) * 1.2)
			} else if f.Water < 30 {
				yield = int(float64(yield) * 0.8)
			}
			if f.Fertilizer > 70 {
				yield = int(float64(yield) * 1.2)
			} else if f.Fertilizer < 30 {
				yield = int(float64(yield) * 0.8)
			}
			
			// 检查旁边是否有成熟的幻昙花，有则产量+1
			for j := 0; j < 4; j++ {
				if j == i {
					continue
				}
				if fields[j] != nil && fields[j].Crop == "幻昙花" {
					// 检查幻昙花是否成熟
					now := time.Now().Unix()
					growthTime := fields[j].GrowthDuration * int64(fields[j].HarvestCount)
					if now-fields[j].PlantTime-growthTime >= fields[j].GrowthDuration && fields[j].HarvestCount < fields[j].TotalHarvests {
						yield++
						break // 只加一次
					}
				}
			}
			
			// 减去被偷的数量
			stolenAmount, err := farmdb.getStolenAmount(uid, i+1, f.PlantTime)
			if err == nil {
				yield -= stolenAmount
			}
			
			if yield < 1 {
				yield = 1
			}

			// 更新背包
			if err = farmdb.updateCrop(uid, f.Crop, yield); err != nil {
				msg += "田地" + strconv.Itoa(i+1) + "：收获 " + f.Crop + " 失败（背包错误）\n"
				continue
			}

			// 帮忙奖励（替换原有一大段逻辑）
			rewardInfo += awardHelpersForHarvest(ctx, farmdb, uid, i, f.Crop, f.PlantTime)

			// 更新田地状态
			f.HarvestCount++
			f.Water -= 10
			if f.Water < 0 {
				f.Water = 0
			}
			f.Fertilizer -= 5
			if f.Fertilizer < 0 {
				f.Fertilizer = 0
			}
			if f.HarvestCount >= f.TotalHarvests {
				f.Crop = ""
				f.Water = 0
				f.Fertilizer = 0
				f.PlantTime = 0
				f.HarvestCount = 0
				f.TotalHarvests = 0
				f.GrowthDuration = 0
			}
			modified = true
			totalHarvested += yield
			msg += "田地" + strconv.Itoa(i+1) + "：收获 " + f.Crop + " x" + strconv.Itoa(yield) + "\n"
		}

		if modified {
			rowToSave := farmdb.buildFullRow(uid, fields)
			if err = farmdb.insertOrUpdateField(rowToSave); err != nil {
				ctx.SendChain(message.Text("[ERROR]: 保存失败:", err))
				return
			}
		}
		if totalHarvested > 0 {
			msg += "总计收获 " + strconv.Itoa(totalHarvested) + " 个作物"
		}
		msg += rewardInfo // 添加奖励信息
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(msg))
	})
}
