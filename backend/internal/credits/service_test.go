package credits

import (
	"errors"
	"sync"
	"testing"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

// memoryRepository is a pure in-memory wallet repository. It deliberately avoids
// SQLite so these tests never trip the Windows temp-file handle cleanup issue,
// and it reproduces the ApplyEntry contract: single mutation path, idempotent on
// idempotency key, before/after balances filled by the adapter.
type memoryRepository struct {
	mu       sync.Mutex
	accounts map[string]*model.CreditAccount
	entries  map[string]*model.CreditLedgerEntry
	order    []string
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		accounts: map[string]*model.CreditAccount{},
		entries:  map[string]*model.CreditLedgerEntry{},
	}
}

func (r *memoryRepository) Wallet(userID string) (*model.CreditAccount, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	account, ok := r.accounts[userID]
	if !ok {
		return nil, false, nil
	}
	copyAccount := *account
	return &copyAccount, true, nil
}

func (r *memoryRepository) CreditAccounts(limit int) ([]model.CreditAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]model.CreditAccount, 0, len(r.accounts))
	for _, account := range r.accounts {
		items = append(items, *account)
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *memoryRepository) CreditEntries(userID string, limit int) ([]model.CreditLedgerEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]model.CreditLedgerEntry, 0, len(r.order))
	for i := len(r.order) - 1; i >= 0; i-- {
		entry := r.entries[r.order[i]]
		if entry.UserID != userID {
			continue
		}
		items = append(items, *entry)
		if len(items) == limit {
			break
		}
	}
	return items, nil
}

func (r *memoryRepository) ApplyEntry(entry *model.CreditLedgerEntry, mutate func(*model.CreditAccount) error) (*model.CreditLedgerEntry, *model.CreditAccount, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.entries[entry.IdempotencyKey]; ok {
		stored := *existing
		account := r.accounts[entry.UserID]
		if account == nil {
			return &stored, &model.CreditAccount{UserID: entry.UserID}, false, nil
		}
		copyAccount := *account
		return &stored, &copyAccount, false, nil
	}
	account, created := r.accounts[entry.UserID], false
	if account == nil {
		account = &model.CreditAccount{ID: "acct-" + entry.UserID, UserID: entry.UserID}
		r.accounts[entry.UserID] = account
		created = true
	}
	before := *account
	if err := mutate(account); err != nil {
		if created {
			delete(r.accounts, entry.UserID)
		} else {
			*account = before
		}
		return nil, nil, false, err
	}
	if account.Balance < 0 || account.Frozen < 0 {
		if created {
			delete(r.accounts, entry.UserID)
		} else {
			*account = before
		}
		return nil, nil, false, errors.New("negative balance invariant violated")
	}
	stored := *entry
	stored.BalanceBefore = before.Balance
	stored.BalanceAfter = account.Balance
	stored.FrozenBefore = before.Frozen
	stored.FrozenAfter = account.Frozen
	r.entries[entry.IdempotencyKey] = &stored
	r.order = append(r.order, entry.IdempotencyKey)
	copyAccount := *account
	return &stored, &copyAccount, true, nil
}

func fixedClock(moment time.Time) Config {
	return Config{Now: func() time.Time { return moment }}
}

// grant funds a wallet so the reserve/settle cases start from a known balance.
func grant(t *testing.T, service *Service, userID string, amount int64) {
	t.Helper()
	if _, err := service.Grant(userID, amount, "admin-1", "测试充值", "test:grant:"+userID); err != nil {
		t.Fatalf("预置积分失败: %v", err)
	}
}

