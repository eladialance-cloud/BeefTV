package asset

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// memoryBlobStore is an in-memory byte store that stands in for a cloud backend.
type memoryBlobStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMemoryBlobStore() *memoryBlobStore {
	return &memoryBlobStore{objects: map[string][]byte{}}
}

func (s *memoryBlobStore) Write(objectKey string, body io.Reader) error {
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[objectKey] = payload
	return nil
}

func (s *memoryBlobStore) Open(objectKey string) (*os.File, error) {
	s.mu.Lock()
	payload, ok := s.objects[objectKey]
	s.mu.Unlock()
	if !ok {
		return nil, os.ErrNotExist
	}
	file, err := os.CreateTemp("", "memory-blob-*")
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func (s *memoryBlobStore) Delete(objectKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, objectKey)
	return nil
}

func (s *memoryBlobStore) Exists(objectKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.objects[objectKey]; !ok {
		return os.ErrNotExist
	}
	return nil
}

func (s *memoryBlobStore) has(objectKey string) bool {
	return s.Exists(objectKey) == nil
}

// routedResolver stands in for the hosted object-storage router: new writes go
// to the remote store, local rows read from the local store.
type routedResolver struct {
	local  BlobStore
	remote BlobStore
	target BlobTarget
	direct string
	stable bool
}

func (r *routedResolver) DefaultTarget() (BlobTarget, error) { return r.target, nil }

func (r *routedResolver) Resolve(target BlobTarget) (BlobStore, error) {
	switch normalizedProvider(target.Provider) {
	case "local":
		return r.local, nil
	default:
		return r.remote, nil
	}
}

func (r *routedResolver) LocalOnly() bool    { return false }
func (r *routedResolver) WriteSpace() string { return "routed-test" }

func (r *routedResolver) DirectURL(BlobTarget, string, time.Time) (string, bool, error) {
	return r.direct, r.stable, nil
}

func newRoutedDomain(t *testing.T, resolver BlobResolver) (*Service, *repository.Repository, string) {
	t.Helper()
	dataDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, "meta.db")+"?_busy_timeout=5000&_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(&model.Resource{}, &model.UserDailyUploadUsage{}, &model.UserUploadReservation{}, &model.CanvasSnapshotResource{}); err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	return NewService(Dependencies{
		Repository: NewRepository(repo),
		Resolver:   resolver,
		Quota:      nopQuota{},
		Lifecycle:  nopLifecycle{},
		DataDir:    dataDir,
	}), repo, dataDir
}

func TestSingleLocationResolverKeepsLocalRowsOnly(t *testing.T) {
	store := newMemoryBlobStore()
	resolver := SingleLocation(store)

	target, err := resolver.DefaultTarget()
	if err != nil {
		t.Fatalf("DefaultTarget error = %v", err)
	}
	if target.Provider != "local" || target.SettingID != "" {
		t.Fatalf("DefaultTarget = %#v, want local with no setting id", target)
	}
	if !resolver.LocalOnly() {
		t.Fatal("SingleLocation must report LocalOnly")
	}
	for _, provider := range []string{"", "local", "LOCAL"} {
		if resolved, resolveErr := resolver.Resolve(BlobTarget{Provider: provider}); resolveErr != nil || resolved != store {
			t.Fatalf("Resolve(%q) = %v, %v; want the local store", provider, resolved, resolveErr)
		}
	}
	_, err = resolver.Resolve(BlobTarget{Provider: "s3", SettingID: "storage-1"})
	if err == nil || !strings.Contains(err.Error(), "资源不在本地存储中") {
		t.Fatalf("Resolve(s3) error = %v, want ResourceNotLocal", err)
	}
}

func TestFileStoreWriteSpaceIsTheResolverWriteSpace(t *testing.T) {
	dataDir := t.TempDir()
	store := NewFileStore(dataDir)
	domain := NewService(Dependencies{Blobs: store})
	if domain.writeSpace() != store.WriteSpace() || store.WriteSpace() == "" {
		t.Fatalf("writeSpace = %q, store = %q", domain.writeSpace(), store.WriteSpace())
	}
	trailing := NewService(Dependencies{Blobs: NewFileStore(dataDir + string(filepath.Separator))})
	if trailing.writeSpace() != domain.writeSpace() {
		t.Fatalf("aliased store writeSpace diverged: %q vs %q", trailing.writeSpace(), domain.writeSpace())
	}
}

