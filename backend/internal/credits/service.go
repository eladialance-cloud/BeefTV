package credits

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

// Sentinel domain errors. The HTTP layer maps them to stable envelopes; the
// service itself never decides on a transport status beyond the AppError it
// already returns for user-facing failures.
var (
	ErrInsufficientBalance = errors.New("可用积分不足")
	ErrFrozenUnderflow     = errors.New("冻结积分不足以结算或释放，任务账目可能已错乱")
	ErrMissingReason       = errors.New("必须填写调整原因")
	ErrMissingIdempotency  = errors.New("缺少幂等键")
)

// Config tunes the clock. Tests inject a fixed clock; production uses time.Now.
type Config struct {
	Now func() time.Time
}

// Service is the hosted-only owner of the credit wallet and its ledger.
type Service struct {
	repo Repository
	now  func() time.Time
}

// New wires the credit service. A nil repository is tolerated so the
// composition root can build the type before the database is ready; every
// method then fails closed.
func New(repo Repository, config Config) *Service {
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{repo: repo, now: now}
}

// Result is the outcome of one ledger write. Applied is false when the call
// replayed an existing idempotency key, in which case Entry is the stored row
// and Account is the untouched current wallet.
type Result struct {
	Entry   *model.CreditLedgerEntry
	Account model.CreditAccount
	Applied bool
}

// ReserveKey is the stable idempotency key for a task's freeze.
func ReserveKey(taskID string) string { return "task:" + taskID + ":reserve" }

// SettleKey is the stable idempotency key for a task's settlement.
func SettleKey(taskID string) string { return "task:" + taskID + ":settle" }

// ReleaseKey is the stable idempotency key for a task's refund.
func ReleaseKey(taskID string) string { return "task:" + taskID + ":release" }

// AttemptReserveKey scopes the freeze to one billing attempt so a user retry
// never replays the previous attempt's idempotency key. Attempt 0 keeps the
// legacy single-attempt key shape.
func AttemptReserveKey(taskID string, attempt int) string {
	return attemptKey(taskID, attempt, "reserve")
}

// AttemptSettleKey scopes a settlement to one billing attempt.
func AttemptSettleKey(taskID string, attempt int) string {
	return attemptKey(taskID, attempt, "settle")
}

// AttemptReleaseKey scopes a release to one billing attempt.
func AttemptReleaseKey(taskID string, attempt int) string {
	return attemptKey(taskID, attempt, "release")
}

func attemptKey(taskID string, attempt int, stage string) string {
	if attempt <= 0 {
		return "task:" + taskID + ":" + stage
	}
	return "task:" + taskID + ":" + strconv.Itoa(attempt) + ":" + stage
}

// Reserve freezes amount for an in-flight task. The balance is untouched;
// available (balance - frozen) must cover the estimate or the call is refused
// and nothing is written.
func (s *Service) Reserve(userID string, taskID string, amount int64) (*Result, error) {
	return s.ReserveAttempt(userID, taskID, 0, amount)
}

// ReserveAttempt is Reserve scoped to one billing attempt. Retries pass the
// new attempt number so the previous freeze can never be replayed.
func (s *Service) ReserveAttempt(userID string, taskID string, attempt int, amount int64) (*Result, error) {
	if err := requirePositive(amount, "冻结积分必须大于零"); err != nil {
		return nil, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, kernel.NewAppError(kernel.CodeInvalidArgument, "缺少任务标识")
	}
	entry := &model.CreditLedgerEntry{
		ID:             kernel.NewID(),
		UserID:         userID,
		IdempotencyKey: AttemptReserveKey(taskID, attempt),
		Kind:           KindFreeze,
		Stage:          StageReserve,
		TaskID:         taskID,
		Amount:         amount,
	}
	return s.apply(entry, func(account *model.CreditAccount) error {
		if account.Available() < amount {
			return kernel.QuotaExceeded(ErrInsufficientBalance.Error())
		}
		account.Frozen += amount
		account.Version++
		return nil
	})
}

// Settle converts a task's freeze into actual usage: it releases the whole
// reserved amount from frozen and deducts actual from the balance. Actual may
// exceed the reservation when a task overshoots its estimate, in which case the
// balance must cover the difference; otherwise the call is refused.
func (s *Service) Settle(userID string, taskID string, reserved int64, actual int64) (*Result, error) {
	return s.SettleAttempt(userID, taskID, 0, reserved, actual)
}

// SettleAttempt is Settle scoped to one billing attempt.
func (s *Service) SettleAttempt(userID string, taskID string, attempt int, reserved int64, actual int64) (*Result, error) {
	if err := requirePositive(reserved, "冻结积分必须大于零"); err != nil {
		return nil, err
	}
	if err := requireNonNegative(actual, "实际用量不能为负"); err != nil {
		return nil, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, kernel.NewAppError(kernel.CodeInvalidArgument, "缺少任务标识")
	}
	entry := &model.CreditLedgerEntry{
		ID:             kernel.NewID(),
		UserID:         userID,
		IdempotencyKey: AttemptSettleKey(taskID, attempt),
		Kind:           KindConsume,
		Stage:          StageSettle,
		TaskID:         taskID,
		Amount:         actual,
	}
	return s.apply(entry, func(account *model.CreditAccount) error {
		if account.Frozen < reserved {
			return kernel.NewAppError(kernel.CodeConflict, ErrFrozenUnderflow.Error())
		}
		if account.Balance < actual {
			return kernel.QuotaExceeded(ErrInsufficientBalance.Error())
		}
		account.Frozen -= reserved
		account.Balance -= actual
		account.Version++
		return nil
	})
}

