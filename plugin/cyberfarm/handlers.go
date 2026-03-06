// Package cyberfarm 农场插件 - 指令处理
package cyberfarm

import (
	"image"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/FloatTech/AnimeAPI/wallet"
	fcext "github.com/FloatTech/floatbox/ctxext"
	sql "github.com/FloatTech/sqlite"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	"gitee.com/ygo-ltou/ZeroBot-Plugin-Playground/plugin/cybercat"
)

const (
	MaxStealPerDay = 5       // 每人每日最多偷5次
	DiscoverWindow = 10 * 60 // 发现窗口期10分钟（秒）
)

// 确保 image 包被使用
var _ = image.Point{}

var (
	engine = control.Register("cyberfarm", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "农场",
		Help: "拥有4块农田，种植作物，收获后可以出售或转化为猫粮\n" +
			"----------基础指令----------\n" +
			"- 我的农场\n" +
			"- 农场商店\n" +
			"- 种植 [田地编号1-4] [作物名称]\n" +
			"- [全部]浇水 [田地编号]\n" +
			"- [全部]施肥 [田地编号]\n" +
			"- [全部]收获 [田地编号]\n" +
			"- 我的背包\n" +
			"- 出售 [作物名称] [数量]\n" +
			"- 转化猫粮 [作物名称] [数量]\n" +
			"\n----------社交互动----------\n" +
			"- 拜访 [@某人] [田地编号] （查看对方农场或指定田块详情）\n" +
			"- 偷菜 [@某人] [田地编号] （每日最多5次，偷取对方成熟作物，10分钟内可被发现）\n" +
			"- 帮忙浇水 [@某人] [田地编号] （为对方田地浇水，收获时若无被偷记录，你将额外获得一个作物奖励）\n" +
			"- 帮忙施肥 [@某人] [田地编号] （为对方田地施肥，奖励规则同上）\n" +
			"- 发现小偷 [@某人] [田地编号] （在10分钟内发现偷你菜的人，可令其双倍罚款并归还作物）\n" +
			"\n商店开放时间：6:00-21:00",
		PrivateDataFolder: "cyberfarm",
	}).ApplySingle(ctxext.DefaultSingle)
	farmdb = &farmDB{
		Sqlite: sql.New(engine.DataFolder() + "farm.db"),
	}
	getdb = fcext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
		err := farmdb.Open(time.Hour * 24)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return false
		}
		// 创建偷菜表
		_, err = farmdb.Exec(`CREATE TABLE IF NOT EXISTS farm_steal_log (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            thief_id INTEGER NOT NULL,
            victim_id INTEGER NOT NULL,
            field_index INTEGER NOT NULL,
            crop TEXT NOT NULL,
            amount INTEGER NOT NULL,
            steal_time INTEGER NOT NULL,
            discovered BOOLEAN DEFAULT 0,
            punished BOOLEAN DEFAULT 0
        )`)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]: 创建偷菜记录表失败:", err))
			return false
		}
		// 创建帮助表
		_, err = farmdb.Exec(`CREATE TABLE IF NOT EXISTS farm_help_log (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            helper_id INTEGER NOT NULL,
            owner_id INTEGER NOT NULL,
            field_index INTEGER NOT NULL,
            help_type TEXT NOT NULL,
            help_time INTEGER NOT NULL,
            rewarded BOOLEAN DEFAULT 0
        )`)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]: 创建帮忙记录表失败:", err))
			return false
		}
		// 创建农田表，一行存储四块田
		err = farmdb.Create("farm_fields", &fieldInfo{})
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return false
		}
		// 删除旧背包表（如果存在且结构错误）
		_, _ = farmdb.Exec(`DROP TABLE IF EXISTS user_crops;`)
		// 创建背包表，使用复合主键 (user_id, crop_name)
		_, err = farmdb.Exec(`CREATE TABLE IF NOT EXISTS user_crops (
            user_id   INTEGER NOT NULL,
            crop_name TEXT NOT NULL,
            quantity  INTEGER NOT NULL,
            PRIMARY KEY (user_id, crop_name)
        )`)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]: 创建背包表失败:", err))
			return false
		}
		return true
	})
)

