package model

import "time"

// Task cost charge statuses. A charge is created with a pricing snapshot and a
// frozen estimate, then either settled by real usage or released back to the
// wallet. Rows are never rewritten into a different attempt; a retry creates a
// new attempt row so the frozen amount of each attempt stays auditable.
const (
	TaskCostStatusReserved = "reserved"
	TaskCostStatusSettled  = "settled"
	TaskCostStatusReleased = "released"
)

// TaskCostCharge is the hosted-only quote snapshot for one task attempt. It
// records which pricing rule the submission used and how many credits were
// frozen, so an operator editing prices later never rewrites an old bill.
type TaskCostCharge struct {
	ID     string `json:"id" gorm:"primaryKey;size:36"`
	TaskID string `json:"taskId" gorm:"size:64;index;uniqueIndex:idx_task_cost_attempt,priority:1"`
	UserID string `json:"userId" gorm:"size:36;index"`
	// Attempt is 1 for the initial submission and increments per user retry.
	Attempt               int        `json:"attempt" gorm:"uniqueIndex:idx_task_cost_attempt,priority:2"`
	TaskType              string     `json:"taskType" gorm:"size:64"`
	Capability            string     `json:"capability" gorm:"size:32"`
	BillingMode           string     `json:"billingMode" gorm:"size:32"`
	RuleID                string     `json:"ruleId" gorm:"size:36;index"`
	RuleKind              string     `json:"ruleKind" gorm:"size:32"`
	RuleName              string     `json:"ruleName" gorm:"size:120"`
	SelectorKey           string     `json:"-" gorm:"size:500"`
	Quantity              int64      `json:"quantity"`
	UnitPriceMicrocredits int64      `json:"unitPriceMicrocredits"`
	MinimumChargeCredits  int64      `json:"minimumChargeCredits"`
	EstimatedCredits      int64      `json:"estimatedCredits"`
	ActualCredits         int64      `json:"actualCredits"`
	Status                string     `json:"status" gorm:"size:24;index"`
	ReleaseReason         string     `json:"releaseReason,omitempty" gorm:"type:text"`
	CreatedAt             time.Time  `json:"createdAt" gorm:"index"`
	SettledAt             *time.Time `json:"settledAt,omitempty"`
	ReleasedAt            *time.Time `json:"releasedAt,omitempty"`
}

func (TaskCostCharge) TableName() string { return "task_cost_charges" }
