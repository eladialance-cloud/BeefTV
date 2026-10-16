package payment

import (
	"errors"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/localcrypto"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

type fakeRepo struct {
	items  []model.PaymentProviderConfig
	audits []model.AdminAuditEvent
}

func (f *fakeRepo) PaymentProviderConfigs() ([]model.PaymentProviderConfig, error) {
	return append([]model.PaymentProviderConfig(nil), f.items...), nil
}

func (f *fakeRepo) PaymentProviderConfig(id string) (*model.PaymentProviderConfig, error) {
	for index := range f.items {
		if f.items[index].ID == id {
			item := f.items[index]
			return &item, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepo) CreatePaymentProviderConfig(item *model.PaymentProviderConfig) error {
	f.items = append(f.items, *item)
	return nil
}

func (f *fakeRepo) UpdatePaymentProviderConfig(item *model.PaymentProviderConfig) error {
	for index := range f.items {
		if f.items[index].ID == item.ID {
			f.items[index] = *item
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *fakeRepo) DeletePaymentProviderConfig(id string) error {
	for index := range f.items {
		if f.items[index].ID == id {
			f.items = append(f.items[:index], f.items[index+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *fakeRepo) AppendAdminAudit(event *model.AdminAuditEvent) error {
	f.audits = append(f.audits, *event)
	return nil
}

// 支付配置用例不涉及充值订单，下面几个方法只用于满足 Repository 接口。
func (f *fakeRepo) TopupProducts() ([]model.TopupProduct, error) { return nil, nil }
func (f *fakeRepo) TopupProduct(id string) (*model.TopupProduct, error) {
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeRepo) CreateTopupProduct(item *model.TopupProduct) error     { return nil }
func (f *fakeRepo) UpdateTopupProduct(item *model.TopupProduct) error     { return gorm.ErrRecordNotFound }
func (f *fakeRepo) DeleteTopupProduct(id string) error                    { return gorm.ErrRecordNotFound }
func (f *fakeRepo) PaymentOrders(limit int) ([]model.PaymentOrder, error) { return nil, nil }
func (f *fakeRepo) PaymentOrdersByUser(userID string, limit int) ([]model.PaymentOrder, error) {
	return nil, nil
}
func (f *fakeRepo) PaymentOrder(id string) (*model.PaymentOrder, error) {
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeRepo) CreatePaymentOrder(item *model.PaymentOrder) error { return nil }
func (f *fakeRepo) UpdatePaymentOrder(item *model.PaymentOrder) error {
	return gorm.ErrRecordNotFound
}
func (f *fakeRepo) CreatePaymentNotification(item *model.PaymentNotification) (bool, error) {
	return true, nil
}

func (f *fakeRepo) UpdatePaymentNotification(item *model.PaymentNotification) error { return nil }

func adminActor() *model.User {
	return &model.User{ID: "admin-1", Role: model.UserRoleAdmin}
}

func newServiceForTest(t *testing.T) (*Service, *fakeRepo) {
	t.Helper()
	repo := &fakeRepo{}
	return New(repo, nil, t.TempDir()), repo
}

func validInput() Input {
	return Input{
		Provider:   model.PaymentProviderAlipay,
		Name:       "支付宝主账号",
		AppID:      "2021000000000000",
		MchID:      "2088000000000000",
		NotifyURL:  "https://example.com/api/payments/alipay/notify",
		PrivateKey: "-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----",
	}
}

func TestCreateRejectsUnsupportedProvider(t *testing.T) {
	svc, _ := newServiceForTest(t)
	input := validInput()
	input.Provider = "paypal"
	if _, err := svc.Create(adminActor(), input); err == nil {
		t.Fatal("不支持的支付渠道必须被拒绝")
	}
}

func TestCreateRejectsMissingSecret(t *testing.T) {
	svc, _ := newServiceForTest(t)
	input := validInput()
	input.PrivateKey = ""
	if _, err := svc.Create(adminActor(), input); err == nil {
		t.Fatal("新增渠道必须至少填一项密钥")
	}
}

func TestCreateRejectsNonAdminActor(t *testing.T) {
	svc, _ := newServiceForTest(t)
	_, err := svc.Create(&model.User{ID: "user-1", Role: model.UserRoleUser}, validInput())
	var appErr *kernel.AppError
	if err == nil || !errors.As(err, &appErr) || appErr.Status != 403 {
		t.Fatalf("普通用户不得管理支付渠道，得到 %v", err)
	}
}

func TestCreateEncryptsSecretAndMasksView(t *testing.T) {
	svc, repo := newServiceForTest(t)
	view, err := svc.Create(adminActor(), validInput())
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if !view.SecretSet || view.SecretMasked != secretMask {
		t.Fatalf("视图必须只暴露掩码，得到 %+v", view)
	}
	if len(repo.items) != 1 {
		t.Fatalf("期望落库 1 条，得到 %d", len(repo.items))
	}
	stored := repo.items[0].SecretsEncrypted
	if stored == "" || strings.Contains(stored, "secret") {
		t.Fatalf("密钥必须加密落库，得到 %q", stored)
	}
	plain, err := localcrypto.Decrypt(svc.dataDir, stored)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if !strings.Contains(plain, "BEGIN PRIVATE KEY") {
		t.Fatalf("解密后应包含原始密钥，得到 %q", plain)
	}
	if len(repo.audits) != 1 || repo.audits[0].Action != "payment_provider.create" {
		t.Fatalf("新增必须写审计，得到 %+v", repo.audits)
	}
}

func TestUpdateKeepsStoredSecretWhenBlank(t *testing.T) {
	svc, repo := newServiceForTest(t)
	created, err := svc.Create(adminActor(), validInput())
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	before := repo.items[0].SecretsEncrypted

	_, err = svc.Update(adminActor(), created.ID, Input{
		Provider: model.PaymentProviderAlipay,
		Name:     "支付宝备用",
		AppID:    created.AppID,
		MchID:    created.MchID,
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if repo.items[0].SecretsEncrypted == "" {
		t.Fatal("更新时留空密钥不得清空原密钥")
	}
	beforePlain, err := localcrypto.Decrypt(svc.dataDir, before)
	if err != nil {
		t.Fatalf("解密原密钥失败: %v", err)
	}
	afterPlain, err := localcrypto.Decrypt(svc.dataDir, repo.items[0].SecretsEncrypted)
	if err != nil {
		t.Fatalf("解密新密钥失败: %v", err)
	}
	if beforePlain != afterPlain {
		t.Fatalf("更新时留空密钥必须保留原密钥，得到 %q", afterPlain)
	}
	if repo.items[0].Name != "支付宝备用" {
		t.Fatalf("名称未更新，得到 %q", repo.items[0].Name)
	}
	if len(repo.audits) != 2 || repo.audits[1].Action != "payment_provider.update" {
		t.Fatalf("更新必须写审计，得到 %+v", repo.audits)
	}
}

func TestDeleteMissingReturnsNotFound(t *testing.T) {
	svc, _ := newServiceForTest(t)
	err := svc.Delete(adminActor(), "missing")
	var appErr *kernel.AppError
	if err == nil || !errors.As(err, &appErr) || appErr.Status != 404 {
		t.Fatalf("删除不存在的渠道应 404，得到 %v", err)
	}
}
