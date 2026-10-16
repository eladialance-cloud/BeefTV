package asset

import (
	"errors"
	"strings"

	"infinite-canvas/backend/internal/model"
)

// namedBlobStore is the optional interface a byte store implements to publish
// its process-wide write-lock identity.
type namedBlobStore interface {
	WriteSpace() string
}

// singleLocationResolver is the local-workspace resolver: every resource lives
// in one byte store, so a row recorded on cloud storage cannot be served here.
type singleLocationResolver struct {
	store BlobStore
}

// SingleLocation adapts one byte store to the BlobResolver port. New writes are
// local; resolving anything else reports ResourceNotLocal so a database copied
// from a hosted deployment cannot silently read the wrong bytes.
func SingleLocation(store BlobStore) BlobResolver {
	return &singleLocationResolver{store: store}
}

func (r *singleLocationResolver) DefaultTarget() (BlobTarget, error) {
	if r == nil || r.store == nil {
		return BlobTarget{}, errors.New("local resource store is not initialized")
	}
	return BlobTarget{Provider: "local"}, nil
}

func (r *singleLocationResolver) Resolve(target BlobTarget) (BlobStore, error) {
	if r == nil || r.store == nil {
		return nil, errors.New("local resource store is not initialized")
	}
	switch normalizedProvider(target.Provider) {
	case "local":
		return r.store, nil
	default:
		return nil, ResourceNotLocal()
	}
}

func (r *singleLocationResolver) LocalOnly() bool { return true }

func (r *singleLocationResolver) WriteSpace() string {
	if r == nil || r.store == nil {
		return ""
	}
	named, ok := r.store.(namedBlobStore)
	if !ok {
		return ""
	}
	return strings.TrimSpace(named.WriteSpace())
}

// normalizedProvider folds the legacy empty provider onto "local".
func normalizedProvider(provider string) string {
	if trimmed := strings.ToLower(strings.TrimSpace(provider)); trimmed != "" {
		return trimmed
	}
	return "local"
}

// blobTargetFor reconstructs a resource's storage identity. An empty provider
// means a legacy local row.
func blobTargetFor(resource *model.Resource) BlobTarget {
	if resource == nil {
		return BlobTarget{}
	}
	return BlobTarget{
		SettingID: strings.TrimSpace(resource.StorageSettingID),
		Provider:  normalizedProvider(resource.Provider),
		Endpoint:  strings.TrimSpace(resource.Endpoint),
		Bucket:    strings.TrimSpace(resource.Bucket),
	}
}

// storeFor resolves the byte store that owns a resource's bytes. The resolver
// itself enforces a local workspace's "local rows only" rule, so this path has
// no extra gate to fall out of sync with.
func (s *Service) storeFor(resource *model.Resource) (BlobStore, error) {
	if s == nil || s.resolver == nil {
		return nil, errors.New("resource byte store is not initialized")
	}
	return s.resolver.Resolve(blobTargetFor(resource))
}

// defaultStore resolves the location new writes must use.
func (s *Service) defaultStore() (BlobTarget, BlobStore, error) {
	if s == nil || s.resolver == nil {
		return BlobTarget{}, nil, errors.New("resource byte store is not initialized")
	}
	target, err := s.resolver.DefaultTarget()
	if err != nil {
		return BlobTarget{}, nil, err
	}
	store, err := s.resolver.Resolve(target)
	if err != nil {
		return BlobTarget{}, nil, err
	}
	return target, store, nil
}

// LocalOnly reports whether this domain serves resource bytes on this machine
// exclusively. Application code uses it instead of inferring the boundary from
// a runtime mode flag.
func (s *Service) LocalOnly() bool {
	return s != nil && s.resolver != nil && s.resolver.LocalOnly()
}
