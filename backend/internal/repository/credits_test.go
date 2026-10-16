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

func newCreditRepo(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:credits-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CreditAccount{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return New(db), db
}

func freezeMutate(amount int64) func(*model.CreditAccount) error {
	return func(account *model.CreditAccount) error {
		account.Frozen += amount
		account.Version++
		return nil
	}
}

func TestApplyEntryProvisionsWalletAndFillsBalances(t *testing.T) {
	repo, db := newCreditRepo(t)
	now := time.Now().UTC().Truncate(time.Second)
	entry := &model.CreditLedgerEntry{
		ID:             "entry-1",
		UserID:         "u1",
		IdempotencyKey: "task:task-1:reserve",
		Kind:           "freeze",
		Stage:          "reserve",
		TaskID:         "task-1",
		Amount:         30,
		CreatedAt:      now,
	}
	stored, account, applied, err := repo.ApplyEntry(entry, freezeMutate(30))
	if err != nil {
		t.Fatalf("写入流水失败: %v", err)
	}
	if !applied {
		t.Fatal("首次写入必须 applied=true")
	}
	if stored.FrozenBefore != 0 || stored.FrozenAfter != 30 || stored.BalanceAfter != 0 {
		t.Fatalf("适配器必须回填冻结前后值，得到 %#v", stored)
	}
	if account == nil || account.Frozen != 30 || account.Balance != 0 {
		t.Fatalf("返回值应是已迁移的账户，得到 %#v", account)
	}
	wallet, found, err := repo.Wallet("u1")
	if err != nil || !found {
		t.Fatalf("首次移动必须自动建钱包，得到 found=%v err=%v", found, err)
	}
	if wallet.Frozen != 30 || wallet.Version != 1 {
		t.Fatalf("钱包应冻结30且版本自增，得到 %#v", wallet)
	}
	var count int64
	if err := db.Model(&model.CreditLedgerEntry{}).Where("user_id = ?", "u1").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("应恰好一条流水，得到 %d", count)
	}
}

func TestApplyEntryReplayConvergesOnStoredEntry(t *testing.T) {
	repo, db := newCreditRepo(t)
	now := time.Now().UTC()
	first := &model.CreditLedgerEntry{
		ID:             "entry-1",
		UserID:         "u1",
		IdempotencyKey: "task:task-1:reserve",
		Kind:           "freeze",
		Stage:          "reserve",
		TaskID:         "task-1",
		Amount:         30,
		CreatedAt:      now,
	}
	if _, _, applied, err := repo.ApplyEntry(first, freezeMutate(30)); err != nil || !applied {
		t.Fatalf("首次写入应成功，applied=%v err=%v", applied, err)
	}
	// A retry reuses the key with a different row id and even a different amount;
	// it must be ignored in favour of the stored row.
	replay := &model.CreditLedgerEntry{
		ID:             "entry-2",
		UserID:         "u1",
		IdempotencyKey: "task:task-1:reserve",
		Kind:           "freeze",
		Stage:          "reserve",
		TaskID:         "task-1",
		Amount:         999,
		CreatedAt:      now.Add(time.Minute),
	}
	stored, account, applied, err := repo.ApplyEntry(replay, freezeMutate(999))
	if err != nil {
		t.Fatalf("重放不得报错: %v", err)
	}
	if applied {
		t.Fatal("重放必须 applied=false")
	}
	if stored.ID != "entry-1" || stored.Amount != 30 {
		t.Fatalf("重放必须返回既有流水，得到 %#v", stored)
	}
	if account == nil || account.Frozen != 30 {
		t.Fatalf("重放不得再次冻结，得到 %#v", account)
	}
	var count int64
	if err := db.Model(&model.CreditLedgerEntry{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("重放不得新增流水，得到 %d 条", count)
	}
}

func TestApplyEntryRollsBackWhenMutateFails(t *testing.T) {
	repo, db := newCreditRepo(t)
	entry := &model.CreditLedgerEntry{
		ID:             "entry-1",
		UserID:         "u1",
		IdempotencyKey: "task:task-1:reserve",
		Kind:           "freeze",
		Stage:          "reserve",
		TaskID:         "task-1",
		Amount:         30,
		CreatedAt:      time.Now().UTC(),
	}
	sentinel := errors.New("insufficient balance")
	_, _, _, err := repo.ApplyEntry(entry, func(account *model.CreditAccount) error {
		account.Frozen += 30
		account.Version++
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("必须透传 mutate 的错误，得到 %v", err)
	}
	if _, found, walletErr := repo.Wallet("u1"); walletErr != nil || found {
		t.Fatalf("回滚后不得留下钱包，得到 found=%v err=%v", found, walletErr)
	}
	var count int64
	if err := db.Model(&model.CreditLedgerEntry{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("回滚后不得留下流水，得到 %d 条", count)
	}
}

func TestApplyEntryRejectsNegativeInvariant(t *testing.T) {
	repo, _ := newCreditRepo(t)
	entry := &model.CreditLedgerEntry{
		ID:             "entry-1",
		UserID:         "u1",
		IdempotencyKey: "task:task-1:reserve",
		Kind:           "freeze",
		Stage:          "reserve",
		TaskID:         "task-1",
		Amount:         30,
		CreatedAt:      time.Now().UTC(),
	}
	_, _, _, err := repo.ApplyEntry(entry, func(account *model.CreditAccount) error {
		account.Frozen -= 5
		account.Version++
		return nil
	})
	if err == nil {
		t.Fatal("冻结额为负必须被适配器拒绝")
	}
}

func TestCreditEntriesReturnsNewestFirst(t *testing.T) {
	repo, _ := newCreditRepo(t)
	base := time.Now().UTC().Truncate(time.Second)
	for index, key := range []string{"a", "b", "c"} {
		entry := &model.CreditLedgerEntry{
			ID:             "entry-" + key,
			UserID:         "u1",
			IdempotencyKey: key,
			Kind:           "grant",
			Stage:          "credit",
			Amount:         10,
			CreatedAt:      base.Add(time.Duration(index) * time.Minute),
		}
		if _, _, _, err := repo.ApplyEntry(entry, func(account *model.CreditAccount) error {
			account.Balance += 10
			account.Version++
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	other := &model.CreditLedgerEntry{
		ID:             "entry-other",
		UserID:         "u2",
		IdempotencyKey: "other",
		Kind:           "grant",
		Stage:          "credit",
		Amount:         99,
		CreatedAt:      base.Add(time.Hour),
	}
	if _, _, _, err := repo.ApplyEntry(other, func(account *model.CreditAccount) error {
		account.Balance += 99
		account.Version++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	entries, err := repo.CreditEntries("u1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("必须只返回该用户的流水，得到 %d 条", len(entries))
	}
	if entries[0].IdempotencyKey != "c" || entries[2].IdempotencyKey != "a" {
		t.Fatalf("流水必须按时间倒序，得到 %#v", entries)
	}
}

func TestWalletMissingReturnsFoundFalse(t *testing.T) {
	repo, _ := newCreditRepo(t)
	if _, found, err := repo.Wallet("nobody"); err != nil || found {
		t.Fatalf("未开钱包应返回 found=false，得到 found=%v err=%v", found, err)
	}
}
