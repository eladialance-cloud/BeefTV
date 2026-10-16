// Package storageconfig owns platform-level object-storage configuration: who
// may add it, how secrets are protected, which entry is the default writer, and
// whether a configuration passed a connectivity test.
package storageconfig

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/localcrypto"
	"infinite-canvas/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository is the durable port for storage configurations and admin audit.
type Repository interface {
	StorageLocations() ([]model.StorageLocation, error)
	StorageLocation(id string) (*model.StorageLocation, error)
	CreateStorageLocation(*model.StorageLocation) error
	UpdateStorageLocation(*model.StorageLocation) error
	DeleteStorageLocation(id string) error
	SetDefaultStorageLocation(id string, updatedAt time.Time) error
	AppendAdminAudit(*model.AdminAuditEvent) error
}

// Service is the admin-facing owner of storage configuration.
type Service struct {
	repo    Repository
	dataDir string
	prober  Prober
	now     func() time.Time
}

func New(repo Repository, dataDir string, prober Prober) *Service {
	return &Service{repo: repo, dataDir: dataDir, prober: prober, now: time.Now}
}

// Input is the admin write payload. SecretAccessKey is optional on update: an
// empty value means "keep the stored secret".
type Input struct {
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	CDNBaseURL      string `json:"cdnBaseUrl"`
	Prefix          string `json:"prefix"`
	PathStyle       *bool  `json:"pathStyle"`
	Enabled         *bool  `json:"enabled"`
}

