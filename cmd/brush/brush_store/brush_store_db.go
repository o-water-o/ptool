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
	ID            string `gorm:"primarykey"`
	Hash          string `comment:"种子hash"`
	Category      TorrentRecordCategory
	Name          string `comment:"种子名称"`
	TrackerDomain string
	Count         int64 `comment:"计数"`
	Remark        string
	gorm.Model
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

func (m *TorrentRecordManager) MarkDeleteRecord(hash string) {
	record := TorrentRecord{}
	result := m.db.Model(&record).Where("hash=?", hash).Updates(map[string]interface{}{"category": DeleteTorrent})
	if result.Error != nil {
		log.Error(result.Error)
	}
}
func (m *TorrentRecordManager) CreateTorrentRecord(siteId, hash, name string) {
	newRecord := TorrentRecord{Hash: hash, Name: name, Category: AddTorrent, ID: siteId}
	result := m.db.Create(&newRecord)
	if result.Error != nil {
		log.Error(result.Error)
	}

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
		torrentRecord := TorrentRecord{}
		countNew := record.Count + 1
		log.Infof("慢速种子 %s 增加计数 %d", name, countNew)
		result := m.db.Model(&torrentRecord).Where(map[string]interface{}{"hash": hash}).Updates(map[string]interface{}{"count": countNew})
		if result.Error != nil {
			log.Error(result.Error)
		}
	}
}

// GetByHash 根据 Hash 查询记录
func (m *TorrentRecordManager) GetByHash(hash string) (foundRecord *TorrentRecord) {
	result := m.db.First(&foundRecord, "Hash = ?", hash)
	if result.Error == nil {
		return foundRecord
	}
	if result.Error != nil && errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil
	}
	log.Error(result.Error)
	return
}
func (m *TorrentRecordManager) IsDeletedRecord(Id string) bool {
	var foundRecords *[]TorrentRecord
	result := m.db.Where(map[string]interface{}{"id": Id, "Category": DeleteTorrent}).Find(&foundRecords)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return true
		}
		panic(result.Error)
	}
	if len(*foundRecords) > 0 {
		return true
	} else {
		return false
	}

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
