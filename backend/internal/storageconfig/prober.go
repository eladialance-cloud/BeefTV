package storageconfig

import (
	"context"
)

// Target is the resolved connection material handed to a storage prober.
// SecretAccessKey is plaintext here only for the duration of the probe.
type Target struct {
	Provider        string
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	Prefix          string
	PathStyle       bool
}

// Prober verifies that a storage configuration can connect, write, read and
// delete a test object. Implementations must not log or persist the secret.
type Prober interface {
	Probe(ctx context.Context, target Target) error
}
