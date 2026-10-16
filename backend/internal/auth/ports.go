// Package auth owns hosted account credentials and login sessions. It is
// imported only by the server profile composition root (cmd/server and
// internal/hostedapi), never by cmd/desktop: TestDesktopDependencyBoundary
// forbids internal/auth in the desktop dependency graph.
package auth

import (
	"time"

	"infinite-canvas/backend/internal/model"
)

// Repository is the durable port for hosted accounts. The GORM adapter lives in
// internal/repository; that adapter must not import this package, so every
// lookup returns an explicit found flag instead of leaking gorm.ErrRecordNotFound.
type Repository interface {
	// CreateUserWithIdentity writes the user and its credential atomically.
	CreateUserWithIdentity(user *model.User, identity *model.UserIdentity) error
	UserIdentity(provider string, identifier string) (*model.UserIdentity, bool, error)
	UserByID(id string) (*model.User, bool, error)
	UserByUsername(username string) (*model.User, bool, error)
	CreateAuthSession(session *model.AuthSession) error
	AuthSessionByTokenHash(tokenHash string) (*model.AuthSession, bool, error)
	RevokeAuthSession(id string, revokedAt time.Time) error
}