func TestRoutedWriteRecordsStorageLocationAndReadsBack(t *testing.T) {
	remote := newMemoryBlobStore()
	resolver := &routedResolver{
		local: NewFileStore(t.TempDir()), remote: remote,
		target: BlobTarget{SettingID: "storage-1", Provider: "s3", Endpoint: "https://oss.example.com", Bucket: "bee"},
	}
	svc, repo, _ := newRoutedDomain(t, resolver)
	payload := []byte("routed-bytes")

	resource, created, err := svc.Store("user-1", "image", "routed.png", "image/png", int64(len(payload)), 0, 0, 0, bytes.NewReader(payload), nil)
	if err != nil || !created {
		t.Fatalf("Store created=%v err=%v", created, err)
	}
	if resource.Provider != "s3" || resource.StorageSettingID != "storage-1" {
		t.Fatalf("resource location = %q/%q", resource.Provider, resource.StorageSettingID)
	}
	if resource.Endpoint != "https://oss.example.com" || resource.Bucket != "bee" {
		t.Fatalf("resource endpoint/bucket = %q/%q", resource.Endpoint, resource.Bucket)
	}
	if !remote.has(resource.ObjectKey) {
		t.Fatalf("remote store missing %q", resource.ObjectKey)
	}

	persisted, err := repo.Resource(resource.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Provider != "s3" || persisted.StorageSettingID != "storage-1" {
		t.Fatalf("persisted location = %q/%q", persisted.Provider, persisted.StorageSettingID)
	}

	stream, err := svc.OpenRange("user-1", resource.ID, "")
	if err != nil {
		t.Fatalf("OpenRange error = %v", err)
	}
	defer stream.Body.Close()
	got, err := io.ReadAll(stream.Body)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("read back = %q err=%v", got, err)
	}

	ranged, err := svc.OpenRange("user-1", resource.ID, "bytes=0-2")
	if err != nil {
		t.Fatalf("OpenRange(range) error = %v", err)
	}
	defer ranged.Body.Close()
	got, err = io.ReadAll(ranged.Body)
	if err != nil || string(got) != "rou" {
		t.Fatalf("range read = %q err=%v", got, err)
	}

	if err := svc.DeleteStoredObject("user-1", resource); err != nil {
		t.Fatalf("Delete error = %v", err)
	}
	if remote.has(resource.ObjectKey) {
		t.Fatalf("remote store still holds %q after delete", resource.ObjectKey)
	}
}

func TestRoutedReadRejectsUnknownCloudRow(t *testing.T) {
	resolver := &routedResolver{local: NewFileStore(t.TempDir()), remote: newMemoryBlobStore()}
	svc, _, _ := newRoutedDomain(t, resolver)
	resource := &model.Resource{
		ID: "resource-cloud", UserID: "user-1", Kind: "image", Status: model.ResourceStatusReady,
		Provider: "s3", ObjectKey: "users/user-1/image/cloud.png",
	}
	// A cloud row with no recorded storage setting has no bucket to read from.
	if _, err := svc.OpenOwnedRange("user-1", resource, ""); err == nil {
		t.Fatal("OpenOwnedRange accepted a cloud row without a setting id")
	}
}

func TestRoutedDomainAllowsCloudRows(t *testing.T) {
	resolver := &routedResolver{local: NewFileStore(t.TempDir()), remote: newMemoryBlobStore()}
	svc, _, _ := newRoutedDomain(t, resolver)
	if svc.LocalOnly() {
		t.Fatal("routed domain must not be local-only")
	}
	if !strings.Contains(resolver.WriteSpace(), "routed") {
		t.Fatalf("resolver write space = %q", resolver.WriteSpace())
	}
}

func cloudResource() *model.Resource {
	return &model.Resource{
		ID: "resource-cloud", UserID: "user-1", Kind: "image", Status: model.ResourceStatusReady,
		Provider: "s3", StorageSettingID: "storage-1",
		ObjectKey: "users/user-1/image/cloud.png",
	}
}

func TestPrepareDeliveryPrefersDirectURLWhenAvailable(t *testing.T) {
	resolver := &routedResolver{
		local: NewFileStore(t.TempDir()), remote: newMemoryBlobStore(),
		direct: "https://cdn.example.com/users/user-1/image/cloud.png", stable: true,
	}
	svc, _, _ := newRoutedDomain(t, resolver)

	delivery, err := svc.PrepareOwnedDelivery("user-1", cloudResource(), ResourceDeliveryOptions{})
	if err != nil {
		t.Fatalf("PrepareOwnedDelivery error = %v", err)
	}
	if delivery.RedirectURL != resolver.direct || !delivery.RedirectStable {
		t.Fatalf("delivery = %#v, want the stable direct URL", delivery)
	}
	if delivery.Stream != nil {
		t.Fatal("直接跳转时不得同时打开代理流")
	}
}

func TestPrepareDeliveryForceProxySkipsDirectURL(t *testing.T) {
	resolver := &routedResolver{
		local: NewFileStore(t.TempDir()), remote: newMemoryBlobStore(),
		direct: "https://cdn.example.com/x.png", stable: true,
	}
	svc, _, _ := newRoutedDomain(t, resolver)

	delivery, err := svc.PrepareOwnedDelivery("user-1", cloudResource(), ResourceDeliveryOptions{ForceProxy: true})
	if err != nil {
		t.Fatalf("PrepareOwnedDelivery error = %v", err)
	}
	if delivery.RedirectURL != "" || delivery.Stream != nil {
		t.Fatalf("force proxy must stay on this server: %#v", delivery)
	}
}

func TestPrepareDeliveryWithoutDirectCapabilityProxies(t *testing.T) {
	store := newMemoryBlobStore()
	svc := NewService(Dependencies{Blobs: store})
	resource := &model.Resource{
		ID: "resource-local", UserID: "user-1", Kind: "image", Status: model.ResourceStatusReady,
		ObjectKey: "users/user-1/image/local.png",
	}

	delivery, err := svc.PrepareOwnedDelivery("user-1", resource, ResourceDeliveryOptions{})
	if err != nil {
		t.Fatalf("PrepareOwnedDelivery error = %v", err)
	}
	if delivery.RedirectURL != "" || delivery.Stream != nil {
		t.Fatalf("本地存储没有直连能力，应当回落同源代理: %#v", delivery)
	}
}