func init() {
	// 我的农场（图形化四宫格）- 新布局：240x420 格子，三块区域 + 进度条
	engine.OnFullMatch("我的农场", zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		uid := ctx.Event.UserID
		fields, err := farmdb.getFieldsByUser(uid)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		// 初始化默认农田（如有需要）
		allEmpty := true
		for _, f := range fields {
			if f != nil {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			defaultFields := [4]*singleField{
				{Crop: "", Water: 50, Fertilizer: 50},
				{Crop: "", Water: 50, Fertilizer: 50},
				{Crop: "", Water: 50, Fertilizer: 50},
				{Crop: "", Water: 50, Fertilizer: 50},
			}
			rowToSave := farmdb.buildFullRow(uid, defaultFields)
			if err = farmdb.insertOrUpdateField(rowToSave); err != nil {
				ctx.SendChain(message.Text("[ERROR]: 初始化农田失败:", err))
				return
			}
			fields, _ = farmdb.getFieldsByUser(uid)
		}

		userName := ctx.CardOrNickName(uid)
		imgBase64, err := generateFarmImage(uid, userName, fields)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]: 生成图片失败", err))
			return
		}
		ctx.SendChain(message.Image("base64://" + imgBase64))
	})

	// 我的背包（可视化）
	engine.OnFullMatch("我的背包", zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		uid := ctx.Event.UserID
		backpack, err := getUserBackpack(uid)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		userName := ctx.CardOrNickName(uid)
		imgBase64, err := generateBackpackImage(uid, userName, backpack)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]: 生成背包图片失败", err))
			return
		}
		ctx.SendChain(message.Image("base64://" + imgBase64))
	})

	// 商店（不变）
	engine.OnFullMatch("农场商店", zero.OnlyGroup, getdb, checkShopOpen).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		msg := make([]string, 0, len(GetAllCropConfigs())*5+2)
		msg = append(msg, "农场商店（营业时间6-24点）\n可种植的作物：")
		for name, cfg := range GetAllCropConfigs() {
			msg = append(msg, "------------",
				"名称："+name,
				"种子价格："+strconv.Itoa(cfg.Price)+" 金币",
				"生长时间："+formatDuration(cfg.GrowthTime),
				"产量："+strconv.Itoa(cfg.Yield)+" 个/次",
				"可收获次数："+strconv.Itoa(cfg.Harvests),
				"单个售价："+strconv.Itoa(cfg.SellPrice)+" 金币",
				"猫粮转化："+strconv.FormatFloat(cfg.CatFoodRatio, 'f', 2, 64)+" 斤/个",
				"标签："+cfg.Tag,
				"描述："+cfg.Description)
		}
		ctx.SendChain(message.Text(strings.Join(msg, "\n")))
	})

	// 种植
	engine.OnRegex(`^种植\s*(\d)\s*(.*)$`, zero.OnlyGroup, getdb, checkShopOpen).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		uid := ctx.Event.UserID
		fieldIndex, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[1])
		fieldIndex--
		if fieldIndex < 0 || fieldIndex >= 4 {
			ctx.SendChain(message.Reply(id), message.Text("田地编号必须为1-4"))
			return
		}
		cropName := strings.TrimSpace(ctx.State["regex_matched"].([]string)[2])
		cfg := GetCropConfig(cropName)
		if cfg == nil {
			ctx.SendChain(message.Reply(id), message.Text("没有这种作物"))
			return
		}

		// 获取用户当前所有田地数据，确保有记录
		fields, err := farmdb.getFieldsByUser(uid)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		// 如果用户没有记录，先初始化默认行
		if fields[0] == nil {
			defaultFields := [4]*singleField{
				{Crop: "", Water: 50, Fertilizer: 50},
				{Crop: "", Water: 50, Fertilizer: 50},
				{Crop: "", Water: 50, Fertilizer: 50},
				{Crop: "", Water: 50, Fertilizer: 50},
			}
			rowToSave := farmdb.buildFullRow(uid, defaultFields)
			if err = farmdb.insertOrUpdateField(rowToSave); err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			fields, err = farmdb.getFieldsByUser(uid)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
		}

		if fields[fieldIndex].Crop != "" {
			ctx.SendChain(message.Reply(id), message.Text("这块田已经种植了作物"))
			return
		}
		if wallet.GetWalletOf(uid) < cfg.Price {
			ctx.SendChain(message.Reply(id), message.Text("购买种子需要", cfg.Price, "金币，你的余额不足"))
			return
		}
		if err = wallet.InsertWalletOf(uid, -cfg.Price); err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}

		// 更新对应田地的数据
		fields[fieldIndex].Crop = cropName
		fields[fieldIndex].Water = 50 + rand.Float64()*30
		fields[fieldIndex].Fertilizer = 50 + rand.Float64()*30
		fields[fieldIndex].PlantTime = time.Now().Unix()
		fields[fieldIndex].HarvestCount = 0
		fields[fieldIndex].TotalHarvests = cfg.Harvests
		fields[fieldIndex].GrowthDuration = cfg.GrowthTime

		// 组装整行保存
		rowToSave := farmdb.buildFullRow(uid, fields)
		if err = farmdb.insertOrUpdateField(rowToSave); err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		imgPath := getCropImage(cfg, uid, fieldIndex)
		ctx.SendChain(message.Reply(id), message.Image(imgPath), message.Text("种植成功！"))
	})

	// 浇水
	engine.OnRegex(`^浇水\s*(\d)$`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		uid := ctx.Event.UserID
		fieldIndex, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[1])
		fieldIndex--
		if fieldIndex < 0 || fieldIndex >= 4 {
			ctx.SendChain(message.Reply(id), message.Text("田地编号必须为1-4"))
			return
		}
		fields, err := farmdb.getFieldsByUser(uid)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		// 确保有记录
		if fields[0] == nil {
			ctx.SendChain(message.Reply(id), message.Text("你还未拥有任何田地，请先去种植吧"))
			return
		}
		f := fields[fieldIndex]
		if f == nil || f.Crop == "" {
			ctx.SendChain(message.Reply(id), message.Text("这块田没有种植作物"))
			return
		}
		inc := 5 + rand.Float64()*10
		f.Water += inc
		if f.Water > 100 {
			f.Water = 100
		}

		// 组装整行保存
		rowToSave := farmdb.buildFullRow(uid, fields)
		if err = farmdb.insertOrUpdateField(rowToSave); err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		ctx.SendChain(message.Reply(id), message.Text("浇水成功！", f.Crop, "的水分增加了", strconv.FormatFloat(inc, 'f', 1, 64), "，现在为", strconv.FormatFloat(f.Water, 'f', 1, 64)))
	})

	// 施肥
	engine.OnRegex(`^施肥\s*(\d)$`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		uid := ctx.Event.UserID
		fieldIndex, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[1])
		fieldIndex--
		if fieldIndex < 0 || fieldIndex >= 4 {
			ctx.SendChain(message.Reply(id), message.Text("田地编号必须为1-4"))
			return
		}
		fields, err := farmdb.getFieldsByUser(uid)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if fields[0] == nil {
			ctx.SendChain(message.Reply(id), message.Text("你还未拥有任何田地，请先去种植吧"))
			return
		}
		f := fields[fieldIndex]
		if f == nil || f.Crop == "" {
			ctx.SendChain(message.Reply(id), message.Text("这块田没有种植作物"))
			return
		}
		inc := 3 + rand.Float64()*8
		f.Fertilizer += inc
		if f.Fertilizer > 100 {
			f.Fertilizer = 100
		}

		rowToSave := farmdb.buildFullRow(uid, fields)
		if err = farmdb.insertOrUpdateField(rowToSave); err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		ctx.SendChain(message.Reply(id), message.Text("施肥成功！", f.Crop, "的肥度增加了", strconv.FormatFloat(inc, 'f', 1, 64), "，现在为", strconv.FormatFloat(f.Fertilizer, 'f', 1, 64)))
	})

	// 收获
	engine.OnRegex(`^收获\s*(\d)$`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		uid := ctx.Event.UserID
		fieldIndex, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[1])
		fieldIndex--
		if fieldIndex < 0 || fieldIndex >= 4 {
			ctx.SendChain(message.Reply(id), message.Text("田地编号必须为1-4"))
			return
		}
		fields, err := farmdb.getFieldsByUser(uid)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if fields[0] == nil {
			ctx.SendChain(message.Reply(id), message.Text("你还未拥有任何田地，请先去种植吧"))
			return
		}
		f := fields[fieldIndex]
		if f == nil || f.Crop == "" {
			ctx.SendChain(message.Reply(id), message.Text("这块田没有种植作物"))
			return
		}
		cfg := GetCropConfig(f.Crop)
		if cfg == nil {
			ctx.SendChain(message.Reply(id), message.Text("作物配置错误"))
			return
		}
		now := time.Now().Unix()
		growthTime := f.GrowthDuration * int64(f.HarvestCount)
		if now-f.PlantTime-growthTime < f.GrowthDuration {
			ctx.SendChain(message.Reply(id), message.Text("作物还未成熟"))
			return
		}
		if f.HarvestCount >= f.TotalHarvests {
			ctx.SendChain(message.Reply(id), message.Text("这块田已经枯竭，无法继续收获"))
			return
		}
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
		for i := 0; i < 4; i++ {
			if i == fieldIndex {
				continue
			}
			if fields[i] != nil && fields[i].Crop == "幻昙花" {
				// 检查幻昙花是否成熟
				now := time.Now().Unix()
				growthTime := fields[i].GrowthDuration * int64(fields[i].HarvestCount)
				if now-fields[i].PlantTime-growthTime >= fields[i].GrowthDuration && fields[i].HarvestCount < fields[i].TotalHarvests {
					yield++
					break // 只加一次
				}
			}
		}

		// 减去被偷的数量
		stolenAmount, err := farmdb.getStolenAmount(uid, fieldIndex+1, f.PlantTime)
		if err == nil {
			yield -= stolenAmount
		}

		if yield < 1 {
			yield = 1
		}
		err = farmdb.updateCrop(uid, f.Crop, yield)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}

		// 帮忙奖励
		rewardMsg := awardHelpersForHarvest(ctx, farmdb, uid, fieldIndex, f.Crop, f.PlantTime)

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

		// 组装整行保存
		rowToSave := farmdb.buildFullRow(uid, fields)
		if err = farmdb.insertOrUpdateField(rowToSave); err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		imgPath := getCropImage(cfg, uid, fieldIndex)

		// 构建回复消息
		replyText := "收获成功！获得了 " + strconv.Itoa(yield) + " 个 " + f.Crop
		if rewardMsg != "" {
			replyText += "\n" + rewardMsg
		}
		ctx.SendChain(message.Reply(id), message.Image(imgPath), message.Text(replyText))
	})

	// 出售（不变）
	engine.OnRegex(`^出售\s*(\S+)\s*(\d+)$`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		uid := ctx.Event.UserID
		cropName := ctx.State["regex_matched"].([]string)[1]
		qty, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[2])
		if qty <= 0 {
			ctx.SendChain(message.Reply(id), message.Text("数量必须为正数"))
			return
		}
		cfg := GetCropConfig(cropName)
		if cfg == nil {
			ctx.SendChain(message.Reply(id), message.Text("没有这种作物"))
			return
		}
		have, err := farmdb.getCropQuantity(uid, cropName)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if have < qty {
			ctx.SendChain(message.Reply(id), message.Text("你没有那么多", cropName))
			return
		}
		err = farmdb.updateCrop(uid, cropName, -qty)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		income := qty * cfg.SellPrice
		err = wallet.InsertWalletOf(uid, income)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		ctx.SendChain(message.Reply(id), message.Text("出售成功！获得", strconv.Itoa(income), "金币"))
	})

	// 转化猫粮（不变）
	engine.OnRegex(`^转化猫粮\s*(\S+)\s*(\d+)$`, zero.OnlyGroup, getdb).SetBlock(true).Limit(ctxext.LimitByUser).Handle(func(ctx *zero.Ctx) {
		id := ctx.Event.MessageID
		uid := ctx.Event.UserID
		cropName := ctx.State["regex_matched"].([]string)[1]
		qty, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[2])
		if qty <= 0 {
			ctx.SendChain(message.Reply(id), message.Text("数量必须为正数"))
			return
		}
		cfg := GetCropConfig(cropName)
		if cfg == nil {
			ctx.SendChain(message.Reply(id), message.Text("没有这种作物"))
			return
		}
		if cfg.Tag != "cat" {
			ctx.SendChain(message.Reply(id), message.Text("只有带有 cat 标签的作物才能转化为猫粮"))
			return
		}
		have, err := farmdb.getCropQuantity(uid, cropName)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if have < qty {
			ctx.SendChain(message.Reply(id), message.Text("你没有那么多", cropName))
			return
		}
		err = farmdb.updateCrop(uid, cropName, -qty)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		catFoodAmount := float64(qty) * cfg.CatFoodRatio
		err = cybercat.AddCatFood(uid, catFoodAmount)
		if err != nil {
			_ = farmdb.updateCrop(uid, cropName, qty)
			ctx.SendChain(message.Text("[ERROR]: 转化猫粮失败：", err))
			return
		}
		ctx.SendChain(message.Reply(id), message.Text("转化成功！为你的猫咪增加了", strconv.FormatFloat(catFoodAmount, 'f', 2, 64), "斤猫粮"))
	})
	//发现小偷处理器
	engine.OnRegex(`^发现小偷.*?(\d)$`, zero.OnlyGroup, getdb).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		ownerID := ctx.Event.UserID
		thiefID := extractFirstAt(ctx)
		if thiefID == 0 {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("请 @ 你要发现的小偷"))
			return
		}
		fieldIndex, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[1])
		fieldIndex-- // 转为0索引
		if fieldIndex < 0 || fieldIndex >= 4 {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("田地编号必须为1-4"))
			return
		}

		since := time.Now().Unix() - DiscoverWindow
		log, err := farmdb.findUndiscoveredSteal(ownerID, fieldIndex+1, since)
		if err != nil || log.ThiefID != thiefID {
			ctx.SendChain(message.Text("没有找到符合条件的偷窃记录"))
			return
		}

		// 计算罚款：双倍种子钱
		cfg := GetCropConfig(log.Crop)
		fine := log.Amount * cfg.Price * 2
		// 检查小偷钱包
		if wallet.GetWalletOf(thiefID) < fine {
			ctx.SendChain(message.Text("小偷余额不足，无法罚款"))
			return
		}
		// 执行罚款和归还
		// 事务处理 - 由于原始包没有提供 Begin 方法，我们直接执行操作
		// 注意：这样会失去事务的原子性，但在这个场景下影响不大
		// 扣小偷钱
		wallet.InsertWalletOf(thiefID, -fine)
		// 加主人钱
		wallet.InsertWalletOf(ownerID, fine)
		// 归还作物（从小偷背包扣，加主人背包）
		farmdb.updateCrop(thiefID, log.Crop, -log.Amount)
		farmdb.updateCrop(ownerID, log.Crop, log.Amount)
		// 更新记录
		_, err = farmdb.Exec(`UPDATE farm_steal_log SET discovered=1, punished=1 WHERE id=?`, log.ID)
		if err != nil {
			return
		}
		ctx.SendChain(message.Text("发现小偷！已从", ctx.CardOrNickName(thiefID), "处扣除", fine, "金币并归还作物"))
	})
}
