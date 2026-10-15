package repository

import (
	"errors"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newStorageLocationRepo(t *testing.T) *Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:storage-locations-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.StorageLocation{}); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

func TestSetDefaultStorageLocationKeepsSingleDefault(t *testing.T) {
	r := newStorageLocationRepo(t)
	now := time.Now()
	first := &model.StorageLocation{ID: "loc-1", Name: "first", Provider: "s3", Enabled: true, IsDefault: true, CreatedAt: now, UpdatedAt: now}
	second := &model.StorageLocation{ID: "loc-2", Name: "second", Provider: "s3", Enabled: true, CreatedAt: now, UpdatedAt: now}
	if err := r.CreateStorageLocation(first); err != nil {
		t.Fatal(err)
	}
	if err := r.CreateStorageLocation(second); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDefaultStorageLocation("loc-2", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	items, err := r.StorageLocations()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 条配置，得到 %d", len(items))
	}
	if !items[0].IsDefault || items[0].ID != "loc-2" {
		t.Fatalf("默认配置应为 loc-2 且排在最前，得到 %#v", items[0])
	}
	if items[1].IsDefault {
		t.Fatalf("旧默认项必须被清掉，得到 %#v", items[1])
	}
}

func TestDeleteStorageLocationReportsMissing(t *testing.T) {
	r := newStorageLocationRepo(t)
	if err := r.DeleteStorageLocation("missing"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("删除不存在的配置应报记录不存在，得到 %v", err)
	}
}

func TestUpdateStorageLocationKeepsCreatedAt(t *testing.T) {
	r := newStorageLocationRepo(t)
	created := time.Now().Add(-time.Hour)
	item := &model.StorageLocation{ID: "loc-1", Name: "before", Provider: "s3", Enabled: true, CreatedAt: created, UpdatedAt: created}
	if err := r.CreateStorageLocation(item); err != nil {
		t.Fatal(err)
	}
	item.Name = "after"
	item.Endpoint = "https://oss.example.com"
	item.UpdatedAt = time.Now()
	if err := r.UpdateStorageLocation(item); err != nil {
		t.Fatal(err)
	}
	stored, err := r.StorageLocation("loc-1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != "after" || stored.Endpoint != "https://oss.example.com" {
		t.Fatalf("可变列未写入，得到 %#v", stored)
	}
	if !stored.CreatedAt.Equal(created.Truncate(time.Second)) && stored.CreatedAt.After(created.Add(time.Second)) {
		t.Fatalf("更新不得改写创建时间，得到 %v", stored.CreatedAt)
	}
}
