package app

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	localasset "infinite-canvas/backend/internal/asset"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

// routedBlobStore stands in for the platform object-storage backend.
type routedBlobStore struct{ objects map[string][]byte }

func newRoutedBlobStore() *routedBlobStore {
	return &routedBlobStore{objects: map[string][]byte{}}
}

func (s *routedBlobStore) Write(objectKey string, body io.Reader) error {
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.objects[objectKey] = payload
	return nil
}

func (s *routedBlobStore) Open(objectKey string) (*os.File, error) {
	payload, ok := s.objects[objectKey]
	if !ok {
		return nil, os.ErrNotExist
	}
	file, err := os.CreateTemp("", "routed-blob-*")
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

func (s *routedBlobStore) Delete(objectKey string) error {
	delete(s.objects, objectKey)
	return nil
}

func (s *routedBlobStore) Exists(objectKey string) error {
	if _, ok := s.objects[objectKey]; !ok {
		return os.ErrNotExist
	}
	return nil
}

// routedAssetResolver is the injection stand-in for storageconfig.Router.
type routedAssetResolver struct {
	local  localasset.BlobStore
	remote localasset.BlobStore
}

func (r *routedAssetResolver) DefaultTarget() (localasset.BlobTarget, error) {
	return localasset.BlobTarget{SettingID: "storage-1", Provider: "s3", Endpoint: "https://oss.example.com", Bucket: "beeftv"}, nil
}

func (r *routedAssetResolver) Resolve(target localasset.BlobTarget) (localasset.BlobStore, error) {
	if target.Provider == "" || target.Provider == "local" {
		return r.local, nil
	}
	return r.remote, nil
}

func (r *routedAssetResolver) LocalOnly() bool    { return false }
func (r *routedAssetResolver) WriteSpace() string { return "app-routed-test" }

func newStorageRouteService(t *testing.T, resolver localasset.BlobResolver) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.UserDailyUploadUsage{}, &model.UserUploadReservation{}, &model.Resource{}); err != nil {
		t.Fatal(err)
	}
	return newService(repository.New(db), t.TempDir(), serviceOptions{mode: serviceModeLocal, blobResolver: resolver})
}

func TestInjectedBlobResolverRoutesResourceBytes(t *testing.T) {
	remote := newRoutedBlobStore()
	dataDir := t.TempDir()
	svc := newStorageRouteService(t, &routedAssetResolver{local: localasset.NewFileStore(dataDir), remote: remote})

	resource := &model.Resource{ID: "resource-routed", UserID: "local", Kind: "image", Status: model.ResourceStatusReady, MimeType: "image/png", Size: 5}
	if _, err := svc.storeResourceObject(resource, "routed.png", bytes.NewReader([]byte("hello"))); err != nil {
		t.Fatal(err)
	}
	if resource.Provider != "s3" || resource.StorageSettingID != "storage-1" {
		t.Fatalf("resource location = %q/%q", resource.Provider, resource.StorageSettingID)
	}
	if _, ok := remote.objects[resource.ObjectKey]; !ok {
		t.Fatalf("injected store missing %q", resource.ObjectKey)
	}
	if err := svc.repo.CreateResource(resource); err != nil {
		t.Fatal(err)
	}

	stream, err := svc.OpenResourceRange("local", resource.ID, "")
	if err != nil {
		t.Fatalf("OpenResourceRange error = %v", err)
	}
	defer stream.Body.Close()
	payload, err := io.ReadAll(stream.Body)
	if err != nil || string(payload) != "hello" {
		t.Fatalf("read back = %q err=%v", payload, err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "resources", filepath.FromSlash(resource.ObjectKey))); err == nil {
		t.Fatal("routed write must not also land in the local store")
	}
}

func TestLocalServiceStillRejectsCloudRows(t *testing.T) {
	svc := &Service{dataDir: t.TempDir(), mode: serviceModeLocal, localResourceStorage: true}
	resource := &model.Resource{
		ID: "resource-cloud", UserID: "local", Kind: "image", Status: model.ResourceStatusReady,
		Provider: "s3", ObjectKey: "users/local/image/cloud.png",
	}
	if _, err := svc.resourceDomain().OpenOwnedRange("local", resource, ""); err == nil {
		t.Fatal("a local-workspace domain accepted a cloud row")
	}
}

func TestRoutedUploadLocalResourceAcceptsObjectStorageResult(t *testing.T) {
	remote := newRoutedBlobStore()
	dataDir := t.TempDir()
	svc := newStorageRouteService(t, &routedAssetResolver{local: localasset.NewFileStore(dataDir), remote: remote})
	var body bytes.Buffer
	if err := png.Encode(&body, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	header := multipartFileHeader(t, "reference.png", "image/png", body.Bytes())
	resource, err := svc.ResourceService().UploadLocalResource("user-1", header, "image", 2, 2, 0)
	if err != nil {
		t.Fatalf("UploadLocalResource with routed object storage: %v", err)
	}
	if resource.Provider != "s3" || resource.Status != model.ResourceStatusReady {
		t.Fatalf("resource = %#v", resource)
	}
	if _, ok := remote.objects[resource.ObjectKey]; !ok {
		t.Fatalf("routed object missing: %q", resource.ObjectKey)
	}
}
