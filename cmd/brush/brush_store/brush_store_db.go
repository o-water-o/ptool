package brush_store

import (
	"errors"
	"github.com/glebarez/sqlite"
	"github.com/sagan/ptool/config"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"path/filepath"
	"sync"
	"time"
)

var (
	BrushStoreDBManagerGlobal *BrushStoreDBManager
	mu                        sync.Mutex
)

type TorrentRecordCategory int

const (
	DeleteTorrent TorrentRecordCategory = 1
	SlowTorrent   TorrentRecordCategory = 2
	AddTorrent    TorrentRecordCategory = 3
)

// TorrentRecord 种子记录
type TorrentRecord struct {
	gorm.Model
	Hash          string `comment:"种子hash"`
	Category      TorrentRecordCategory
	Name          string `comment:"种子名称"`
	TrackerDomain string
	Count         int64 `comment:"计数"`
	Remark        string
}

// BrushStoreDBManager 数据库管理类
type BrushStoreDBManager struct {
	db *gorm.DB
}

func (receiver *BrushStoreDBManager) GetDB() *gorm.DB {
	return receiver.db
}

// NewBrushStoreDBManager 初始化数据库管理类
func NewBrushStoreDBManager() *BrushStoreDBManager {
	var dbFilePath string = filepath.Join(config.ConfigDir, "brush_store.db")
	db, err := gorm.Open(sqlite.Open(dbFilePath), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
		panic(err)
	}

	// 自动迁移表结构
	err = db.AutoMigrate(&TorrentRecord{})
	if err != nil {
		panic(err)
	}

	return &BrushStoreDBManager{db: db}
}

// TorrentRecordManager TorrentRecord 操作类
type TorrentRecordManager struct {
	db *gorm.DB
}

// NewTorrentRecordManager 初始化 TorrentRecord 操作类
func NewTorrentRecordManager(db *gorm.DB) *TorrentRecordManager {
	return &TorrentRecordManager{db: db}
}

func (m *TorrentRecordManager) CreateDeleteRecord(hash, name, trackerDomain, remark string) {
	newRecord := TorrentRecord{Hash: hash, Name: name, TrackerDomain: trackerDomain, Remark: remark, Category: DeleteTorrent}
	m.db.Create(&newRecord)
}
func (m *TorrentRecordManager) CreateSlowTorrentRecord(hash, name string) {
	mu.Lock()
	defer mu.Unlock()
	record := m.GetByHash(hash)
	if record == nil {
		newRecord := TorrentRecord{
			Hash: hash, Name: name, Category: SlowTorrent, Count: 1}
		m.db.Create(&newRecord)
	} else {
		m.db.Update(hash, TorrentRecord{Count: record.Count + 1})
	}
}

// GetByHash 根据 Hash 查询记录
func (m *TorrentRecordManager) GetByHash(hash string) *TorrentRecord {
	var foundRecord TorrentRecord
	result := m.db.First(&foundRecord, "Hash = ?", hash)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil
		}
		log.Fatalf("failed to query record: %v", result.Error)
	}
	return &foundRecord
}
func (m *TorrentRecordManager) IsDeletedRecord(name string) bool {
	var foundRecords *[]TorrentRecord
	result := m.db.Where(map[string]interface{}{"Name": name, "Category": DeleteTorrent}).Find(&foundRecords)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return true
		}
		panic(result.Error)
	}
	if result.RowsAffected > 0 {
		return true
	}
	return false
}
func (m *TorrentRecordManager) GetSlowTorrentCountByHash(hash string) int64 {
	byHash := m.GetByHash(hash)
	if byHash == nil {
		return 0
	} else {
		return byHash.Count
	}
}

// UpdateReason 更新记录的 Reason
func (m *TorrentRecordManager) UpdateByHash(hash string, record TorrentRecord) {
	result := m.db.First(&record, "Hash = ?", hash)
	if result.Error != nil {
		log.Fatalf("failed to query record for update: %v", result.Error)
		panic(result.Error)
	}
	m.db.Model(&record).Where("Hash=?", hash).Updates(record)
}

// DeleteByHash 根据 Hash 删除记录
func (m *TorrentRecordManager) DeleteByHash(hash string) {
	var record TorrentRecord
	result := m.db.First(&record, "Hash = ?", hash)
	if result.Error != nil {
		log.Fatalf("failed to query record for delete: %v", result.Error)
	}
	m.db.Delete(&record)
}

func (m *TorrentRecordManager) DeleteMarkRecords(category TorrentRecordCategory, keepDays int64) {
	daysAgo := time.Now().Add(-48 * time.Hour)
	record := TorrentRecord{Category: category}
	result := m.db.Where("created_at < ?", daysAgo).Delete(&record)
	if result.Error != nil {
		log.Errorf("删除失败, %v", result.Error)
	}
}
