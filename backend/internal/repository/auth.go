package repository

import (
	"errors"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateUserWithIdentity writes the hosted user and its credential in one
// transaction so a half-created account can never be logged into.
func (r *Repository) CreateUserWithIdentity(user *model.User, identity *model.UserIdentity) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		return tx.Create(identity).Error
	})
}

// UserIdentity looks up a hosted credential by provider and identifier. The
// found flag replaces gorm.ErrRecordNotFound so internal/auth stays free of
// gorm, and internal/repository never imports internal/auth.
func (r *Repository) UserIdentity(provider string, identifier string) (*model.UserIdentity, bool, error) {
	var item model.UserIdentity
	err := r.db.First(&item, "provider = ? AND identifier = ?", provider, identifier).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &item, true, nil
}

func (r *Repository) UserByID(id string) (*model.User, bool, error) {
	var item model.User
	err := r.db.First(&item, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &item, true, nil
}

func (r *Repository) UserByUsername(username string) (*model.User, bool, error) {
	var item model.User
	err := r.db.First(&item, "username = ?", username).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &item, true, nil
}

// UpdatePasswordHash replaces the password credential in place. The boolean
// reports whether a password identity existed for the user.
func (r *Repository) UpdatePasswordHash(userID string, provider string, passwordHash string, updatedAt time.Time) (bool, error) {
	result := r.db.Model(&model.UserIdentity{}).
		Where("user_id = ? AND provider = ?", userID, provider).
		Updates(map[string]any{"password_hash": passwordHash, "updated_at": updatedAt})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// CreateAuthSession keeps exactly one active login per account. The user row
// lock serializes concurrent logins, then all older sessions are revoked in the
// same transaction as the new session insert.
func (r *Repository) CreateAuthSession(session *model.AuthSession) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			First(&user, "id = ?", session.UserID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.AuthSession{}).
			Where("user_id = ? AND revoked_at IS NULL", session.UserID).
			Update("revoked_at", session.CreatedAt).Error; err != nil {
			return err
		}
		return tx.Create(session).Error
	})
}

func (r *Repository) AuthSessionByTokenHash(tokenHash string) (*model.AuthSession, bool, error) {
	var item model.AuthSession
	err := r.db.First(&item, "token_hash = ?", tokenHash).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &item, true, nil
}

// RevokeAuthSession stamps revoked_at once; a second call is a no-op.
func (r *Repository) RevokeAuthSession(id string, revokedAt time.Time) error {
	return r.db.Model(&model.AuthSession{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", revokedAt).Error
}
