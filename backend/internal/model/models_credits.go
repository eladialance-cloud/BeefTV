package model

import "time"

// CreditAccount is the hosted wallet for one user. Balance is spendable points,
// Frozen is reserved by in-flight tasks; Available is Balance - Frozen.
//
// Invariant: Balance + Frozen always equals the cumulative effect of every
// ledger entry for this user, and neither field may go negative.
type CreditAccount struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	UserID    string    `json:"userId" gorm:"size:36;uniqueIndex"`
	Balance   int64     `json:"balance"`
	Frozen    int64     `json:"frozen"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (CreditAccount) TableName() string { return "credit_accounts" }

// Available is the spendable remainder: the balance not already promised to an
// in-flight task. A task may only be frozen against the available amount.
func (a CreditAccount) Available() int64 { return a.Balance - a.Frozen }

// CreditLedgerEntry is an immutable wallet movement. Entries are never updated
// or deleted: corrections are new entries. IdempotencyKey is unique, so a
// repeated submit, retry or provider callback converges on one entry.
type CreditLedgerEntry struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	UserID         string    `json:"userId" gorm:"size:36;index"`
	IdempotencyKey string    `json:"idempotencyKey" gorm:"size:191;uniqueIndex"`
	Kind           string    `json:"kind" gorm:"size:24;index"`
	Stage          string    `json:"stage" gorm:"size:24;index"`
	TaskID         string    `json:"taskId" gorm:"size:64;index"`
	Amount         int64     `json:"amount"`
	BalanceBefore  int64     `json:"balanceBefore"`
	BalanceAfter   int64     `json:"balanceAfter"`
	FrozenBefore   int64     `json:"frozenBefore"`
	FrozenAfter    int64     `json:"frozenAfter"`
	OperatorID     string    `json:"operatorId" gorm:"size:36"`
	Reason         string    `json:"reason" gorm:"type:text"`
	CreatedAt      time.Time `json:"createdAt" gorm:"index"`
}

func (CreditLedgerEntry) TableName() string { return "credit_ledger_entries" }