func TestReserveFreezesWithoutSpending(t *testing.T) {
	service := New(newMemoryRepository(), fixedClock(time.Now()))
	grant(t, service, "u1", 100)
	result, err := service.Reserve("u1", "task-1", 30)
	if err != nil {
		t.Fatalf("冻结失败: %v", err)
	}
	if !result.Applied {
		t.Fatal("首次冻结必须 applied=true")
	}
	if result.Account.Balance != 100 || result.Account.Frozen != 30 || result.Account.Available() != 70 {
		t.Fatalf("冻结后账户应为 余额100/冻结30/可用70，得到 %#v", result.Account)
	}
	if result.Entry.FrozenBefore != 0 || result.Entry.FrozenAfter != 30 || result.Entry.BalanceAfter != 100 {
		t.Fatalf("流水前后值应记录冻结变化，得到 %#v", result.Entry)
	}
	account, err := service.Account("u1")
	if err != nil {
		t.Fatal(err)
	}
	if account.Balance != 100 || account.Frozen != 30 {
		t.Fatalf("持久化账户与返回值不一致: %#v", account)
	}
}

func TestReserveRefusesInsufficientAvailable(t *testing.T) {
	repo := newMemoryRepository()
	service := New(repo, fixedClock(time.Now()))
	grant(t, service, "u1", 100)
	if _, err := service.Reserve("u1", "task-1", 30); err != nil {
		t.Fatalf("首次冻结失败: %v", err)
	}
	_, err := service.Reserve("u1", "task-2", 80)
	if err == nil {
		t.Fatal("可用额不足时必须拒绝")
	}
	var appErr *kernel.AppError
	if !errors.As(err, &appErr) || appErr.Status != kernel.CodeForbidden {
		t.Fatalf("应返回 403 积分不足，得到 %v", err)
	}
	account, _ := service.Account("u1")
	if account.Frozen != 30 {
		t.Fatalf("失败的冻结不得改动账户，得到 %#v", account)
	}
	if _, found := repo.entries[ReserveKey("task-2")]; found {
		t.Fatal("失败的冻结不得写流水")
	}
}

func TestReserveIsIdempotentPerTask(t *testing.T) {
	repo := newMemoryRepository()
	service := New(repo, fixedClock(time.Now()))
	grant(t, service, "u1", 100)
	first, err := service.Reserve("u1", "task-1", 30)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Reserve("u1", "task-1", 30)
	if err != nil {
		t.Fatalf("重放不得报错: %v", err)
	}
	if !first.Applied || second.Applied {
		t.Fatalf("首次 applied=true、重放 applied=false，得到 %v/%v", first.Applied, second.Applied)
	}
	account, _ := service.Account("u1")
	if account.Frozen != 30 {
		t.Fatalf("重放不得重复冻结，得到 %#v", account)
	}
	if len(repo.entries) != 2 {
		t.Fatalf("发放+冻结应只有两条流水，得到 %d", len(repo.entries))
	}
}

func TestSettleReleasesFreezeAndDeductsActual(t *testing.T) {
	service := New(newMemoryRepository(), fixedClock(time.Now()))
	grant(t, service, "u1", 100)
	if _, err := service.Reserve("u1", "task-1", 30); err != nil {
		t.Fatal(err)
	}
	result, err := service.Settle("u1", "task-1", 30, 12)
	if err != nil {
		t.Fatalf("结算失败: %v", err)
	}
	if result.Account.Balance != 88 || result.Account.Frozen != 0 {
		t.Fatalf("结算后应 余额88/冻结0，得到 %#v", result.Account)
	}
	if result.Entry.Amount != 12 || result.Entry.FrozenAfter != 0 {
		t.Fatalf("结算流水应记录实际用量并释放冻结，得到 %#v", result.Entry)
	}
}

func TestSettleTwiceDoesNotDoubleCharge(t *testing.T) {
	service := New(newMemoryRepository(), fixedClock(time.Now()))
	grant(t, service, "u1", 100)
	if _, err := service.Reserve("u1", "task-1", 30); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Settle("u1", "task-1", 30, 12); err != nil {
		t.Fatal(err)
	}
	second, err := service.Settle("u1", "task-1", 30, 12)
	if err != nil {
		t.Fatalf("重复结算不得报错: %v", err)
	}
	if second.Applied {
		t.Fatal("重复结算必须收敛为 applied=false")
	}
	account, _ := service.Account("u1")
	if account.Balance != 88 || account.Frozen != 0 {
		t.Fatalf("重复结算不得双扣，得到 %#v", account)
	}
}

