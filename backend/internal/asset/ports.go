package asset

import (
	"io"
	"os"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

// BlobStore is the crash-safe byte owner for local resources. FileStore is the
// canonical local implementation; a hosted deployment adds an
// object-storage implementation. Callers must not write resource files beside
// it.
type BlobStore interface {
	Write(objectKey string, body io.Reader) error
	Open(objectKey string) (*os.File, error)
	Delete(objectKey string) error
	Exists(objectKey string) error
}

// BlobTarget is the durable identity of the storage location that owns a
// resource's bytes. It is persisted on the resource row so reads, retries and
// deletions keep resolving the same backend after the default location is
// switched. An empty Provider means a legacy local row.
type BlobTarget struct {
	SettingID string
	Provider  string
	Endpoint  string
	Bucket    string
}

// BlobResolver maps a storage location to its byte store. A local workspace
// installs a single-location resolver over its FileStore; a hosted deployment
// installs one backed by the platform object-storage configuration.
// Implementations must be safe for concurrent use.
type BlobResolver interface {
	// DefaultTarget is the location new writes must use.
	DefaultTarget() (BlobTarget, error)
	// Resolve returns the byte store for a recorded target. It must report an
	// error instead of silently falling back when the location is unknown.
	Resolve(target BlobTarget) (BlobStore, error)
	// LocalOnly reports whether this resolver serves bytes on this machine
	// exclusively. A local workspace is local-only; a hosted deployment with
	// object storage is not, and cloud rows are legitimate there.
	LocalOnly() bool
	// WriteSpace is the process-wide write-lock identity shared by every
	// Service using this resolver. Empty falls back to a per-Service identity.
	WriteSpace() string
}

// DirectURLResolver is the optional capability a BlobResolver implements when
// its stores can hand browsers a direct download address, so large files do
// not have to be proxied through this server.
type DirectURLResolver interface {
	// DirectURL returns a browser-usable URL for objectKey, or "" when this
	// location cannot serve one directly. stable reports whether the URL is a
	// long-lived public address (a CDN) rather than a short-lived signed one,
	// which decides whether the redirect may be cached.
	DirectURL(target BlobTarget, objectKey string, expires time.Time) (url string, stable bool, err error)
}

// Repository is the durable metadata port for resource rows, deletion outbox,
// and live-reference snapshots. Adapters wrap repository.Repository.
type Repository interface {
	CreateResource(*model.Resource) error
	SaveResource(*model.Resource) error
	Resource(id string) (*model.Resource, error)
	ResourceForUser(userID string, id string) (*model.Resource, error)
	Resources(userID string, limit int) ([]model.Resource, error)
	ResourceByUploadKey(userID string, uploadKey string) (*model.Resource, error)
	ListUploadReservations() ([]model.UserUploadReservation, error)
	UploadReservation(userID string, identity string) (*model.UserUploadReservation, error)
	ClearUploadReservation(userID string, identity string) error
	ReleaseIdentifiedDailyUpload(userID string, day string, identity string, size int64) error
	ClaimFailedResourceUpload(userID string, id string) (bool, error)
	DeleteResource(userID string, id string) error
	ResourcesForUserIDs(userID string, resourceIDs []string) ([]model.Resource, error)
	ResourceStorageReferenceCount(resource *model.Resource, excludedResourceIDs []string) (int64, error)
	ResourceReferenceSnapshot(userID string, excludingAssetID string, resourceIDs []string) (repository.ResourceReferenceSnapshot, error)
	DeleteAssetAndResources(userID string, assetID string, resourceIDs []string, deletionJobs []model.ResourceDeletionJob, expectedStatus ...string) error
	DeleteDetachedResources(resources []model.Resource, deletionJobs []model.ResourceDeletionJob) error
	ResourceCleanupCandidates(incompleteBefore time.Time, readyBefore time.Time, limit int) ([]model.Resource, error)
	ClaimNextResourceDeletionJob(owner string, leaseDuration time.Duration) (*model.ResourceDeletionJob, error)
	CompleteResourceDeletionJob(id string, owner string) error
	RetryResourceDeletionJob(id string, owner string, lastError string, nextAttemptAt time.Time) error
	CanvasHistoryReferencesObject(resource *model.Resource) (bool, error)
	AssetForUser(userID string, assetID string) (*model.Asset, error)
	AssetBusinessReferences(userID string, assetID string) ([]repository.ResourceDirectReference, error)
	AssetResourceRecords(assetID string) ([]model.AssetVersion, []model.AssetRepresentation, error)
	FindExpiredArchivedAssets(cutoff time.Time, limit int) ([]model.Asset, error)
}

// Quota is implemented by the application upload-quota owner. The domain never
// accounts bytes itself. Upload and generated artifacts share the same daily
// and storage ledger; only the single-file cap differs. identity scopes the
// in-memory pending storage entry so Release/Commit apply to this
// resource/artifact reservation, not another request's. Retry reserves are
// daily-only and do not create pending entries. Commit is a no-op when this
// identity has no pending reservation.
type Quota interface {
	ReserveUpload(userID string, size int64, identity string) (day string, err error)
	ReserveChunked(userID string, size int64, identity string) (day string, err error)
	ReserveRetry(userID string, size int64, identity string) (day string, err error)
	ReserveGenerated(userID string, size int64, identity string) (day string, err error)
	ReserveGeneratedRetry(userID string, size int64, identity string) (day string, err error)
	Release(userID string, day string, size int64, identity string)
	ReleaseRetry(userID string, day string, size int64, identity string)
	Commit(userID string, size int64, identity string)
}

// Lifecycle is the typed application seam for activity, playback, appearance
// references, recycle-bin policy, and background work. Methods are the port;
// the application adapter is a concrete type, not a bag of callbacks.
type Lifecycle interface {
	RecordActivity(userID string, kind string, count int)
	AfterResourceReady(resource *model.Resource)
	AppearanceReferencedIDs(resourceIDs []string) map[string]struct{}
	RecycleRetentionDays() (int, error)
	WorkerID() string
	RunBackground(func())
	DeleteUserAsset(userID string, assetID string) error
	WithStorageLock(func() error) error
}

// Dependencies constructs the durable resource domain.
type Dependencies struct {
	Repository Repository
	Blobs      BlobStore
	// Resolver takes precedence over Blobs when set: it routes bytes across
	// storage locations instead of pinning every resource to one store.
	Resolver     BlobResolver
	Quota        Quota
	Lifecycle    Lifecycle
	LocalStorage bool
	DataDir      string
}
