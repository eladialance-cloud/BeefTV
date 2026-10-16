package storageconfig

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	localasset "infinite-canvas/backend/internal/asset"
	"infinite-canvas/backend/internal/localcrypto"
	"infinite-canvas/backend/internal/model"
)

// Router resolves resource bytes across platform storage locations. New writes
// go to the configured default location; existing rows keep resolving to the
// location recorded when they were written, so switching the default never
// strands older objects.
type Router struct {
	repo    Repository
	dataDir string
	local   localasset.BlobStore

	mu     sync.Mutex
	stores map[string]cachedBlobStore
}

type cachedBlobStore struct {
	updatedAt time.Time
	store     localasset.BlobStore
}

// NewRouter wires the platform storage configuration to the resource domain.
// local is the byte store used for rows recorded on this machine (legacy rows
// and the no-object-storage fallback).
func NewRouter(repo Repository, dataDir string, local localasset.BlobStore) *Router {
	return &Router{repo: repo, dataDir: strings.TrimSpace(dataDir), local: local, stores: map[string]cachedBlobStore{}}
}

// DefaultTarget returns the location new writes must use. A deployment with no
// object storage configured keeps writing to local disk; a configured default
// that is disabled or untested is reported instead of silently ignored, because
// falling back would quietly fill the server disk.
func (r *Router) DefaultTarget() (localasset.BlobTarget, error) {
	if r == nil || r.repo == nil {
		return localasset.BlobTarget{}, errors.New("存储配置路由未初始化")
	}
	items, err := r.repo.StorageLocations()
	if err != nil {
		return localasset.BlobTarget{}, err
	}
	var candidate *model.StorageLocation
	for index := range items {
		if !items[index].IsDefault {
			continue
		}
		candidate = &items[index]
		break
	}
	if candidate == nil {
		return localasset.BlobTarget{Provider: "local"}, nil
	}
	if !candidate.Enabled {
		return localasset.BlobTarget{}, fmt.Errorf("默认对象存储「%s」已停用，请先在管理后台启用或切换默认存储", candidate.Name)
	}
	if candidate.LastTestedAt == nil {
		return localasset.BlobTarget{}, fmt.Errorf("默认对象存储「%s」尚未通过连通性测试，请先在管理后台测试", candidate.Name)
	}
	if strings.TrimSpace(candidate.LastTestError) != "" {
		return localasset.BlobTarget{}, fmt.Errorf("默认对象存储「%s」最近一次连通性测试未通过，请先在管理后台重新测试", candidate.Name)
	}
	return targetFor(candidate), nil
}

// Resolve returns the byte store for a recorded target. It never silently
// falls back to another location: reading bucket A's key from bucket B would
// hand back the wrong bytes.
func (r *Router) Resolve(target localasset.BlobTarget) (localasset.BlobStore, error) {
	if r == nil {
		return nil, errors.New("存储配置路由未初始化")
	}
	if normalizedProvider(target.Provider) == "local" {
		if r.local == nil {
			return nil, errors.New("本地资源存储未初始化")
		}
		return r.local, nil
	}
	settingID := strings.TrimSpace(target.SettingID)
	if settingID == "" {
		return nil, fmt.Errorf("资源缺少存储配置标识，无法解析 %s 存储", target.Provider)
	}
	if r.repo == nil {
		return nil, errors.New("存储配置路由未初始化")
	}
	item, err := r.repo.StorageLocation(settingID)
	if err != nil {
		return nil, fmt.Errorf("读取存储配置失败：%w", err)
	}
	if !item.Enabled {
		return nil, fmt.Errorf("资源所属的对象存储「%s」已停用", item.Name)
	}
	return r.storeFor(item)
}

// LocalOnly is false: object storage is exactly what this router exists for.
func (r *Router) LocalOnly() bool { return false }

// blobSigner is the optional capability an object-storage backend offers when
// it can hand out a browser-usable direct download address.
type blobSigner interface {
	DirectURL(objectKey string, expires time.Time) (string, bool, error)
}

// DirectURL asks the resource's own storage location for a browser-usable
// address. Local rows answer "" so they keep flowing through this server.
func (r *Router) DirectURL(target localasset.BlobTarget, objectKey string, expires time.Time) (string, bool, error) {
	store, err := r.Resolve(target)
	if err != nil {
		return "", false, err
	}
	signer, ok := store.(blobSigner)
	if !ok {
		return "", false, nil
	}
	return signer.DirectURL(objectKey, expires)
}

// WriteSpace is the process-wide write-lock identity. Every Service sharing
// this router serializes on the same workspace identity, which is what the
// in-process lock table guaranteed for a shared FileStore root.
func (r *Router) WriteSpace() string {
	if r == nil || r.dataDir == "" {
		return ""
	}
	return "storage-router:" + filepath.Clean(r.dataDir)
}

func (r *Router) storeFor(item *model.StorageLocation) (localasset.BlobStore, error) {
	r.mu.Lock()
	if cached, ok := r.stores[item.ID]; ok && cached.updatedAt.Equal(item.UpdatedAt) {
		r.mu.Unlock()
		return cached.store, nil
	}
	r.mu.Unlock()

	secret, err := localcrypto.Decrypt(r.dataDir, item.SecretAccessKey)
	if err != nil {
		return nil, err
	}
	store, err := NewS3BlobStore(Target{
		Provider: item.Provider, Endpoint: item.Endpoint, Region: item.Region, Bucket: item.Bucket,
		CDNBaseURL:  item.CDNBaseURL,
		AccessKeyID: item.AccessKeyID, SecretAccessKey: secret, Prefix: item.Prefix, PathStyle: item.PathStyle,
	}, filepath.Join(r.dataDir, "storage-temp"))
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.stores[item.ID] = cachedBlobStore{updatedAt: item.UpdatedAt, store: store}
	r.mu.Unlock()
	return store, nil
}

func targetFor(item *model.StorageLocation) localasset.BlobTarget {
	return localasset.BlobTarget{
		SettingID: item.ID,
		Provider:  normalizedProvider(item.Provider),
		Endpoint:  strings.TrimSpace(item.Endpoint),
		Bucket:    strings.TrimSpace(item.Bucket),
	}
}

// normalizedProvider folds an empty provider onto "local" so legacy rows and
// half-filled configuration keep a single meaning.
func normalizedProvider(provider string) string {
	if trimmed := strings.ToLower(strings.TrimSpace(provider)); trimmed != "" {
		return trimmed
	}
	return "local"
}

// 编译期断言：Router 与资源域的存储解析端口一致。端口漂移会在这里失败。
var _ localasset.BlobResolver = (*Router)(nil)

// 编译期断言：Router 同时提供浏览器直连地址能力。
var _ localasset.DirectURLResolver = (*Router)(nil)
