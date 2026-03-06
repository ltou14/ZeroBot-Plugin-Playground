// Package cyberfarm 农场插件 - 数据库操作
package cyberfarm

import (
	"database/sql"
	"sync"
	"time"

	sqlite "github.com/FloatTech/sqlite"
)

// fieldInfo 存储四块田的所有数据，以 UserID 为主键
type fieldInfo struct {
	UserID int64 `db:"user_id"` // 主键

	// 田地1
	Crop1          string  `db:"crop1"`
	Water1         float64 `db:"water1"`
	Fertilizer1    float64 `db:"fertilizer1"`
	PlantTime1     int64   `db:"plant_time1"`
	HarvestCount1  int     `db:"harvest_count1"`
	TotalHarvests1 int     `db:"total_harvests1"`
	GrowthDur1     int64   `db:"growth_dur1"`

	// 田地2
	Crop2          string  `db:"crop2"`
	Water2         float64 `db:"water2"`
	Fertilizer2    float64 `db:"fertilizer2"`
	PlantTime2     int64   `db:"plant_time2"`
	HarvestCount2  int     `db:"harvest_count2"`
	TotalHarvests2 int     `db:"total_harvests2"`
	GrowthDur2     int64   `db:"growth_dur2"`

	// 田地3
	Crop3          string  `db:"crop3"`
	Water3         float64 `db:"water3"`
	Fertilizer3    float64 `db:"fertilizer3"`
	PlantTime3     int64   `db:"plant_time3"`
	HarvestCount3  int     `db:"harvest_count3"`
	TotalHarvests3 int     `db:"total_harvests3"`
	GrowthDur3     int64   `db:"growth_dur3"`

	// 田地4
	Crop4          string  `db:"crop4"`
	Water4         float64 `db:"water4"`
	Fertilizer4    float64 `db:"fertilizer4"`
	PlantTime4     int64   `db:"plant_time4"`
	HarvestCount4  int     `db:"harvest_count4"`
	TotalHarvests4 int     `db:"total_harvests4"`
	GrowthDur4     int64   `db:"growth_dur4"`
}

// 定义一个单块田地的数据结构，用于 handlers 中方便处理
type singleField struct {
	Crop           string
	Water          float64
	Fertilizer     float64
	PlantTime      int64
	HarvestCount   int
	TotalHarvests  int
	GrowthDuration int64
}

type userCrop struct {
	UserID   int64  `db:"user_id"`
	CropName string `db:"crop_name"`
	Quantity int    `db:"quantity"`
}

type farmDB struct {
	sqlite.Sqlite
	sync.RWMutex
}

type farmStealLog struct {
	ID         int64  `db:"id"`
	ThiefID    int64  `db:"thief_id"`
	VictimID   int64  `db:"victim_id"`
	FieldIndex int    `db:"field_index"`
	Crop       string `db:"crop"`
	Amount     int    `db:"amount"`
	StealTime  int64  `db:"steal_time"`
	Discovered bool   `db:"discovered"`
	Punished   bool   `db:"punished"`
}

type farmHelpLog struct {
	ID         int64  `db:"id"`
	HelperID   int64  `db:"helper_id"`
	OwnerID    int64  `db:"owner_id"`
	FieldIndex int    `db:"field_index"`
	HelpType   string `db:"help_type"` // 'water' 或 'fertilizer'
	HelpTime   int64  `db:"help_time"`
	Rewarded   bool   `db:"rewarded"`
}

// insertOrUpdateField 插入或更新用户的所有农田数据
func (db *farmDB) insertOrUpdateField(f *fieldInfo) error {
	db.Lock()
	defer db.Unlock()
	return db.Insert("farm_fields", f)
}

