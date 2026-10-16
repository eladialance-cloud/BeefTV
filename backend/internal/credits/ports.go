// Package credits owns the hosted wallet: freezing points for in-flight tasks,
// settling by actual usage, refunding failures, and the immutable ledger that
// proves every movement. It is imported only by the server profile; cmd/desktop
// never links it.
package credits

import "infinite-canvas/backend/internal/model"

// Ledger kinds. A kind records the business meaning of a movement; the
// direction lives in the before/after balances, not in the sign of Amount.
const (
	KindFreeze   = "freeze"
	KindConsume  = "consume"
	KindUnfreeze = "unfreeze"
	KindRecharge = "recharge"
	KindGrant    = "grant"
	KindAdjust   = "adjust"
)

// Stages of the three-phase charging contract.
const (
	StageReserve = "reserve"
	StageSettle  = "settle"
	StageRelease = "release"
	StageCredit  = "credit"
)

// Repository is the durable port. ApplyEntry is the only mutation path: it
// appends the entry and moves the account inside one transaction, and it is
// idempotent on entry.IdempotencyKey.
//
// Contract for ApplyEntry:
//   - mutate runs inside the transaction and owns the business invariants;
//     returning an error rolls the whole thing back, leaving no account change
//     and no ledger row.
//   - The adapter snapshots the account before mutate, fills
//     BalanceBefore/After and FrozenBefore/After from that snapshot, and writes
//     the entry; the caller must not pre-fill those four fields.
//   - A replayed call returns the previously stored entry with applied=false and
//     the account as it currently stands; it must not move the balance again.
//   - The adapter provisions an empty wallet on the first movement for a user so
//     the caller never has to create one out of band.
//   - The returned account is never nil on success.
//
// A missing wallet/entry is reported with a bool rather than
// gorm.ErrRecordNotFound so this package never imports gorm, and
// internal/repository never imports credits.
type Repository interface {
	Wallet(userID string) (*model.CreditAccount, bool, error)
	ApplyEntry(entry *model.CreditLedgerEntry, mutate func(*model.CreditAccount) error) (stored *model.CreditLedgerEntry, account *model.CreditAccount, applied bool, err error)
	CreditEntries(userID string, limit int) ([]model.CreditLedgerEntry, error)
}
