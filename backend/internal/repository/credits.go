package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Wallet returns the hosted credit account for a user. found=false means the
// user has never moved points; the caller treats that as a zero wallet.
func (r *Repository) Wallet(userID string) (*model.CreditAccount, bool, error) {
	var item model.CreditAccount
	err := r.db.First(&item, "user_id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &item, true, nil
}

// CreditEntries returns a user's ledger, newest first.
func (r *Repository) CreditEntries(userID string, limit int) ([]model.CreditLedgerEntry, error) {
	var items []model.CreditLedgerEntry
	err := r.db.
		Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ApplyEntry appends one immutable ledger row and moves the account inside a
// single transaction. It is idempotent on entry.IdempotencyKey.
//
// The row lock taken by clause.Locking serializes concurrent writers on the same
// wallet on PostgreSQL; SQLite already serializes writers, so the clause is a
// harmless hint there. If a different transaction inserts the same idempotency
// key between our lookup and our insert, the insert fails and we converge by
// re-reading the stored entry — the caller sees applied=false, exactly like a
// plain replay.
func (r *Repository) ApplyEntry(entry *model.CreditLedgerEntry, mutate func(*model.CreditAccount) error) (*model.CreditLedgerEntry, *model.CreditAccount, bool, error) {
	stored, account, applied, err := r.applyEntryTx(entry, mutate)
	if err == nil {
		return stored, account, applied, nil
	}
	if existing, found, lookupErr := r.creditEntryByKey(entry.IdempotencyKey); lookupErr == nil && found {
		current, _, walletErr := r.Wallet(entry.UserID)
		if walletErr != nil {
			return existing, nil, false, nil
		}
		return existing, current, false, nil
	}
	return nil, nil, false, err
}

func (r *Repository) applyEntryTx(entry *model.CreditLedgerEntry, mutate func(*model.CreditAccount) error) (*model.CreditLedgerEntry, *model.CreditAccount, bool, error) {
	var (
		stored  model.CreditLedgerEntry
		account model.CreditAccount
		applied bool
	)
	err := r.db.Transaction(func(tx *gorm.DB) error {
		existing, found, err := creditEntryByKeyTx(tx, entry.IdempotencyKey)
		if err != nil {
			return err
		}
		if found {
			stored = *existing
			applied = false
			if current, walletErr := walletTx(tx, entry.UserID); walletErr == nil && current != nil {
				account = *current
			}
			return nil
		}
		if err := loadOrCreateWalletTx(tx, entry, &account); err != nil {
			return err
		}
		before := account
		if err := mutate(&account); err != nil {
			return err
		}
		if account.Balance < 0 || account.Frozen < 0 {
			return errors.New("积分账户不变量被破坏：余额或冻结额不能为负")
		}
		entry.BalanceBefore = before.Balance
		entry.BalanceAfter = account.Balance
		entry.FrozenBefore = before.Frozen
		entry.FrozenAfter = account.Frozen
		entry.CreatedAt = entry.CreatedAt.UTC()
		updatedAt := entry.CreatedAt
		if updatedAt.IsZero() {
			updatedAt = time.Now().UTC()
		}
		result := tx.Model(&model.CreditAccount{}).
			Where("id = ? AND version = ?", account.ID, before.Version).
			Updates(map[string]any{
				"balance":    account.Balance,
				"frozen":     account.Frozen,
				"version":    account.Version,
				"updated_at": updatedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("积分账户版本冲突，请重试")
		}
		if err := tx.Create(entry).Error; err != nil {
			return err
		}
		stored = *entry
		applied = true
		return nil
	})
	if err != nil {
		return nil, nil, false, err
	}
	return &stored, &account, applied, nil
}

// creditEntryByKey looks up a stored ledger row by its idempotency key.
func (r *Repository) creditEntryByKey(key string) (*model.CreditLedgerEntry, bool, error) {
	return creditEntryByKeyTx(r.db, key)
}

func creditEntryByKeyTx(tx *gorm.DB, key string) (*model.CreditLedgerEntry, bool, error) {
	var item model.CreditLedgerEntry
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&item, "idempotency_key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &item, true, nil
}

func walletTx(tx *gorm.DB, userID string) (*model.CreditAccount, error) {
	var item model.CreditAccount
	err := tx.First(&item, "user_id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// loadOrCreateWalletTx reads the wallet under a row lock, creating a zero
// wallet on the user's first movement so callers never have to provision one.
func loadOrCreateWalletTx(tx *gorm.DB, entry *model.CreditLedgerEntry, account *model.CreditAccount) error {
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(account, "user_id = ?", entry.UserID).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	now := entry.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	*account = model.CreditAccount{
		ID:        uuid.NewString(),
		UserID:    entry.UserID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return tx.Create(account).Error
}
