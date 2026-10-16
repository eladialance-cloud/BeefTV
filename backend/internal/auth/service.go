package auth

import (
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

// defaultSessionTTL is how long a freshly issued login session stays valid.
const defaultSessionTTL = 30 * 24 * time.Hour

// Sentinel causes surfaced to callers as structured errors.
var (
	ErrIdentifierTaken    = errors.New("该登录名已被注册")
	ErrInvalidCredentials = errors.New("登录名或密码错误")
	ErrAccountDisabled    = errors.New("账号已停用")
	ErrSessionInvalid     = errors.New("登录状态已失效")
)

// Config tunes the session lifetime and the clock. Tests inject a fixed clock;
// production uses time.Now.
type Config struct {
	SessionTTL time.Duration
	Now        func() time.Time
}

// Service is the hosted-only owner of registration, login and sessions.
type Service struct {
	repo Repository
	ttl  time.Duration
	now  func() time.Time
}

// New wires the account service. A nil repository is tolerated so callers can
// build the type before the database is ready; every method then fails closed.
func New(repo Repository, config Config) *Service {
	ttl := config.SessionTTL
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{repo: repo, ttl: ttl, now: now}
}

// RegisterInput is the self-service sign-up payload.
type RegisterInput struct {
	Username    string
	DisplayName string
	Password    string
}

// SessionMeta records request context on the session row for later revocation.
type SessionMeta struct {
	UserAgent string
	IP        string
}

// Register creates a normal user plus its password identity.
func (s *Service) Register(input RegisterInput) (*model.User, error) {
	if s == nil || s.repo == nil {
		return nil, kernel.Unauthorized("账号服务尚未初始化")
	}
	username := strings.TrimSpace(input.Username)
	if username == "" {
		return nil, kernel.BadAuthRequest("请填写登录名")
	}
	if _, exists, err := s.repo.UserIdentity(model.IdentityProviderPassword, username); err != nil {
		return nil, err
	} else if exists {
		return nil, kernel.BadAuthRequest(ErrIdentifierTaken.Error())
	}
	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, kernel.BadAuthRequest(err.Error())
	}
	display := strings.TrimSpace(input.DisplayName)
	if display == "" {
		display = username
	}
	now := s.now()
	user := &model.User{
		ID:          kernel.NewID(),
		Username:    username,
		DisplayName: display,
		Role:        model.UserRoleUser,
		Status:      model.UserStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	identity := &model.UserIdentity{
		ID:           kernel.NewID(),
		UserID:       user.ID,
		Provider:     model.IdentityProviderPassword,
		Identifier:   username,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.CreateUserWithIdentity(user, identity); err != nil {
		return nil, err
	}
	return user, nil
}

// Authenticate verifies a password login. Unknown identifiers and wrong
// passwords return the same error so the endpoint cannot enumerate accounts.
func (s *Service) Authenticate(identifier string, password string) (*model.User, error) {
	if s == nil || s.repo == nil {
		return nil, kernel.Unauthorized("账号服务尚未初始化")
	}
	identity, found, err := s.repo.UserIdentity(model.IdentityProviderPassword, strings.TrimSpace(identifier))
	if err != nil {
		return nil, err
	}
	if !found || !VerifyPassword(identity.PasswordHash, password) {
		return nil, kernel.Unauthorized(ErrInvalidCredentials.Error())
	}
	user, found, err := s.repo.UserByID(identity.UserID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, kernel.Unauthorized(ErrInvalidCredentials.Error())
	}
	if user.Status != model.UserStatusActive {
		return nil, kernel.Forbidden(ErrAccountDisabled.Error())
	}
	return user, nil
}

// IssueSession mints a clear token and stores only its hash.
func (s *Service) IssueSession(userID string, meta SessionMeta) (string, *model.AuthSession, error) {
	if s == nil || s.repo == nil {
		return "", nil, kernel.Unauthorized("账号服务尚未初始化")
	}
	token, tokenHash, err := NewSessionToken()
	if err != nil {
		return "", nil, err
	}
	now := s.now()
	session := &model.AuthSession{
		ID:        kernel.NewID(),
		UserID:    userID,
		TokenHash: tokenHash,
		UserAgent: truncate(meta.UserAgent, 255),
		IP:        truncate(meta.IP, 64),
		ExpiresAt: now.Add(s.ttl),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateAuthSession(session); err != nil {
		return "", nil, err
	}
	return token, session, nil
}

// ValidateSession resolves a clear token to its active user. Expired or
// revoked sessions and disabled accounts all fail closed.
func (s *Service) ValidateSession(token string) (*model.User, error) {
	if s == nil || s.repo == nil {
		return nil, kernel.Unauthorized(ErrSessionInvalid.Error())
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, kernel.Unauthorized(ErrSessionInvalid.Error())
	}
	session, found, err := s.repo.AuthSessionByTokenHash(HashSessionToken(token))
	if err != nil {
		return nil, err
	}
	if !found || session.RevokedAt != nil || !session.ExpiresAt.After(s.now()) {
		return nil, kernel.Unauthorized(ErrSessionInvalid.Error())
	}
	user, found, err := s.repo.UserByID(session.UserID)
	if err != nil {
		return nil, err
	}
	if !found || user.Status != model.UserStatusActive {
		return nil, kernel.Unauthorized(ErrSessionInvalid.Error())
	}
	return user, nil
}

// RevokeSession signs a token out. An unknown token is a no-op so logout is
// idempotent.
func (s *Service) RevokeSession(token string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	session, found, err := s.repo.AuthSessionByTokenHash(HashSessionToken(token))
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return s.repo.RevokeAuthSession(session.ID, s.now())
}

func truncate(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit]
}
