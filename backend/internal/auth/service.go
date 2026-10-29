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
	repo   Repository
	policy RegistrationPolicyPort
	ttl    time.Duration
	now    func() time.Time
}

// RegistrationPolicyPort 是站点级注册策略与邀请码核销的窄接口。
// 只由服务端组合根注入；未注入时注册保持开放，行为与旧版一致。
type RegistrationPolicyPort interface {
	RegistrationPolicyMode() (mode string, bonusCredits int64, err error)
}

// SetRegistrationPolicyPort 由组合根在构造后注入，保持 New 签名不变。
func (s *Service) SetRegistrationPolicyPort(port RegistrationPolicyPort) {
	if s != nil {
		s.policy = port
	}
}

// RegistrationPolicyMode exposes the current mode and signup bonus without
// leaking the policy adapter into the HTTP layer.
func (s *Service) RegistrationPolicyMode() (string, int64, error) {
	if s == nil || s.policy == nil {
		return "open", 0, nil
	}
	return s.policy.RegistrationPolicyMode()
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
	InviteCode  string
	// Role 仅供组合根赋予引导管理员；请求体永远不设置它。空值等于普通用户。
	Role model.UserRole
}

// SessionMeta records request context on the session row for later revocation.
type SessionMeta struct {
	UserAgent string
	IP        string
}

// Register creates a normal user plus its password identity.
func (s *Service) Register(input RegisterInput) (*model.User, error) {
	return s.createUser(input, true)
}

// CreateManagedUser is the administrator-only creation path. It keeps the same
// username uniqueness, password strength and credential transaction guarantees
// as self-service registration, but it does not consult the public registration
// policy or issue a login session.
func (s *Service) CreateManagedUser(input RegisterInput) (*model.User, error) {
	input.Role = model.UserRoleUser
	return s.createUser(input, false)
}

func (s *Service) createUser(input RegisterInput, enforceRegistrationPolicy bool) (*model.User, error) {
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
	role := input.Role
	if role == "" {
		role = model.UserRoleUser
	}
	now := s.now()
	user := &model.User{
		ID:          kernel.NewID(),
		Username:    username,
		DisplayName: display,
		Role:        role,
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
	create := func() error { return s.repo.CreateUserWithIdentity(user, identity) }
	if s.policy == nil {
		if err := create(); err != nil {
			return nil, err
		}
		return user, nil
	}
	if !enforceRegistrationPolicy {
		if err := create(); err != nil {
			return nil, err
		}
		return user, nil
	}
	mode, _, err := s.RegistrationPolicyMode()
	if err != nil {
		return nil, err
	}
	switch mode {
	case "closed":
		return nil, kernel.Forbidden("管理员已关闭注册")
	case "invite":
		code := strings.TrimSpace(input.InviteCode)
		if code == "" {
			return nil, kernel.BadAuthRequest("请输入邀请码")
		}
		if err := s.repo.ConsumeInviteCode(code, user, identity); err != nil {
			return nil, kernel.BadAuthRequest(err.Error())
		}
		return user, nil
	default:
		if err := create(); err != nil {
			return nil, err
		}
		return user, nil
	}
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

// ResetPassword replaces the hosted password credential without issuing a new
// session. Existing sessions remain valid until revoked; this endpoint is for
// credential recovery rather than session administration.
func (s *Service) ResetPassword(userID string, password string) error {
	if s == nil || s.repo == nil {
		return kernel.Unauthorized("账号服务尚未初始化")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return kernel.BadAuthRequest("缺少目标用户")
	}
	if _, found, err := s.repo.UserByID(userID); err != nil {
		return err
	} else if !found {
		return kernel.NotFound("用户不存在")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return kernel.BadAuthRequest(err.Error())
	}
	updated, err := s.repo.UpdatePasswordHash(userID, model.IdentityProviderPassword, hash, s.now())
	if err != nil {
		return err
	}
	if !updated {
		return kernel.NotFound("该用户没有密码登录凭据")
	}
	return nil
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