// View is the safe projection returned to the admin UI. It never contains the
// plaintext secret.
type View struct {
	ID                    string     `json:"id"`
	Name                  string     `json:"name"`
	Provider              string     `json:"provider"`
	Endpoint              string     `json:"endpoint"`
	Region                string     `json:"region"`
	Bucket                string     `json:"bucket"`
	AccessKeyID           string     `json:"accessKeyId"`
	SecretAccessKeyMasked string     `json:"secretAccessKeyMasked"`
	CDNBaseURL            string     `json:"cdnBaseUrl"`
	Prefix                string     `json:"prefix"`
	PathStyle             bool       `json:"pathStyle"`
	IsDefault             bool       `json:"isDefault"`
	Enabled               bool       `json:"enabled"`
	LastTestedAt          *time.Time `json:"lastTestedAt"`
	LastTestError         string     `json:"lastTestError"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

// s3CompatibleProviders are the storage families reachable through the same
// S3 protocol adapter. They differ only in endpoint conventions and the
// console that issues the keys.
var s3CompatibleProviders = map[string]struct{}{
	"s3":    {},
	"oss":   {},
	"cos":   {},
	"qiniu": {},
	"minio": {},
}

const secretMask = "********"

func (s *Service) List() ([]View, error) {
	items, err := s.repo.StorageLocations()
	if err != nil {
		return nil, err
	}
	views := make([]View, 0, len(items))
	for index := range items {
		views = append(views, s.toView(&items[index]))
	}
	return views, nil
}

func (s *Service) Create(actor *model.User, input Input) (*View, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	item, err := s.build(actor, input, nil)
	if err != nil {
		return nil, err
	}
	item.ID = uuid.NewString()
	item.CreatedAt = s.now()
	item.UpdatedAt = item.CreatedAt
	item.CreatedBy = actor.ID
	item.UpdatedBy = actor.ID
	if err := s.repo.CreateStorageLocation(item); err != nil {
		return nil, err
	}
	if err := s.audit(actor, "storage_location.create", item, "新增对象存储配置"); err != nil {
		return nil, err
	}
	view := s.toView(item)
	return &view, nil
}

func (s *Service) Update(actor *model.User, id string, input Input) (*View, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	existing, err := s.load(id)
	if err != nil {
		return nil, err
	}
	item, err := s.build(actor, input, existing)
	if err != nil {
		return nil, err
	}
	item.ID = existing.ID
	item.CreatedAt = existing.CreatedAt
	item.CreatedBy = existing.CreatedBy
	item.IsDefault = existing.IsDefault
	item.LastTestedAt = existing.LastTestedAt
	item.LastTestError = existing.LastTestError
	item.UpdatedBy = actor.ID
	item.UpdatedAt = s.now()
	// 连接信息变化后，旧的测试结论不再可信。
	if connectionChanged(existing, item) {
		item.LastTestedAt = nil
		item.LastTestError = ""
	}
	if err := s.repo.UpdateStorageLocation(item); err != nil {
		return nil, err
	}
	if err := s.audit(actor, "storage_location.update", item, "修改对象存储配置"); err != nil {
		return nil, err
	}
	view := s.toView(item)
	return &view, nil
}

// Delete 拒绝删除默认存储：默认项决定新资源写到哪里，必须先切换默认。
func (s *Service) Delete(actor *model.User, id string) error {
	if err := requireActor(actor); err != nil {
		return err
	}
	existing, err := s.load(id)
	if err != nil {
		return err
	}
	if existing.IsDefault {
		return kernel.NewAppError(http.StatusConflict, "默认对象存储不能删除，请先切换默认存储")
	}
	if err := s.repo.DeleteStorageLocation(id); err != nil {
		return err
	}
	return s.audit(actor, "storage_location.delete", existing, "删除对象存储配置")
}

// SetDefault 要求配置已启用且最近一次连通性测试通过。
func (s *Service) SetDefault(actor *model.User, id string) (*View, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	item, err := s.load(id)
	if err != nil {
		return nil, err
	}
	if !item.Enabled {
		return nil, kernel.NewAppError(http.StatusConflict, "配置已停用，不能设为默认存储")
	}
	if item.LastTestedAt == nil {
		return nil, kernel.NewAppError(http.StatusConflict, "请先通过连通性测试再设为默认存储")
	}
	if strings.TrimSpace(item.LastTestError) != "" {
		return nil, kernel.NewAppError(http.StatusConflict, "最近一次连通性测试未通过，不能设为默认存储")
	}
	now := s.now()
	if err := s.repo.SetDefaultStorageLocation(item.ID, now); err != nil {
		return nil, err
	}
	item.IsDefault = true
	item.UpdatedAt = now
	if err := s.audit(actor, "storage_location.set_default", item, "切换默认对象存储"); err != nil {
		return nil, err
	}
	view := s.toView(item)
	return &view, nil
}

// TestConnection 连接、写入、读取、删除一条测试对象。失败结论会落库，
// 用于阻止未通过测试的配置被设为默认。
func (s *Service) TestConnection(ctx context.Context, actor *model.User, id string) (*View, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	item, err := s.load(id)
	if err != nil {
		return nil, err
	}
	secret, err := localcrypto.Decrypt(s.dataDir, item.SecretAccessKey)
	if err != nil {
		return nil, err
	}
	now := s.now()
	probeErr := ErrProberUnavailable
	if s.prober != nil {
		probeErr = s.prober.Probe(ctx, Target{
			Provider: item.Provider, Endpoint: item.Endpoint, Region: item.Region, Bucket: item.Bucket,
			CDNBaseURL:  item.CDNBaseURL,
			AccessKeyID: item.AccessKeyID, SecretAccessKey: secret, Prefix: item.Prefix, PathStyle: item.PathStyle,
		})
	}
	if probeErr != nil {
		item.LastTestedAt = &now
		item.LastTestError = truncate(probeErr.Error(), 500)
		item.UpdatedBy = actor.ID
		item.UpdatedAt = now
		if err := s.repo.UpdateStorageLocation(item); err != nil {
			return nil, err
		}
		if err := s.audit(actor, "storage_location.test_failed", item, "对象存储连通性测试未通过"); err != nil {
			return nil, err
		}
		return nil, kernel.WrapAppError(http.StatusBadGateway, "对象存储连通性测试未通过", probeErr)
	}
	item.LastTestedAt = &now
	item.LastTestError = ""
	item.UpdatedBy = actor.ID
	item.UpdatedAt = now
	if err := s.repo.UpdateStorageLocation(item); err != nil {
		return nil, err
	}
	if err := s.audit(actor, "storage_location.test_passed", item, "对象存储连通性测试通过"); err != nil {
		return nil, err
	}
	view := s.toView(item)
	return &view, nil
}

// ErrProberUnavailable 说明运行形态没有装配对象存储探针。
var ErrProberUnavailable = errors.New("对象存储探针未装配")

func (s *Service) load(id string) (*model.StorageLocation, error) {
	item, err := s.repo.StorageLocation(strings.TrimSpace(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, kernel.NotFound("对象存储配置不存在")
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

// build 校验输入并构造持久化记录。existing 为 nil 表示新增。
func (s *Service) build(actor *model.User, input Input, existing *model.StorageLocation) (*model.StorageLocation, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, kernel.BadAuthRequest("配置名称不能为空")
	}
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if _, ok := s3CompatibleProviders[provider]; !ok {
		return nil, kernel.BadAuthRequest("暂不支持该对象存储类型，当前支持 S3 兼容存储")
	}
	endpoint := strings.TrimSpace(input.Endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, kernel.BadAuthRequest("接入点必须是 http 或 https 地址")
	}
	bucket := strings.TrimSpace(input.Bucket)
	if bucket == "" {
		return nil, kernel.BadAuthRequest("Bucket 不能为空")
	}
	accessKeyID := strings.TrimSpace(input.AccessKeyID)
	if accessKeyID == "" {
		return nil, kernel.BadAuthRequest("AccessKey 不能为空")
	}
	prefix := strings.TrimSpace(input.Prefix)
	if strings.HasPrefix(prefix, "/") || strings.Contains(prefix, "..") || strings.Contains(prefix, "\\") {
		return nil, kernel.BadAuthRequest("路径前缀不能以斜杠开头，且不能包含 .. 或反斜杠")
	}
	secret := input.SecretAccessKey
	if secret == "" {
		if existing == nil {
			return nil, kernel.BadAuthRequest("SecretKey 不能为空")
		}
		secret = existing.SecretAccessKey
	} else {
		encrypted, encErr := localcrypto.Encrypt(s.dataDir, secret)
		if encErr != nil {
			return nil, encErr
		}
		secret = encrypted
	}
	enabled := true
	if existing != nil {
		enabled = existing.Enabled
	}
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	pathStyle := false
	if existing != nil {
		pathStyle = existing.PathStyle
	}
	if input.PathStyle != nil {
		pathStyle = *input.PathStyle
	}
	return &model.StorageLocation{
		Name: name, Provider: provider, Endpoint: endpoint, Region: strings.TrimSpace(input.Region),
		Bucket: bucket, AccessKeyID: accessKeyID, SecretAccessKey: secret,
		CDNBaseURL: strings.TrimSpace(input.CDNBaseURL), Prefix: prefix, PathStyle: pathStyle, Enabled: enabled,
	}, nil
}

func connectionChanged(before *model.StorageLocation, after *model.StorageLocation) bool {
	return before.Provider != after.Provider || before.Endpoint != after.Endpoint || before.Region != after.Region ||
		before.Bucket != after.Bucket || before.AccessKeyID != after.AccessKeyID ||
		before.SecretAccessKey != after.SecretAccessKey || before.PathStyle != after.PathStyle
}

func (s *Service) toView(item *model.StorageLocation) View {
	masked := ""
	if strings.TrimSpace(item.SecretAccessKey) != "" {
		masked = secretMask
	}
	return View{
		ID: item.ID, Name: item.Name, Provider: item.Provider, Endpoint: item.Endpoint, Region: item.Region,
		Bucket: item.Bucket, AccessKeyID: item.AccessKeyID, SecretAccessKeyMasked: masked,
		CDNBaseURL: item.CDNBaseURL, Prefix: item.Prefix, PathStyle: item.PathStyle, IsDefault: item.IsDefault,
		Enabled: item.Enabled, LastTestedAt: item.LastTestedAt, LastTestError: item.LastTestError,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func (s *Service) audit(actor *model.User, action string, item *model.StorageLocation, summary string) error {
	return s.repo.AppendAdminAudit(&model.AdminAuditEvent{
		ID: uuid.NewString(), ActorUserID: actor.ID, Action: action, TargetType: "storage_location",
		TargetID: item.ID, Summary: summary,
		MetadataJSON: fmt.Sprintf(`{"name":%q,"provider":%q,"endpoint":%q,"bucket":%q,"isDefault":%t}`,
			item.Name, item.Provider, item.Endpoint, item.Bucket, item.IsDefault),
		CreatedAt: s.now(),
	})
}

func requireActor(actor *model.User) error {
	if actor == nil {
		return kernel.Unauthorized("请先登录")
	}
	if actor.Role != model.UserRoleAdmin {
		return kernel.Forbidden("需要管理员权限")
	}
	return nil
}

func truncate(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}
