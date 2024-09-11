package test

import (
	"github.com/sagan/ptool/cmd/brush/brush_store"
	"github.com/sagan/ptool/config"
	log "github.com/sirupsen/logrus"
	"testing"
)

func TestName(t *testing.T) {
	config.ConfigDir = "D:\\Projects\\Golang\\github.com\\ptool"
	storeDBManager := brush_store.NewBrushStoreDBManager()
	torrentRecordManager := brush_store.NewTorrentRecordManager(storeDBManager.GetDB())
	record := torrentRecordManager.GetByHash("f6d0a32103e23e0784cc0cf9b572fe8280399734")
	log.Info(record)
	torrentRecordManager.MarkSlowTorrentRecord("f6d0a32103e23e0784cc0cf9b572fe8280399734", "")
	torrentRecordManager.MarkDeleteRecord("f6d0a32103e23e0784cc0cf9b572fe8280399734")
	result2 := torrentRecordManager.IsDeletedRecord("mteam.226025")
	log.Infof("是否删除 %v", result2)

}
