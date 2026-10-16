package model

import "time"

// IdentityProviderPassword is the built-in password identity provider. More
// providers (OAuth, email code) reuse the same identity row shape.
const IdentityProviderPassword = "password"

// UserIdentity is the hosted-only credential record. It is deliberately kept
// out of models_identity.go: the local/desktop identity projection must stay
// free of password material. The digest is excluded from JSON and never
// leaves the database layer.
type UserIdentity struct {
	ID           string    `json:"id" gorm:"primaryKey;size:36"`
	UserID       string    `json:"userId" gorm:"size:36;index;uniqueIndex:idx_user_identities_provider_identifier,priority:1"`
	Provider     string    `json:"provider" gorm:"size:32;uniqueIndex:idx_user_identities_provider_identifier,priority:2"`
	Identifier   string    `json:"identifier" gorm:"size:160;uniqueIndex:idx_user_identities_provider_identifier,priority:3"`
	PasswordHash string    `json:"-" gorm:"size:255"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (UserIdentity) TableName() string { return "user_identities" }

// AuthSession is the hosted-only login session. Only the token hash is stored;
// the clear token is returned once when the session is issued.
type AuthSession struct {
	ID        string     `json:"id" gorm:"primaryKey;size:36"`
	UserID    string     `json:"userId" gorm:"size:36;index"`
	TokenHash string     `json:"-" gorm:"size:64;uniqueIndex"`
	UserAgent string     `json:"userAgent" gorm:"size:255"`
	IP        string     `json:"ip" gorm:"size:64"`
	ExpiresAt time.Time  `json:"expiresAt" gorm:"index"`
	RevokedAt *time.Time `json:"revokedAt"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

func (AuthSession) TableName() string { return "auth_sessions" }
