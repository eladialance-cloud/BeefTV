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
	// UpdatePasswordHash replaces the user's password credential in place.
	UpdatePasswordHash(userID string, provider string, passwordHash string, updatedAt time.Time) (bool, error)
	// ConsumeInviteCode 在同一事务里校验邀请码、创建用户与身份并写核销记录。
	ConsumeInviteCode(code string, user *model.User, identity *model.UserIdentity) error
}
