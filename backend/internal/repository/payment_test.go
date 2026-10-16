package repository

import (
	"testing"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newPaymentRepo(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:payment-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.PaymentNotification{}); err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return New(db), db
}

func TestCreatePaymentNotificationScopesIdempotencyByProvider(t *testing.T) {
	repo, db := newPaymentRepo(t)
	first := &model.PaymentNotification{
		ID: "n-1", Provider: model.PaymentProviderAlipay, ProviderNotificationID: "notify-1", OrderID: "o-1",
	}
	created, err := repo.CreatePaymentNotification(first)
	if err != nil || !created {
		t.Fatalf("首次通知应写入，created=%v err=%v", created, err)
	}

	replayed := &model.PaymentNotification{
		ID: "n-2", Provider: model.PaymentProviderAlipay, ProviderNotificationID: "notify-1", OrderID: "o-1",
	}
	created, err = repo.CreatePaymentNotification(replayed)
	if err != nil {
		t.Fatalf("重复通知不应报错: %v", err)
	}
	if created {
		t.Fatal("同一渠道的同一通知号必须视为重复")
	}

	otherProvider := &model.PaymentNotification{
		ID: "n-3", Provider: model.PaymentProviderWeChat, ProviderNotificationID: "notify-1", OrderID: "o-2",
	}
	created, err = repo.CreatePaymentNotification(otherProvider)
	if err != nil || !created {
		t.Fatalf("不同渠道可使用相同通知号，created=%v err=%v", created, err)
	}

	var count int64
	if err := db.Model(&model.PaymentNotification{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("应只落两条通知，得到 %d", count)
	}
}
