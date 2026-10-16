package hostedapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/payment"

	"github.com/gin-gonic/gin"
)

type fakePaymentService struct {
	items  []payment.View
	nextID int
}

func (f *fakePaymentService) List() ([]payment.View, error) {
	return append([]payment.View(nil), f.items...), nil
}

func (f *fakePaymentService) Create(_ *model.User, input payment.Input) (*payment.View, error) {
	if input.Name == "" {
		return nil, kernel.BadAuthRequest("请填写渠道名称")
	}
	f.nextID++
	item := payment.View{
		ID:           "pay-" + string(rune('0'+f.nextID)),
		Provider:     input.Provider,
		Name:         input.Name,
		AppID:        input.AppID,
		SecretMasked: "********",
		SecretSet:    input.PrivateKey != "" || input.APIKey != "",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	f.items = append(f.items, item)
	return &item, nil
}

func (f *fakePaymentService) Update(_ *model.User, id string, input payment.Input) (*payment.View, error) {
	for index := range f.items {
		if f.items[index].ID != id {
			continue
		}
		f.items[index].Name = input.Name
		updated := f.items[index]
		return &updated, nil
	}
	return nil, kernel.NotFound("支付渠道不存在")
}

func (f *fakePaymentService) Delete(_ *model.User, id string) error {
	for index := range f.items {
		if f.items[index].ID == id {
			f.items = append(f.items[:index], f.items[index+1:]...)
			return nil
		}
	}
	return kernel.NotFound("支付渠道不存在")
}

func newPaymentTestEnv(t *testing.T) (*gin.Engine, *fakePaymentService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service := &fakePaymentService{}
	resolve := func(c *gin.Context) (*model.User, error) {
		switch c.GetHeader(testCreditHeader) {
		case "admin":
			return &model.User{ID: testAdminID, Role: model.UserRoleAdmin}, nil
		case "alice":
			return &model.User{ID: testAliceID, Role: model.UserRoleUser}, nil
		default:
			return nil, kernel.Unauthorized("请先登录")
		}
	}
	engine := gin.New()
	RegisterPaymentAdminRoutes(engine.Group("/api"), service, resolve, func(user *model.User) error {
		if user.Role != model.UserRoleAdmin {
			return kernel.Forbidden("需要管理员权限")
		}
		return nil
	})
	return engine, service
}

func paymentRequest(t *testing.T, engine *gin.Engine, method string, path string, body any, actor string) *httptest.ResponseRecorder {
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

func TestPaymentProvidersRejectNonAdmin(t *testing.T) {
	engine, _ := newPaymentTestEnv(t)
	for _, path := range []string{"/api/admin/payment-providers"} {
		if recorder := paymentRequest(t, engine, http.MethodGet, path, nil, "alice"); recorder.Code != http.StatusForbidden {
			t.Fatalf("普通用户访问 %s 应 403，得到 %d", path, recorder.Code)
		}
	}
	if recorder := paymentRequest(t, engine, http.MethodPost, "/api/admin/payment-providers",
		map[string]any{"provider": "alipay", "name": "x", "privateKey": "k"}, "alice"); recorder.Code != http.StatusForbidden {
		t.Fatalf("普通用户新增渠道应 403，得到 %d", recorder.Code)
	}
}

func TestPaymentProviderLifecycle(t *testing.T) {
	engine, _ := newPaymentTestEnv(t)

	created := paymentRequest(t, engine, http.MethodPost, "/api/admin/payment-providers", map[string]any{
		"provider":   "alipay",
		"name":       "支付宝主账号",
		"appId":      "2021000000000000",
		"privateKey": "-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----",
	}, "admin")
	if created.Code != http.StatusOK {
		t.Fatalf("新增应 200，得到 %d: %s", created.Code, created.Body.String())
	}
	if bytes.Contains(created.Body.Bytes(), []byte("BEGIN PRIVATE KEY")) {
		t.Fatal("响应不得回显明文密钥")
	}

	var list struct {
		Data struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	listed := paymentRequest(t, engine, http.MethodGet, "/api/admin/payment-providers", nil, "admin")
	if listed.Code != http.StatusOK {
		t.Fatalf("列表应 200，得到 %d", listed.Code)
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Data.Items) != 1 {
		t.Fatalf("期望 1 条渠道，得到 %d", len(list.Data.Items))
	}
	id := list.Data.Items[0].ID

	updated := paymentRequest(t, engine, http.MethodPut, "/api/admin/payment-providers/"+id,
		map[string]any{"name": "支付宝备用"}, "admin")
	if updated.Code != http.StatusOK {
		t.Fatalf("更新应 200，得到 %d: %s", updated.Code, updated.Body.String())
	}

	removed := paymentRequest(t, engine, http.MethodDelete, "/api/admin/payment-providers/"+id, nil, "admin")
	if removed.Code != http.StatusOK {
		t.Fatalf("删除应 200，得到 %d", removed.Code)
	}
	missing := paymentRequest(t, engine, http.MethodDelete, "/api/admin/payment-providers/"+id, nil, "admin")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("删除不存在的渠道应 404，得到 %d", missing.Code)
	}
}
