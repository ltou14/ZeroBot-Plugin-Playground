package cyberfarm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// cropConfig 作物配置
type cropConfig struct {
	Name         string
	Price        int     // 种子价格
	GrowthTime   int64   // 生长时间（秒）
	Yield        int     // 每次收获产量
	Harvests     int     // 可收获次数
	SellPrice    int     // 单个售价
	CatFoodRatio float64 // 转化为猫粮的系数（斤/个）
	Tag          string  // 标签，如"cat"
	ImageURL     string  // 图片URL
	Description  string  // 作物描述
}

var (
	cropConfigs map[string]*cropConfig
	configOnce  sync.Once
)

// loadCropConfigs 从 JSON 文件加载配置
func loadCropConfigs() error {
	configPath := filepath.Join(engine.DataFolder(), "crops.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return saveDefaultCropConfigs()
		}
		return err
	}
	var configs map[string]*cropConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return err
	}
	cropConfigs = configs
	return nil
}

// saveDefaultCropConfigs 生成默认配置并保存
func saveDefaultCropConfigs() error {
	defaultConfigs := map[string]*cropConfig{
		"小麦": {
			Name:         "小麦",
			Price:        10,
			GrowthTime:   3600,
			Yield:        3,
			Harvests:     1,
			SellPrice:    5,
			CatFoodRatio: 0.5,
			Tag:          "cat",
			ImageURL:     "crops/小麦.png",
			Description:  "常见的粮食作物，适合制作各种食物。",
		},
		"胡萝卜": {
			Name:         "胡萝卜",
			Price:        15,
			GrowthTime:   7200,
			Yield:        2,
			Harvests:     2,
			SellPrice:    8,
			CatFoodRatio: 0.8,
			Tag:          "cat",
			ImageURL:     "crops/胡萝卜.png",
			Description:  "营养丰富的根茎类蔬菜，猫咪很喜欢。",
		},
		"西红柿": {
			Name:         "西红柿",
			Price:        8,
			GrowthTime:   5400,
			Yield:        4,
			Harvests:     1,
			SellPrice:    4,
			CatFoodRatio: 0.3,
			Tag:          "",
			ImageURL:     "crops/西红柿.png",
			Description:  "常见的蔬菜，可用于制作各种料理。",
		},
		"幻昙花": {
			Name:         "幻昙花",
			Price:        100,
			GrowthTime:   10800,
			Yield:        1,
			Harvests:     1,
			SellPrice:    60,
			CatFoodRatio: 2.0,
			Tag:          "rare",
			ImageURL:     "crops/幻昙花.png",
			Description:  "生长在沼泽中的奇迹之花，非常珍贵。成熟时会给周围作物带来产量加成。",
		},
	}
	data, err := json.MarshalIndent(defaultConfigs, "", "  ")
	if err != nil {
		return err
	}
	configPath := filepath.Join(engine.DataFolder(), "crops.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// GetCropConfig 获取单个作物配置
func GetCropConfig(name string) *cropConfig {
	configOnce.Do(func() {
		if err := loadCropConfigs(); err != nil {
			panic("加载作物配置失败: " + err.Error())
		}
	})
	if cfg, ok := cropConfigs[name]; ok {
		return cfg
	}
	return nil
}

// GetAllCropConfigs 获取所有作物配置（用于商店展示）
func GetAllCropConfigs() map[string]*cropConfig {
	configOnce.Do(func() {
		if err := loadCropConfigs(); err != nil {
			panic("加载作物配置失败: " + err.Error())
		}
	})
	// 返回副本防止外部修改
	copy := make(map[string]*cropConfig, len(cropConfigs))
	for k, v := range cropConfigs {
		copy[k] = v
	}
	return copy
}
