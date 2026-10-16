package hostedapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	testCreditHeader = "X-Test-User"
	testAliceID      = "u-alice"
	testAdminID      = "u-admin"
)

// newCreditTestEnv wires the real credit service over a throwaway SQLite
// database plus a header-driven session resolver, so the tests exercise the GORM
// adapter and the HTTP surface at once without the full auth stack.
func newCreditTestEnv(t *testing.T) (*gin.Engine, *credits.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "credits-api.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(&model.User{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, user := range []model.User{
		{ID: testAliceID, Username: "alice", DisplayName: "Alice", Role: model.UserRoleUser, Status: model.UserStatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: testAdminID, Username: "admin", DisplayName: "Admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive, CreatedAt: now, UpdatedAt: now},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := repository.New(db)
	service := credits.New(repo, credits.Config{})

	resolve := func(c *gin.Context) (*model.User, error) {
		switch c.GetHeader(testCreditHeader) {
		case "alice":
			return &model.User{ID: testAliceID, Username: "alice", DisplayName: "Alice", Role: model.UserRoleUser, Status: model.UserStatusActive}, nil
		case "admin":
			return &model.User{ID: testAdminID, Username: "admin", DisplayName: "Admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}, nil
		default:
			return nil, kernel.Unauthorized("请先登录")
		}
	}
	engine := gin.New()
	api := engine.Group("/api")
	RegisterCreditRoutes(api, service, resolve)
	RegisterCreditAdminRoutes(api, service, repo, resolve, func(user *model.User) error {
		if user.Role != model.UserRoleAdmin {
			return kernel.Forbidden("需要管理员权限")
		}
		return nil
	})
	return engine, service
}

func creditRequest(t *testing.T, engine *gin.Engine, method string, path string, body any, actor string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}
	request := httptest.NewRequest(method, path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		request.Header.Set(testCreditHeader, actor)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestCreditWalletRequiresLogin(t *testing.T) {
	engine, _ := newCreditTestEnv(t)
	recorder := creditRequest(t, engine, http.MethodGet, "/api/credits/wallet", nil, "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("未登录读钱包应 401，得到 %d", recorder.Code)
	}
}

func TestCreditWalletReportsBalanceFrozenAndAvailable(t *testing.T) {
	engine, service := newCreditTestEnv(t)
	if _, err := service.Grant(testAliceID, 100, testAdminID, "初始赠送", "grant:alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reserve(testAliceID, "task-1", 30); err != nil {
		t.Fatal(err)
	}
	recorder := creditRequest(t, engine, http.MethodGet, "/api/credits/wallet", nil, "alice")
	if recorder.Code != http.StatusOK {
		t.Fatalf("读钱包应 200，得到 %d：%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data struct {
			Account creditWalletView `json:"account"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	account := envelope.Data.Account
	if account.Balance != 100 || account.Frozen != 30 || account.Available != 70 {
		t.Fatalf("钱包应为 余额100/冻结30/可用70，得到 %#v", account)
	}
}

func TestCreditLedgerIsScopedToSessionUser(t *testing.T) {
	engine, service := newCreditTestEnv(t)
	if _, err := service.Grant(testAliceID, 100, testAdminID, "Alice 赠送", "grant:alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Grant("u-bob", 999, testAdminID, "Bob 赠送", "grant:bob"); err != nil {
		t.Fatal(err)
	}
	recorder := creditRequest(t, engine, http.MethodGet, "/api/credits/ledger", nil, "alice")
	if recorder.Code != http.StatusOK {
		t.Fatalf("读流水应 200，得到 %d：%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data struct {
			Items []model.CreditLedgerEntry `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Items) != 1 {
		t.Fatalf("用户只能看到自己的流水，得到 %d 条", len(envelope.Data.Items))
	}
	if envelope.Data.Items[0].Amount != 100 {
		t.Fatalf("流水金额应属于 Alice，得到 %#v", envelope.Data.Items[0])
	}
}

func TestCreditLedgerRejectsInvalidLimit(t *testing.T) {
	engine, _ := newCreditTestEnv(t)
	recorder := creditRequest(t, engine, http.MethodGet, "/api/credits/ledger?limit=0", nil, "alice")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("非法 limit 应 400，得到 %d", recorder.Code)
	}
}

func TestCreditAdjustRequiresAdmin(t *testing.T) {
	engine, _ := newCreditTestEnv(t)
	recorder := creditRequest(t, engine, http.MethodPost, "/api/admin/credits/adjust", map[string]any{
		"userId": testAliceID, "amount": 100, "reason": "手动发放",
	}, "alice")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("非管理员调整应 403，得到 %d：%s", recorder.Code, recorder.Body.String())
	}
}

func TestCreditAdjustWritesAuditedLedger(t *testing.T) {
	engine, service := newCreditTestEnv(t)
	recorder := creditRequest(t, engine, http.MethodPost, "/api/admin/credits/adjust", map[string]any{
		"userId": testAliceID, "amount": 250, "reason": "活动奖励", "idempotencyKey": "adjust:promo-1",
	}, "admin")
	if recorder.Code != http.StatusOK {
		t.Fatalf("管理员调整应 200，得到 %d：%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data struct {
			Account creditWalletView        `json:"account"`
			Entry   model.CreditLedgerEntry `json:"entry"`
			Applied bool                    `json:"applied"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Data.Applied || envelope.Data.Account.Balance != 250 {
		t.Fatalf("调整后余额应为 250，得到 %#v", envelope.Data)
	}
	if envelope.Data.Entry.OperatorID != testAdminID || envelope.Data.Entry.Reason != "活动奖励" {
		t.Fatalf("流水必须记录操作人与原因，得到 %#v", envelope.Data.Entry)
	}
	// The same idempotency key must not credit twice, even across HTTP retries.
	repeat := creditRequest(t, engine, http.MethodPost, "/api/admin/credits/adjust", map[string]any{
		"userId": testAliceID, "amount": 250, "reason": "活动奖励", "idempotencyKey": "adjust:promo-1",
	}, "admin")
	if repeat.Code != http.StatusOK {
		t.Fatalf("重复调整应 200，得到 %d", repeat.Code)
	}
	account, err := service.Account(testAliceID)
	if err != nil {
		t.Fatal(err)
	}
	if account.Balance != 250 {
		t.Fatalf("重复调整不得二次入账，得到 %#v", account)
	}
}

func TestCreditAdjustRejectsZeroAmountAndMissingReason(t *testing.T) {
	engine, _ := newCreditTestEnv(t)
	for name, body := range map[string]map[string]any{
		"zero":          {"userId": testAliceID, "amount": 0, "reason": "无效"},
		"missingReason": {"userId": testAliceID, "amount": 10, "reason": "   "},
		"missingUser":   {"amount": 10, "reason": "缺少用户"},
	} {
		recorder := creditRequest(t, engine, http.MethodPost, "/api/admin/credits/adjust", body, "admin")
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s 应 400，得到 %d：%s", name, recorder.Code, recorder.Body.String())
		}
	}
}

func TestCreditAdminAccountsJoinsUserDirectory(t *testing.T) {
	engine, service := newCreditTestEnv(t)
	if _, err := service.Grant(testAliceID, 100, testAdminID, "初始赠送", "grant:alice"); err != nil {
		t.Fatal(err)
	}
	recorder := creditRequest(t, engine, http.MethodGet, "/api/admin/credits/accounts", nil, "admin")
	if recorder.Code != http.StatusOK {
		t.Fatalf("管理员查账户列表应 200，得到 %d：%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data struct {
			Items []creditAccountView `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Items) != 1 {
		t.Fatalf("应只有 Alice 一个钱包，得到 %#v", envelope.Data.Items)
	}
	view := envelope.Data.Items[0]
	if view.UserID != testAliceID || view.Username != "alice" || view.Balance != 100 {
		t.Fatalf("账户视图应带上用户名，得到 %#v", view)
	}
}

func TestCreditAdminAccountsRequiresAdmin(t *testing.T) {
	engine, _ := newCreditTestEnv(t)
	recorder := creditRequest(t, engine, http.MethodGet, "/api/admin/credits/accounts", nil, "alice")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("非管理员查账户列表应 403，得到 %d", recorder.Code)
	}
}