func TestSettleAllowsOvershootWithinBalance(t *testing.T) {
	service := New(newMemoryRepository(), fixedClock(time.Now()))
	grant(t, service, "u1", 100)
	if _, err := service.Reserve("u1", "task-1", 30); err != nil {
		t.Fatal(err)
	}
	result, err := service.Settle("u1", "task-1", 30, 55)
	if err != nil {
		t.Fatalf("余额足够时超估结算应放行: %v", err)
	}
	if result.Account.Balance != 45 || result.Account.Frozen != 0 {
		t.Fatalf("超估结算后应 余额45/冻结0，得到 %#v", result.Account)
	}
}

func TestSettleRejectsWhenBalanceCannotCoverActual(t *testing.T) {
	service := New(newMemoryRepository(), fixedClock(time.Now()))
	grant(t, service, "u1", 10)
	if _, err := service.Reserve("u1", "task-1", 10); err != nil {
		t.Fatal(err)
	}
	_, err := service.Settle("u1", "task-1", 10, 50)
	if err == nil {
		t.Fatal("余额不足时必须拒绝结算")
	}
	account, _ := service.Account("u1")
	if account.Balance != 10 || account.Frozen != 10 {
		t.Fatalf("失败的结算必须整体回滚，得到 %#v", account)
	}
}

func TestSettleRejectsMissingFreeze(t *testing.T) {
	service := New(newMemoryRepository(), fixedClock(time.Now()))
	grant(t, service, "u1", 100)
	_, err := service.Settle("u1", "task-1", 30, 12)
	if err == nil {
		t.Fatal("没有冻结记录时结算必须失败")
	}
	var appErr *kernel.AppError
	if !errors.As(err, &appErr) || appErr.Status != kernel.CodeConflict {
		t.Fatalf("应返回 409 状态冲突，得到 %v", err)
	}
}

func TestReleaseReturnsFreezeWithoutTouchingBalance(t *testing.T) {
	service := New(newMemoryRepository(), fixedClock(time.Now()))
	grant(t, service, "u1", 100)
	if _, err := service.Reserve("u1", "task-1", 30); err != nil {
		t.Fatal(err)
	}
	result, err := service.Release("u1", "task-1", 30)
	if err != nil {
		t.Fatalf("释放失败: %v", err)
	}
	if result.Account.Balance != 100 || result.Account.Frozen != 0 || result.Account.Available() != 100 {
		t.Fatalf("释放后应全额可用，得到 %#v", result.Account)
	}
	if result.Entry.Kind != KindUnfreeze || result.Entry.Stage != StageRelease {
		t.Fatalf("释放流水类型应为 unfreeze/release，得到 %#v", result.Entry)
	}
}

func TestReleaseRejectsWhenNothingFrozen(t *testing.T) {
	service := New(newMemoryRepository(), fixedClock(time.Now()))
	grant(t, service, "u1", 100)
	if _, err := service.Release("u1", "task-1", 10); err == nil {
		t.Fatal("没有冻结时必须拒绝释放")
	}
}

func TestAdjustCannotPushBalanceNegative(t *testing.T) {
	service := New(newMemoryRepository(), fixedClock(time.Now()))
	grant(t, service, "u1", 50)
	if _, err := service.Adjust("u1", -60, "admin-1", "纠错", "adjust:1"); err == nil {
		t.Fatal("扣减超过余额必须拒绝")
	}
	account, _ := service.Account("u1")
	if account.Balance != 50 {
		t.Fatalf("被拒的调整不得改动余额，得到 %#v", account)
	}
}

func TestAdjustRequiresReason(t *testing.T) {
	service := New(newMemoryRepository(), fixedClock(time.Now()))
	grant(t, service, "u1", 50)
	if _, err := service.Adjust("u1", 10, "admin-1", "   ", "adjust:reason"); err == nil {
		t.Fatal("缺少原因必须拒绝")
	}
}