// getFieldsByUser 获取用户的所有农田数据，返回一个包含4个singleField指针的数组
func (db *farmDB) getFieldsByUser(uid int64) (fields [4]*singleField, err error) {
	db.RLock()
	defer db.RUnlock()

	var row fieldInfo
	err = db.Find("farm_fields", &row, "WHERE user_id = ?", uid)
	if err != nil {
		if err == sqlite.ErrNullResult {
			err = nil // 用户没有记录，返回全空数组
		}
		return
	}

	// 将数据库中的一行数据映射到四个 singleField
	fields[0] = &singleField{
		Crop:           row.Crop1,
		Water:          row.Water1,
		Fertilizer:     row.Fertilizer1,
		PlantTime:      row.PlantTime1,
		HarvestCount:   row.HarvestCount1,
		TotalHarvests:  row.TotalHarvests1,
		GrowthDuration: row.GrowthDur1,
	}
	fields[1] = &singleField{
		Crop:           row.Crop2,
		Water:          row.Water2,
		Fertilizer:     row.Fertilizer2,
		PlantTime:      row.PlantTime2,
		HarvestCount:   row.HarvestCount2,
		TotalHarvests:  row.TotalHarvests2,
		GrowthDuration: row.GrowthDur2,
	}
	fields[2] = &singleField{
		Crop:           row.Crop3,
		Water:          row.Water3,
		Fertilizer:     row.Fertilizer3,
		PlantTime:      row.PlantTime3,
		HarvestCount:   row.HarvestCount3,
		TotalHarvests:  row.TotalHarvests3,
		GrowthDuration: row.GrowthDur3,
	}
	fields[3] = &singleField{
		Crop:           row.Crop4,
		Water:          row.Water4,
		Fertilizer:     row.Fertilizer4,
		PlantTime:      row.PlantTime4,
		HarvestCount:   row.HarvestCount4,
		TotalHarvests:  row.TotalHarvests4,
		GrowthDuration: row.GrowthDur4,
	}
	return
}

// buildFullRow 将四个 singleField 重新组装成完整的 fieldInfo 行
func (db *farmDB) buildFullRow(uid int64, fields [4]*singleField) *fieldInfo {
	row := &fieldInfo{UserID: uid}
	if fields[0] != nil {
		row.Crop1, row.Water1, row.Fertilizer1, row.PlantTime1, row.HarvestCount1, row.TotalHarvests1, row.GrowthDur1 =
			fields[0].Crop, fields[0].Water, fields[0].Fertilizer, fields[0].PlantTime, fields[0].HarvestCount, fields[0].TotalHarvests, fields[0].GrowthDuration
	}
	if fields[1] != nil {
		row.Crop2, row.Water2, row.Fertilizer2, row.PlantTime2, row.HarvestCount2, row.TotalHarvests2, row.GrowthDur2 =
			fields[1].Crop, fields[1].Water, fields[1].Fertilizer, fields[1].PlantTime, fields[1].HarvestCount, fields[1].TotalHarvests, fields[1].GrowthDuration
	}
	if fields[2] != nil {
		row.Crop3, row.Water3, row.Fertilizer3, row.PlantTime3, row.HarvestCount3, row.TotalHarvests3, row.GrowthDur3 =
			fields[2].Crop, fields[2].Water, fields[2].Fertilizer, fields[2].PlantTime, fields[2].HarvestCount, fields[2].TotalHarvests, fields[2].GrowthDuration
	}
	if fields[3] != nil {
		row.Crop4, row.Water4, row.Fertilizer4, row.PlantTime4, row.HarvestCount4, row.TotalHarvests4, row.GrowthDur4 =
			fields[3].Crop, fields[3].Water, fields[3].Fertilizer, fields[3].PlantTime, fields[3].HarvestCount, fields[3].TotalHarvests, fields[3].GrowthDuration
	}
	return row
}

