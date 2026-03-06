package cyberfarm

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/FloatTech/floatbox/file"
	sqlite "github.com/FloatTech/sqlite"
	"github.com/FloatTech/zbputils/img/text"
	"github.com/fogleman/gg"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

// checkShopOpen 检查商店是否开放（6:00-24:00）
func checkShopOpen(ctx *zero.Ctx) bool {
	hour := time.Now().Hour()
	if hour >= 6 && hour < 24 {
		return true
	}
	ctx.SendChain(message.Text("农场商店现在关门了，营业时间为6:00-24:00"))
	return false
}

// getUserBackpack 获取用户背包中所有作物的数量
func getUserBackpack(uid int64) (map[string]int, error) {
	backpack := make(map[string]int)
	for name := range GetAllCropConfigs() {
		qty, err := farmdb.getCropQuantity(uid, name)
		if err != nil {
			return nil, err
		}
		if qty > 0 {
			backpack[name] = qty
		}
	}
	return backpack, nil
}

// formatDuration 将秒数格式化为易读的时长字符串
func formatDuration(seconds int64) string {
	if seconds <= 0 {
		return "已成熟"
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	return strconv.FormatInt(hours, 10) + "小时" + strconv.FormatInt(minutes, 10) + "分"
}

// getCropImage 获取作物图片的本地路径或 Base64 占位图
func getCropImage(cfg *cropConfig, uid int64, fieldIndex int) string {
	// 直接从 crops 文件夹加载图片
	if cfg.ImageURL != "" {
		localPath := filepath.Join(engine.DataFolder(), cfg.ImageURL)
		absPath := filepath.Join(file.BOTPATH, localPath)
		if file.IsExist(absPath) {
			return "file:///" + absPath
		} else {
			fmt.Println("读取作物图片失败:", localPath, "错误: 文件不存在")
		}
	}

	// 默认透明占位图
	return "base64://iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
}

// generateFarmImage 生成农场图片（四宫格）
func generateFarmImage(uid int64, userName string, fields [4]*singleField) (string, error) {
	const cellWidth = 240
	const cellHeight = 440
	const gapHeight = 0
	const extraHeight = 50
	width := cellWidth * 4
	height := cellHeight + gapHeight + extraHeight

	dc := gg.NewContext(width, height)
	dc.SetRGB(0.95, 0.95, 0.95)
	dc.Clear()

	// 加载字体（使用包级变量缓存，避免每次加载）
	font18, err := gg.LoadFontFace(text.BoldFontFile, 18)
	if err != nil {
		return "", err
	}
	font12, err := gg.LoadFontFace(text.BoldFontFile, 12)
	if err != nil {
		return "", err
	}
	font24, err := gg.LoadFontFace(text.BoldFontFile, 24)
	if err != nil {
		font24 = font18
	}

	// 加载图标（建议也缓存到包级变量）
	iconDir := engine.DataFolder() + "icons/"
	waterIcon, _ := gg.LoadImage(iconDir + "water.png")
	fertilizerIcon, _ := gg.LoadImage(iconDir + "fertilizer.png")
	sickleIcon, _ := gg.LoadImage(iconDir + "sickle.png")
	hourglassIcon, _ := gg.LoadImage(iconDir + "hourglass.png")

	for i := 0; i < 4; i++ {
		x := float64(i * cellWidth)
		y := 0.0
		f := fields[i]

		// 绘制格子边框
		dc.SetRGB(0, 0, 0)
		dc.DrawRectangle(x, y, cellWidth-1, cellHeight-1)
		dc.Stroke()

		// 第一块：240x240 图片区
		if f == nil || f.Crop == "" {
			dc.SetRGB(0.9, 0.9, 0.9)
		} else {
			now := time.Now().Unix()
			growthTime := f.GrowthDuration * int64(f.HarvestCount)
			elapsed := now - f.PlantTime - growthTime
			if f.HarvestCount >= f.TotalHarvests {
				dc.SetRGB(0.5, 0.5, 0.5) // 枯竭：深灰
			} else if elapsed >= f.GrowthDuration {
				dc.SetRGB(1.0, 0.8, 0.0) // 可收获：金黄
			} else {
				dc.SetRGB(0.7, 1.0, 0.7) // 生长中：浅绿
			}
		}
		dc.DrawRectangle(x+1, y+1, cellWidth-2, 240-1)
		dc.Fill()

		// 内边框
		dc.SetRGB(1, 1, 1)
		dc.DrawRectangle(x+10, y+10, 220, 220)
		dc.Fill()

		// 绘制作物图片
		if f != nil && f.Crop != "" {
			cfg := GetCropConfig(f.Crop)
			if cfg != nil {
				imgPath := getCropImage(cfg, uid, i)
				if !strings.HasPrefix(imgPath, "base64://") {
					filePath := strings.TrimPrefix(imgPath, "file:///")
					im, err := gg.LoadImage(filePath)
					if err == nil {
						dc.Push()
						dc.Translate(x+10, y+10)
						dc.Scale(220/float64(im.Bounds().Dx()), 220/float64(im.Bounds().Dy()))
						dc.DrawImage(im, 0, 0)
						dc.Pop()
					}
				}
			}
		}

		// 进度条
		progressY := y + 240
		dc.SetRGB(0.8, 0.8, 0.8)
		dc.DrawRectangle(x+10, progressY+2, 220, 16)
		dc.Fill()

		if f != nil && f.Crop != "" {
			now := time.Now().Unix()
			growthTime := f.GrowthDuration * int64(f.HarvestCount)
			elapsed := now - f.PlantTime - growthTime
			var progress float64
			if f.HarvestCount >= f.TotalHarvests {
				progress = 1.0
			} else if elapsed >= f.GrowthDuration {
				progress = 1.0
			} else {
				progress = float64(elapsed) / float64(f.GrowthDuration)
				if progress < 0 {
					progress = 0
				}
			}

			if f.HarvestCount >= f.TotalHarvests {
				dc.SetRGB(0.3, 0.3, 0.3)
			} else if elapsed >= f.GrowthDuration {
				dc.SetRGB(1.0, 0.8, 0.0)
			} else {
				dc.SetRGB(0.2, 0.8, 0.2)
			}
			barWidth := 220 * progress
			if barWidth > 0 {
				dc.DrawRectangle(x+10, progressY+2, barWidth, 16)
				dc.Fill()
			}
		}

		// 名称区
		nameY := y + 260
		dc.SetRGBA(1, 1, 1, 0.8)
		dc.DrawRectangle(x, nameY, cellWidth, 60)
		dc.Fill()
		dc.SetRGB(0, 0, 0)
		dc.SetFontFace(font18)
		if f != nil && f.Crop != "" {
			dc.DrawStringAnchored(f.Crop, x+cellWidth/2, nameY+30, 0.5, 0.5)
		} else {
			dc.DrawStringAnchored("空闲", x+cellWidth/2, nameY+30, 0.5, 0.5)
		}

		// 信息区
		infoY := y + 320
		if f != nil && f.Crop != "" {
			// 水分
			if waterIcon != nil {
				dc.DrawImage(waterIcon, int(x), int(infoY))
			} else {
				dc.SetFontFace(font12)
				dc.DrawStringAnchored("💧", x+40, infoY+30, 0.5, 0.5)
			}
			dc.SetFontFace(font12)
			waterStr := strconv.FormatFloat(f.Water, 'f', 0, 64)
			dc.DrawStringAnchored(waterStr, x+90, infoY+30, 0.5, 0.5)

			// 肥料
			if fertilizerIcon != nil {
				dc.DrawImage(fertilizerIcon, int(x), int(infoY+60))
			} else {
				dc.SetFontFace(font12)
				dc.DrawStringAnchored("🌿", x+40, infoY+90, 0.5, 0.5)
			}
			dc.SetFontFace(font12)
			fertStr := strconv.FormatFloat(f.Fertilizer, 'f', 0, 64)
			dc.DrawStringAnchored(fertStr, x+90, infoY+90, 0.5, 0.5)

			// 可收获次数
			now := time.Now().Unix()
			growthTime := f.GrowthDuration * int64(f.HarvestCount)
			elapsed := now - f.PlantTime - growthTime
			remain := f.GrowthDuration - elapsed
			if remain < 0 {
				remain = 0
			}

			if sickleIcon != nil {
				dc.DrawImage(sickleIcon, int(x+120), int(infoY))
			} else {
				dc.SetFontFace(font12)
				dc.DrawStringAnchored("🔪", x+160, infoY+30, 0.5, 0.5)
			}
			dc.SetFontFace(font12)
			harvestStr := strconv.Itoa(f.HarvestCount) + "/" + strconv.Itoa(f.TotalHarvests)
			dc.DrawStringAnchored(harvestStr, x+210, infoY+30, 0.5, 0.5)

			// 剩余时间
			if hourglassIcon != nil {
				dc.DrawImage(hourglassIcon, int(x+120), int(infoY+60))
			} else {
				dc.SetFontFace(font12)
				dc.DrawStringAnchored("⏳", x+160, infoY+90, 0.5, 0.5)
			}
			dc.SetFontFace(font12)
			remainStr := formatDuration(remain)
			if f.HarvestCount >= f.TotalHarvests {
				remainStr = "枯竭"
			} else if elapsed >= f.GrowthDuration {
				remainStr = "可收获"
			}
			dc.DrawStringAnchored(remainStr, x+210, infoY+90, 0.5, 0.5)
		} else {
			dc.SetFontFace(font12)
			dc.DrawStringAnchored("暂无作物", x+cellWidth/2, infoY+60, 0.5, 0.5)
		}
	}

	// 绘制底部用户名
	dc.SetRGB(0.8, 0.8, 0.8)
	dc.DrawRectangle(0, float64(cellHeight+gapHeight), float64(width), float64(extraHeight))
	dc.Fill()
	dc.SetRGB(0, 0, 0)
	dc.SetFontFace(font24)
	dc.DrawStringAnchored(userName, float64(width)/2, float64(cellHeight+gapHeight)+float64(extraHeight)/2, 0.5, 0.5)

	buf := new(bytes.Buffer)
	err = dc.EncodePNG(buf)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// extractFirstAt 从消息中提取第一个被 @ 的 QQ 号
func extractFirstAt(ctx *zero.Ctx) int64 {
	for _, elem := range ctx.Event.Message {
		if elem.Type == "at" {
			if qq, ok := elem.Data["qq"]; ok {
				if id, err := strconv.ParseInt(qq, 10, 64); err == nil {
					return id
				}
			}
		}
	}
	return 0
}

// generateBackpackImage 生成背包图像
func generateBackpackImage(uid int64, userName string, backpack map[string]int) (string, error) {
	const cellWidth = 120
	const cellHeight = 120
	const gap = 10
	const padding = 20
	const maxItemsPerPage = 12 // 每页最多显示12个物品

	// 将背包物品转换为切片，以便分页
	items := make([]struct {
		crop string
		qty  int
	}, 0, len(backpack))
	for crop, qty := range backpack {
		items = append(items, struct {
			crop string
			qty  int
		}{crop, qty})
	}

	itemCount := len(items)
	pageCount := (itemCount + maxItemsPerPage - 1) / maxItemsPerPage

	// 如果背包为空
	if itemCount == 0 {
		const width = 400
		const height = 300
		dc := gg.NewContext(width, height)
		dc.SetRGB(0.95, 0.95, 0.95)
		dc.Clear()

		// 加载字体
		font18, err := gg.LoadFontFace(text.BoldFontFile, 18)
		if err != nil {
			return "", err
		}
		font24, err := gg.LoadFontFace(text.BoldFontFile, 24)
		if err != nil {
			font24 = font18
		}

		// 绘制标题
		dc.SetRGB(0, 0, 0)
		dc.SetFontFace(font24)
		title := userName + "的背包"
		dc.DrawStringAnchored(title, float64(width)/2, float64(padding)+15, 0.5, 0.5)

		// 绘制空背包提示
		dc.SetRGB(0.5, 0.5, 0.5)
		dc.SetFontFace(font18)
		dc.DrawStringAnchored("背包空空如也", float64(width)/2, float64(height)/2, 0.5, 0.5)

		buf := new(bytes.Buffer)
		err = dc.EncodePNG(buf)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
	}

	// 生成第一页图像
	cols := 4
	rows := 3 // 每页3行
	width := padding*2 + cols*cellWidth + (cols-1)*gap
	height := padding*2 + rows*cellHeight + (rows-1)*gap + 50 + 30 // 额外空间用于标题和页码

	dc := gg.NewContext(width, height)
	dc.SetRGB(0.95, 0.95, 0.95)
	dc.Clear()

	// 加载字体
	font18, err := gg.LoadFontFace(text.BoldFontFile, 18)
	if err != nil {
		return "", err
	}
	font12, err := gg.LoadFontFace(text.BoldFontFile, 12)
	if err != nil {
		return "", err
	}
	font24, err := gg.LoadFontFace(text.BoldFontFile, 24)
	if err != nil {
		font24 = font18
	}

	// 绘制标题
	dc.SetRGB(0, 0, 0)
	dc.SetFontFace(font24)
	title := userName + "的背包"
	dc.DrawStringAnchored(title, float64(width)/2, float64(padding)+15, 0.5, 0.5)

	// 绘制第一页物品
	for i := 0; i < maxItemsPerPage && i < itemCount; i++ {
		item := items[i]
		crop := item.crop
		qty := item.qty

		col := i % cols
		row := i / cols
		x := float64(padding + col*(cellWidth+gap))
		y := float64(padding + 50 + row*(cellHeight+gap)) // 50 是标题高度

		// 绘制物品框
		dc.SetRGB(0, 0, 0)
		dc.DrawRectangle(x, y, cellWidth-1, cellHeight-1)
		dc.Stroke()

		// 绘制背景
		dc.SetRGB(1, 1, 1)
		dc.DrawRectangle(x+1, y+1, cellWidth-2, cellHeight-2)
		dc.Fill()

		// 绘制作物图片
		cfg := GetCropConfig(crop)
		if cfg != nil && cfg.ImageURL != "" {
			localPath := filepath.Join(engine.DataFolder(), cfg.ImageURL)
			absPath := filepath.Join(file.BOTPATH, localPath)
			if file.IsExist(absPath) {
				im, err := gg.LoadImage(absPath)
				if err == nil {
					dc.Push()
					dc.Translate(x+10, y+10)
					dc.Scale(100/float64(im.Bounds().Dx()), 80/float64(im.Bounds().Dy()))
					dc.DrawImage(im, 0, 0)
					dc.Pop()
				}
			}
		}

		// 绘制作物名称（自动换行）
		dc.SetRGB(0, 0, 0)
		dc.SetFontFace(font12)
		// 计算文本宽度，超过则换行
		textWidth, _ := dc.MeasureString(crop)
		if textWidth > float64(cellWidth-20) {
			// 简单的换行处理
			if len(crop) > 4 {
				firstLine := crop[:4]
				secondLine := crop[4:]
				dc.DrawStringAnchored(firstLine, x+float64(cellWidth)/2, y+90, 0.5, 0.5)
				dc.DrawStringAnchored(secondLine, x+float64(cellWidth)/2, y+105, 0.5, 0.5)
			} else {
				dc.DrawStringAnchored(crop, x+float64(cellWidth)/2, y+95, 0.5, 0.5)
			}
		} else {
			dc.DrawStringAnchored(crop, x+float64(cellWidth)/2, y+95, 0.5, 0.5)
		}

		// 绘制数量
		dc.SetFontFace(font18)
		dc.DrawStringAnchored("x"+strconv.Itoa(qty), x+float64(cellWidth)/2, y+110, 0.5, 0.5)
	}

	// 绘制页码
	if pageCount > 1 {
		dc.SetRGB(0.5, 0.5, 0.5)
		dc.SetFontFace(font12)
		pageInfo := "第 1/" + strconv.Itoa(pageCount) + " 页"
		dc.DrawStringAnchored(pageInfo, float64(width)/2, float64(height)-15, 0.5, 0.5)
	}

	buf := new(bytes.Buffer)
	err = dc.EncodePNG(buf)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// awardHelpersForHarvest 为收获时的帮助者发放奖励，返回奖励信息字符串
func awardHelpersForHarvest(ctx *zero.Ctx, db *farmDB, ownerID int64, fieldIndex int, crop string, plantTime int64) string {
	var rewardMsg string

	// 查询帮助记录（未奖励且帮助时间 >= 种植时间）
	type helperRecord struct {
		HelperID int64 `db:"helper_id"`
	}
	var helpers []*helperRecord
	var record helperRecord
	var err error
	err = db.Sqlite.QueryFor(`
        SELECT helper_id FROM farm_help_log
        WHERE owner_id = ? AND field_index = ? AND help_time >= ? AND rewarded = 0`,
		&record, func() error {
			helpers = append(helpers, &helperRecord{HelperID: record.HelperID})
			return nil
		}, ownerID, fieldIndex+1, plantTime)
	if err != nil && err != sqlite.ErrNullResult {
		return ""
	}

	// 检查该田地生长周期内是否有未被发现的偷窃
	var stealExists struct {
		Exists int
	}
	err = db.Sqlite.Query(`
        SELECT 1 FROM farm_steal_log
        WHERE victim_id = ? AND field_index = ? AND steal_time >= ? AND discovered = 0
        LIMIT 1`,
		&stealExists, ownerID, fieldIndex+1, plantTime)
	if err == nil {
		return "" // 有偷窃，不发奖励
	}

	// 遍历帮助者发放奖励（每人2个）
	for _, helper := range helpers {
		helperID := helper.HelperID
		// 奖励1个作物
		_ = db.updateCrop(helperID, crop, 1) // 奖励失败不影响主流程
		// 标记已奖励
		_, _ = db.Exec(`
            UPDATE farm_help_log SET rewarded = 1
            WHERE owner_id = ? AND field_index = ? AND helper_id = ?`,
			ownerID, fieldIndex+1, helperID)

		// 拼接奖励信息
		if rewardMsg == "" {
			rewardMsg = "\n帮忙奖励："
		}
		rewardMsg += "\n" + ctx.CardOrNickName(helperID) + " 获得了 1 个 " + crop
	}
	return rewardMsg
}