func TestRechargeIsIdempotentOnOrderKey(t *testing.T) {
	service := New(newMemoryRepository(), fixedClock(time.Now()))
	first, err := service.Recharge("u1", 500, "alipay", "支付宝订单 A", "order:alipay:1001")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Recharge("u1", 500, "alipay", "支付宝订单 A", "order:alipay:1001")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Applied || second.Applied {
		t.Fatalf("充值回调必须幂等，得到 %v/%v", first.Applied, second.Applied)
	}
	account, _ := service.Account("u1")
	if account.Balance != 500 {
		t.Fatalf("重复回调只能入账一次，得到 %#v", account)
	}
}

func TestLedgerRebuildsAccountAndOrdersNewestFirst(t *testing.T) {
	repo := newMemoryRepository()
	service := New(repo, fixedClock(time.Now()))
	grant(t, service, "u1", 100)
	if _, err := service.Reserve("u1", "task-1", 30); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Settle("u1", "task-1", 30, 12); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reserve("u1", "task-2", 20); err != nil {
		t.Fatal(err)
	}
	entries, err := service.Ledger("u1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("应有 4 条流水，得到 %d", len(entries))
	}
	if entries[0].Stage != StageReserve || entries[0].TaskID != "task-2" {
		t.Fatalf("流水应按时间倒序，得到 %#v", entries[0])
	}
	var balance, frozen int64
	for i := len(entries) - 1; i >= 0; i-- {
		balance += entries[i].BalanceAfter - entries[i].BalanceBefore
		frozen += entries[i].FrozenAfter - entries[i].FrozenBefore
	}
	account, _ := service.Account("u1")
	if balance != account.Balance || frozen != account.Frozen {
		t.Fatalf("流水累计应重建账户：期望 %d/%d，得到 %d/%d", account.Balance, account.Frozen, balance, frozen)
	}
	if account.Balance != 88 || account.Frozen != 20 {
		t.Fatalf("最终账户应为 余额88/冻结20，得到 %#v", account)
	}
}

func TestLedgerUnknownUserIsEmpty(t *testing.T) {
	service := New(newMemoryRepository(), fixedClock(time.Now()))
	entries, err := service.Ledger("nobody", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("无流水用户应返回空列表，得到 %#v", entries)
	}
	account, err := service.Account("nobody")
	if err != nil {
		t.Fatal(err)
	}
	if account.Balance != 0 || account.Frozen != 0 {
		t.Fatalf("无账户用户应读作零钱包，得到 %#v", account)
	}
}

// TestConcurrentReserveNeverOversubscribes drives many simultaneous freezes at
// one wallet. Only as many as the available balance allows may succeed, and the
// frozen total must stay within the balance.
func TestConcurrentReserveNeverOversubscribes(t *testing.T) {
	repo := newMemoryRepository()
	service := New(repo, fixedClock(time.Now()))
	grant(t, service, "u1", 100)

	const attempts = 20
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
	)
	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func(index int) {
			defer wg.Done()
			result, err := service.Reserve("u1", "task-"+time.Duration(index).String(), 10)
			if err != nil || result == nil || !result.Applied {
				return
			}
			mu.Lock()
			succeeded++
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if succeeded != 10 {
		t.Fatalf("余额 100、每笔冻结 10，应恰好成功 10 笔，得到 %d", succeeded)
	}
	account, _ := service.Account("u1")
	if account.Frozen != 100 || account.Balance != 100 || account.Available() != 0 {
		t.Fatalf("并发冻结后应为 余额100/冻结100/可用0，得到 %#v", account)
	}
}

func TestServiceFailsClosedWithoutRepository(t *testing.T) {
	service := New(nil, Config{})
	if _, err := service.Reserve("u1", "task-1", 10); err == nil {
		t.Fatal("未初始化仓储时必须失败关闭")
	}
	if _, err := service.Account("u1"); err == nil {
		t.Fatal("未初始化仓储时查询必须失败关闭")
	}
}
