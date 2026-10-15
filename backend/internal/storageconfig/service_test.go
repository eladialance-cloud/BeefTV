package storageconfig

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/localcrypto"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

type fakeRepo struct {
	items  []model.StorageLocation
	audits []model.AdminAuditEvent
}

func (f *fakeRepo) StorageLocations() ([]model.StorageLocation, error) {
	return append([]model.StorageLocation(nil), f.items...), nil
}

func (f *fakeRepo) StorageLocation(id string) (*model.StorageLocation, error) {
	for index := range f.items {
		if f.items[index].ID == id {
			item := f.items[index]
			return &item, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepo) CreateStorageLocation(item *model.StorageLocation) error {
	f.items = append(f.items, *item)
	return nil
}

func (f *fakeRepo) UpdateStorageLocation(item *model.StorageLocation) error {
	for index := range f.items {
		if f.items[index].ID == item.ID {
			f.items[index] = *item
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *fakeRepo) DeleteStorageLocation(id string) error {
	for index := range f.items {
		if f.items[index].ID == id {
			f.items = append(f.items[:index], f.items[index+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *fakeRepo) SetDefaultStorageLocation(id string, updatedAt time.Time) error {
	found := false
	for index := range f.items {
		if f.items[index].ID == id {
			f.items[index].IsDefault = true
			f.items[index].UpdatedAt = updatedAt
			found = true
			continue
		}
		f.items[index].IsDefault = false
	}
	if !found {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (f *fakeRepo) AppendAdminAudit(event *model.AdminAuditEvent) error {
	f.audits = append(f.audits, *event)
	return nil
}

type fakeProber struct {
	err   error
	calls int
	last  Target
}

func (p *fakeProber) Probe(_ context.Context, target Target) error {
	p.calls++
	p.last = target
	return p.err
}

func adminActor() *model.User {
	return &model.User{ID: "admin-1", Role: model.UserRoleAdmin}
}

func newServiceForTest(t *testing.T, prober Prober) (*Service, *fakeRepo) {
	t.Helper()
	repo := &fakeRepo{}
	return New(repo, t.TempDir(), prober), repo
}

func validInput() Input {
	return Input{
		Name: "primary", Provider: "s3", Endpoint: "https://oss-example.aliyuncs.com",
		Region: "cn-hangzhou", Bucket: "beeftv", AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: "super-secret-value",
	}
}

func TestCreateEncryptsSecretAndMasksView(t *testing.T) {
	svc, repo := newServiceForTest(t, &fakeProber{})
	view, err := svc.Create(adminActor(), validInput())
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if view.SecretAccessKeyMasked != secretMask {
		t.Fatalf("列表视图必须掩码密钥，得到 %q", view.SecretAccessKeyMasked)
	}
	if len(repo.items) != 1 {
		t.Fatalf("期望落库 1 条，得到 %d", len(repo.items))
	}
	stored := repo.items[0].SecretAccessKey
	if !strings.HasPrefix(stored, localcrypto.Prefix) {
		t.Fatalf("落库密钥必须加密，得到 %q", stored)
	}
	if strings.Contains(stored, "super-secret-value") {
		t.Fatal("落库密钥不得包含明文")
	}
	plain, err := localcrypto.Decrypt(svc.dataDir, stored)
	if err != nil || plain != "super-secret-value" {
		t.Fatalf("解密回读失败: %v %q", err, plain)
	}
	if len(repo.audits) != 1 || repo.audits[0].ActorUserID != "admin-1" {
		t.Fatalf("写操作必须留审计，得到 %#v", repo.audits)
	}
	if strings.Contains(repo.audits[0].MetadataJSON, "super-secret-value") {
		t.Fatal("审计元数据不得包含明文密钥")
	}
}

func TestSetDefaultRequiresPassingProbe(t *testing.T) {
	prober := &fakeProber{}
	svc, _ := newServiceForTest(t, prober)
	view, err := svc.Create(adminActor(), validInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetDefault(adminActor(), view.ID); err == nil {
		t.Fatal("未测试的配置不得设为默认")
	} else {
		var appErr *kernel.AppError
		if !errors.As(err, &appErr) || appErr.Status != 409 {
			t.Fatalf("应以 409 拒绝，得到 %v", err)
		}
	}
	prober.err = nil
	if _, err := svc.TestConnection(context.Background(), adminActor(), view.ID); err != nil {
		t.Fatalf("连通性测试应通过: %v", err)
	}
	if prober.last.SecretAccessKey != "super-secret-value" {
		t.Fatalf("探针应收到解密后的密钥，得到 %q", prober.last.SecretAccessKey)
	}
	result, err := svc.SetDefault(adminActor(), view.ID)
	if err != nil {
		t.Fatalf("测试通过后应可设为默认: %v", err)
	}
	if !result.IsDefault {
		t.Fatal("默认标记未生效")
	}
}

func TestTestConnectionFailureIsPersisted(t *testing.T) {
	prober := &fakeProber{err: errors.New("dial tcp: connection refused")}
	svc, repo := newServiceForTest(t, prober)
	view, err := svc.Create(adminActor(), validInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TestConnection(context.Background(), adminActor(), view.ID); err == nil {
		t.Fatal("探针失败必须返回错误")
	}
	if repo.items[0].LastTestedAt == nil || !strings.Contains(repo.items[0].LastTestError, "connection refused") {
		t.Fatalf("失败结论必须落库，得到 %#v", repo.items[0])
	}
	if _, err := svc.SetDefault(adminActor(), view.ID); err == nil {
		t.Fatal("测试未通过的配置不得设为默认")
	}
}

func TestUpdateWithEmptySecretKeepsStoredSecret(t *testing.T) {
	svc, repo := newServiceForTest(t, &fakeProber{})
	view, err := svc.Create(adminActor(), validInput())
	if err != nil {
		t.Fatal(err)
	}
	before := repo.items[0].SecretAccessKey
	update := validInput()
	update.SecretAccessKey = ""
	update.Name = "renamed"
	if _, err := svc.Update(adminActor(), view.ID, update); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if repo.items[0].SecretAccessKey != before {
		t.Fatal("密钥留空表示保持不变，不得清空或重加密")
	}
	if repo.items[0].Name != "renamed" {
		t.Fatalf("名称未更新，得到 %q", repo.items[0].Name)
	}
}

func TestDeleteRefusesDefaultStorage(t *testing.T) {
	svc, repo := newServiceForTest(t, &fakeProber{})
	view, err := svc.Create(adminActor(), validInput())
	if err != nil {
		t.Fatal(err)
	}
	repo.items[0].IsDefault = true
	if err := svc.Delete(adminActor(), view.ID); err == nil {
		t.Fatal("默认存储不得删除")
	}
}

func TestNonAdminIsRejected(t *testing.T) {
	svc, _ := newServiceForTest(t, &fakeProber{})
	user := &model.User{ID: "user-1", Role: model.UserRoleUser}
	if _, err := svc.Create(user, validInput()); err == nil {
		t.Fatal("非管理员不得新增存储配置")
	}
}