// Release refunds a failed or cancelled task by returning the whole reserved
// amount to available. The balance is untouched.
func (s *Service) Release(userID string, taskID string, reserved int64) (*Result, error) {
	return s.ReleaseAttempt(userID, taskID, 0, reserved)
}

// ReleaseAttempt is Release scoped to one billing attempt.
func (s *Service) ReleaseAttempt(userID string, taskID string, attempt int, reserved int64) (*Result, error) {
	if err := requirePositive(reserved, "冻结积分必须大于零"); err != nil {
		return nil, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, kernel.NewAppError(kernel.CodeInvalidArgument, "缺少任务标识")
	}
	entry := &model.CreditLedgerEntry{
		ID:             kernel.NewID(),
		UserID:         userID,
		IdempotencyKey: AttemptReleaseKey(taskID, attempt),
		Kind:           KindUnfreeze,
		Stage:          StageRelease,
		TaskID:         taskID,
		Amount:         reserved,
	}
	return s.apply(entry, func(account *model.CreditAccount) error {
		if account.Frozen < reserved {
			return kernel.NewAppError(kernel.CodeConflict, ErrFrozenUnderflow.Error())
		}
		account.Frozen -= reserved
		account.Version++
		return nil
	})
}

// Grant credits a user by hand (promotion, apology, bootstrap wallet). The
// operator and a reason are required so the ledger stays auditable.
func (s *Service) Grant(userID string, amount int64, operatorID string, reason string, idempotencyKey string) (*Result, error) {
	if err := requirePositive(amount, "发放积分必须大于零"); err != nil {
		return nil, err
	}
	return s.credit(userID, KindGrant, amount, operatorID, reason, idempotencyKey)
}

// Recharge credits a paid top-up. The payment order id should be passed as the
// idempotency key so a duplicated provider callback credits the user once.
func (s *Service) Recharge(userID string, amount int64, operatorID string, reason string, idempotencyKey string) (*Result, error) {
	if err := requirePositive(amount, "充值积分必须大于零"); err != nil {
		return nil, err
	}
	return s.credit(userID, KindRecharge, amount, operatorID, reason, idempotencyKey)
}

// Adjust applies a signed correction. A negative delta must not push the
// balance below zero.
func (s *Service) Adjust(userID string, delta int64, operatorID string, reason string, idempotencyKey string) (*Result, error) {
	if delta == 0 {
		return nil, kernel.NewAppError(kernel.CodeInvalidArgument, "调整积分不能为零")
	}
	return s.credit(userID, KindAdjust, delta, operatorID, reason, idempotencyKey)
}

func (s *Service) credit(userID string, kind string, delta int64, operatorID string, reason string, idempotencyKey string) (*Result, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, kernel.NewAppError(kernel.CodeInvalidArgument, ErrMissingReason.Error())
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = kind + ":" + kernel.NewID()
	}
	entry := &model.CreditLedgerEntry{
		ID:             kernel.NewID(),
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		Kind:           kind,
		Stage:          StageCredit,
		Amount:         delta,
		OperatorID:     operatorID,
		Reason:         reason,
	}
	return s.apply(entry, func(account *model.CreditAccount) error {
		if account.Balance+delta < 0 {
			return kernel.QuotaExceeded(ErrInsufficientBalance.Error())
		}
		account.Balance += delta
		account.Version++
		return nil
	})
}

// Account returns the user's wallet. A user who has never moved points reads as
// a zero wallet rather than an error so the front end can render a fresh state.
func (s *Service) Account(userID string) (*model.CreditAccount, error) {
	if s == nil || s.repo == nil {
		return nil, kernel.NewAppError(kernel.CodeUnavailable, "积分服务尚未初始化")
	}
	account, found, err := s.repo.Wallet(userID)
	if err != nil {
		return nil, err
	}
	if !found {
		return &model.CreditAccount{UserID: userID}, nil
	}
	return account, nil
}

// Ledger returns the user's most recent movements, newest first.
// Accounts lists wallets for the admin console, newest activity first.
func (s *Service) Accounts(limit int) ([]model.CreditAccount, error) {
	if s == nil || s.repo == nil {
		return nil, kernel.NewAppError(kernel.CodeUnavailable, "积分服务尚未初始化")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	return s.repo.CreditAccounts(limit)
}

func (s *Service) Ledger(userID string, limit int) ([]model.CreditLedgerEntry, error) {
	if s == nil || s.repo == nil {
		return nil, kernel.NewAppError(kernel.CodeUnavailable, "积分服务尚未初始化")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return s.repo.CreditEntries(userID, limit)
}

func (s *Service) apply(entry *model.CreditLedgerEntry, mutate func(*model.CreditAccount) error) (*Result, error) {
	if s == nil || s.repo == nil {
		return nil, kernel.NewAppError(kernel.CodeUnavailable, "积分服务尚未初始化")
	}
	entry.CreatedAt = s.now()
	stored, account, applied, err := s.repo.ApplyEntry(entry, mutate)
	if err != nil {
		return nil, err
	}
	result := &Result{Entry: stored, Applied: applied}
	if account != nil {
		result.Account = *account
	}
	return result, nil
}

func requirePositive(value int64, message string) error {
	if value <= 0 {
		return kernel.NewAppError(kernel.CodeInvalidArgument, message)
	}
	return nil
}

func requireNonNegative(value int64, message string) error {
	if value < 0 {
		return kernel.NewAppError(kernel.CodeInvalidArgument, message)
	}
	return nil
}
