package storageconfig

import (
	"strings"
	"testing"
	"time"

	localasset "infinite-canvas/backend/internal/asset"
	"infinite-canvas/backend/internal/localcrypto"
	"infinite-canvas/backend/internal/model"
)

func encryptedSecret(t *testing.T, dataDir string, secret string) string {
	t.Helper()
	value, err := localcrypto.Encrypt(dataDir, secret)
	if err != nil {
		t.Fatalf("加密密钥失败: %v", err)
	}
	return value
}

func testedLocation(t *testing.T, dataDir string, id string) model.StorageLocation {
	t.Helper()
	now := time.Now()
	return model.StorageLocation{
		ID: id, Name: id, Provider: "s3", Endpoint: "https://oss.example.com", Region: "cn-hangzhou",
		Bucket: "beeftv", AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: encryptedSecret(t, dataDir, "super-secret-value"),
		IsDefault: true, Enabled: true, LastTestedAt: &now,
	}
}

func newRouterFixture(t *testing.T) (string, *fakeRepo, localasset.BlobStore) {
	t.Helper()
	dataDir := t.TempDir()
	return dataDir, &fakeRepo{}, localasset.NewFileStore(dataDir)
}

func TestRouterFallsBackToLocalWhenNothingConfigured(t *testing.T) {
	dataDir, repo, local := newRouterFixture(t)
	router := NewRouter(repo, dataDir, local)

	target, err := router.DefaultTarget()
	if err != nil {
		t.Fatalf("DefaultTarget 失败: %v", err)
	}
	if target.Provider != "local" || target.SettingID != "" {
		t.Fatalf("未配置对象存储应当回落本地，得到 %#v", target)
	}
	store, err := router.Resolve(localasset.BlobTarget{Provider: "local"})
	if err != nil || store != local {
		t.Fatalf("Resolve(local) = %v, %v; 期望本地存储", store, err)
	}
	if router.LocalOnly() {
		t.Fatal("对象存储路由不应声明为 LocalOnly")
	}
	if strings.TrimSpace(router.WriteSpace()) == "" {
		t.Fatal("路由必须给出写锁身份")
	}
}

func TestRouterDefaultTargetUsesConfiguredLocation(t *testing.T) {
	dataDir, repo, local := newRouterFixture(t)
	repo.items = []model.StorageLocation{testedLocation(t, dataDir, "storage-1")}
	router := NewRouter(repo, dataDir, local)

	target, err := router.DefaultTarget()
	if err != nil {
		t.Fatalf("DefaultTarget 失败: %v", err)
	}
	if target.SettingID != "storage-1" || target.Provider != "s3" {
		t.Fatalf("DefaultTarget = %#v", target)
	}
	if target.Endpoint != "https://oss.example.com" || target.Bucket != "beeftv" {
		t.Fatalf("DefaultTarget endpoint/bucket = %q/%q", target.Endpoint, target.Bucket)
	}
}

func TestRouterDefaultTargetReportsUnusableDefault(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*model.StorageLocation)
	}{
		{"disabled", func(item *model.StorageLocation) { item.Enabled = false }},
		{"untested", func(item *model.StorageLocation) { item.LastTestedAt = nil }},
		{"failed", func(item *model.StorageLocation) { item.LastTestError = "连接超时" }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			dataDir, repo, local := newRouterFixture(t)
			item := testedLocation(t, dataDir, "storage-1")
			testCase.mutate(&item)
			repo.items = []model.StorageLocation{item}
			router := NewRouter(repo, dataDir, local)

			_, err := router.DefaultTarget()
			if err == nil || !strings.Contains(err.Error(), "storage-1") {
				t.Fatalf("DefaultTarget error = %v, 期望带配置名的明确错误", err)
			}
		})
	}
}

func TestRouterResolveCachesStoreAndRefreshesOnUpdate(t *testing.T) {
	dataDir, repo, local := newRouterFixture(t)
	repo.items = []model.StorageLocation{testedLocation(t, dataDir, "storage-1")}
	router := NewRouter(repo, dataDir, local)
	target := localasset.BlobTarget{SettingID: "storage-1", Provider: "s3", Endpoint: "https://oss.example.com", Bucket: "beeftv"}

	first, err := router.Resolve(target)
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	second, err := router.Resolve(target)
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if first != second {
		t.Fatal("同一配置的两次 Resolve 必须复用同一个存储句柄")
	}

	repo.items[0].UpdatedAt = repo.items[0].UpdatedAt.Add(time.Minute)
	third, err := router.Resolve(target)
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if third == first {
		t.Fatal("配置更新后必须重建存储句柄")
	}
}

func TestRouterResolveRejectsUnknownAndDisabledLocations(t *testing.T) {
	dataDir, repo, local := newRouterFixture(t)
	item := testedLocation(t, dataDir, "storage-1")
	item.Enabled = false
	repo.items = []model.StorageLocation{item}
	router := NewRouter(repo, dataDir, local)

	if _, err := router.Resolve(localasset.BlobTarget{SettingID: "missing", Provider: "s3"}); err == nil {
		t.Fatal("未知存储配置必须报错，不能回落其它存储")
	}
	if _, err := router.Resolve(localasset.BlobTarget{SettingID: "storage-1", Provider: "s3"}); err == nil {
		t.Fatal("已停用的存储配置必须报错")
	}
	if _, err := router.Resolve(localasset.BlobTarget{Provider: "s3"}); err == nil {
		t.Fatal("缺少存储标识的云资源必须报错")
	}
}