// getCropQuantity 保持不变
func (db *farmDB) getCropQuantity(uid int64, crop string) (int, error) {
	db.RLock()
	defer db.RUnlock()

	exists := db.CanFind("user_crops", "WHERE user_id = ? AND crop_name = ?", uid, crop)
	if !exists {
		return 0, nil
	}

	var uc userCrop
	err := db.Find("user_crops", &uc, "WHERE user_id = ? AND crop_name = ?", uid, crop)
	if err != nil {
		return 0, err
	}
	return uc.Quantity, nil
}

// updateCrop 保持不变
func (db *farmDB) updateCrop(uid int64, crop string, delta int) error {
	db.Lock()
	defer db.Unlock()

	var uc userCrop
	exists := db.CanFind("user_crops", "WHERE user_id = ? AND crop_name = ?", uid, crop)
	if exists {
		err := db.Find("user_crops", &uc, "WHERE user_id = ? AND crop_name = ?", uid, crop)
		if err != nil {
			return err
		}
	} else {
		uc = userCrop{UserID: uid, CropName: crop, Quantity: 0}
	}

	newQty := uc.Quantity + delta
	if newQty < 0 {
		newQty = 0
	}
	if newQty == 0 {
		return db.Del("user_crops", "WHERE user_id = ? AND crop_name = ?", uid, crop)
	}
	uc.Quantity = newQty
	return db.Insert("user_crops", &uc)
}

// Exec 执行一条 SQL 语句，用于自定义表创建等操作
func (db *farmDB) Exec(query string, args ...any) (sql.Result, error) {
	db.Lock()
	defer db.Unlock()
	// 直接使用 Sqlite 提供的 Exec 方法
	return db.Sqlite.Exec(query, args...)
}

// 插入偷菜记录
func (db *farmDB) insertStealLog(log *farmStealLog) error {
	_, err := db.Sqlite.Exec(`
        INSERT INTO farm_steal_log (thief_id, victim_id, field_index, crop, amount, steal_time, discovered, punished)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ThiefID, log.VictimID, log.FieldIndex, log.Crop, log.Amount, log.StealTime, log.Discovered, log.Punished)
	return err
}

// 查询未发现的偷菜记录
func (db *farmDB) findUndiscoveredSteal(victimID int64, fieldIndex int, since int64) (*farmStealLog, error) {
	var log struct {
		ID      int64
		ThiefID int64
		Amount  int
		Crop    string
	}
	err := db.Sqlite.Query(`
        SELECT id, thief_id, amount, crop FROM farm_steal_log
        WHERE victim_id = ? AND field_index = ? AND steal_time >= ? AND discovered = 0
        ORDER BY steal_time DESC LIMIT 1`,
		&log, victimID, fieldIndex, since)
	if err != nil {
		return nil, err
	}
	return &farmStealLog{
		ID:      log.ID,
		ThiefID: log.ThiefID,
		Amount:  log.Amount,
		Crop:    log.Crop,
	}, nil
}

// getTodayStealCount 获取今天偷菜次数
func (db *farmDB) getTodayStealCount(thiefID int64) (int, error) {
	startOfDay := time.Now().Truncate(24 * time.Hour).Unix()
	var count struct {
		Count int
	}
	err := db.Sqlite.Query(`SELECT COUNT(*) as count FROM farm_steal_log WHERE thief_id = ? AND steal_time >= ?`, &count, thiefID, startOfDay)
	if err != nil {
		return 0, err
	}
	return count.Count, nil
}

// getStolenAmount 获取田地在当前生长周期内被偷的总量
func (db *farmDB) getStolenAmount(victimID int64, fieldIndex int, plantTime int64) (int, error) {
	var result struct {
		TotalStolen int
	}
	err := db.Sqlite.Query(`
        SELECT COALESCE(SUM(amount), 0) as total_stolen FROM farm_steal_log 
        WHERE victim_id = ? AND field_index = ? AND steal_time >= ?
    `, &result, victimID, fieldIndex, plantTime)
	if err != nil {
		return 0, err
	}
	return result.TotalStolen, nil
}
