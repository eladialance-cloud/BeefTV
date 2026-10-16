package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

// memoryRepository is a pure in-memory auth repository. It deliberately avoids
// SQLite so these tests never trip the Windows temp-file handle cleanup issue.
type memoryRepository struct {
	users      map[string]*model.User
	identities map[string]*model.UserIdentity
	sessions   map[string]*model.AuthSession
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		users:      map[string]*model.User{},
		identities: map[string]*model.UserIdentity{},
		sessions:   map[string]*model.AuthSession{},
	}
}

func identityKey(provider string, identifier string) string {
	return provider + "\x00" + identifier
}

func (r *memoryRepository) CreateUserWithIdentity(user *model.User, identity *model.UserIdentity) error {
	key := identityKey(identity.Provider, identity.Identifier)
	if _, exists := r.identities[key]; exists {
		return errors.New("identity already exists")
	}
	userCopy := *user
	r.users[user.ID] = &userCopy
	identityCopy := *identity
	r.identities[key] = &identityCopy
	return nil
}

func (r *memoryRepository) UserIdentity(provider string, identifier string) (*model.UserIdentity, bool, error) {
	item, ok := r.identities[identityKey(provider, identifier)]
	if !ok {
		return nil, false, nil
	}
	copyItem := *item
	return &copyItem, true, nil
}

func (r *memoryRepository) UserByID(id string) (*model.User, bool, error) {
	item, ok := r.users[id]
	if !ok {
		return nil, false, nil
	}
	copyItem := *item
	return &copyItem, true, nil
}

func (r *memoryRepository) UserByUsername(username string) (*model.User, bool, error) {
	for _, item := range r.users {
		if item.Username == username {
			copyItem := *item
			return &copyItem, true, nil
		}
	}
	return nil, false, nil
}

func (r *memoryRepository) CreateAuthSession(session *model.AuthSession) error {
	if _, exists := r.sessions[session.TokenHash]; exists {
		return errors.New("session token hash already exists")
	}
	copyItem := *session
	r.sessions[session.TokenHash] = &copyItem
	return nil
}

func (r *memoryRepository) AuthSessionByTokenHash(tokenHash string) (*model.AuthSession, bool, error) {
	item, ok := r.sessions[tokenHash]
	if !ok {
		return nil, false, nil
	}
	copyItem := *item
	return &copyItem, true, nil
}

func (r *memoryRepository) RevokeAuthSession(id string, revokedAt time.Time) error {
	for _, item := range r.sessions {
		if item.ID == id && item.RevokedAt == nil {
			stamp := revokedAt
			item.RevokedAt = &stamp
		}
	}
	return nil
}

func testService(t *testing.T) (*Service, *memoryRepository) {
	t.Helper()
	repo := newMemoryRepository()
	fixed := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	return New(repo, Config{SessionTTL: time.Hour, Now: func() time.Time { return fixed }}), repo
}

func mustRegister(t *testing.T, svc *Service, username string, password string) *model.User {
	t.Helper()
	user, err := svc.Register(RegisterInput{Username: username, DisplayName: username, Password: password})
	if err != nil {
		t.Fatalf("Register(%q) error = %v", username, err)
	}
	return user
}

func TestHashPasswordRoundTripAndRejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatalf("HashPassword error = %v", err)
	}
	if !VerifyPassword(hash, "correct-horse") {
		t.Fatal("正确密码校验失败")
	}
	if VerifyPassword(hash, "wrong-horse") {
		t.Fatal("错误密码不应通过校验")
	}
	if VerifyPassword("", "correct-horse") {
		t.Fatal("空摘要不应通过校验")
	}
	if VerifyPassword("not-a-bcrypt-digest", "correct-horse") {
		t.Fatal("非法摘要不应 panic 或通过校验")
	}
}

func TestHashPasswordIsSalted(t *testing.T) {
	first, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword error = %v", err)
	}
	second, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword error = %v", err)
	}
	if first == second {
		t.Fatal("同一密码两次哈希必须不同（加盐）")
	}
	if strings.Contains(first, "same-password") {
		t.Fatal("摘要不得包含明文密码")
	}
}

func TestHashPasswordRejectsShortPasswords(t *testing.T) {
	if _, err := HashPassword("short"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("过短密码应返回 ErrWeakPassword，得到 %v", err)
	}
	if _, err := HashPassword("   "); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("空白密码应返回 ErrWeakPassword，得到 %v", err)
	}
}

func TestRegisterRejectsDuplicateIdentifier(t *testing.T) {
	svc, _ := testService(t)
	mustRegister(t, svc, "alice", "password-1")
	if _, err := svc.Register(RegisterInput{Username: "alice", Password: "password-2"}); err == nil {
		t.Fatal("重复登录名必须被拒绝")
	}
}

func TestAuthenticateRejectsUnknownUserAndWrongPassword(t *testing.T) {
	svc, _ := testService(t)
	mustRegister(t, svc, "alice", "password-1")

	if _, err := svc.Authenticate("alice", "wrong-password"); err == nil {
		t.Fatal("错误密码必须被拒绝")
	}
	if _, err := svc.Authenticate("nobody", "password-1"); err == nil {
		t.Fatal("未知用户必须被拒绝")
	}
	user, err := svc.Authenticate("alice", "password-1")
	if err != nil {
		t.Fatalf("正确凭据应通过，得到 %v", err)
	}
	if user.Username != "alice" || user.Role != model.UserRoleUser || user.Status != model.UserStatusActive {
		t.Fatalf("返回用户字段不符: %#v", user)
	}
}

func TestAuthenticateRejectsDisabledAccount(t *testing.T) {
	svc, repo := testService(t)
	user := mustRegister(t, svc, "bob", "password-1")
	repo.users[user.ID].Status = model.UserStatusDisabled

	_, err := svc.Authenticate("bob", "password-1")
	var appErr *kernel.AppError
	if !errors.As(err, &appErr) || appErr.Status != 403 {
		t.Fatalf("停用账号应返回 403，得到 %v", err)
	}
}

func TestIssueSessionStoresOnlyTokenHash(t *testing.T) {
	svc, repo := testService(t)
	user := mustRegister(t, svc, "carol", "password-1")

	token, session, err := svc.IssueSession(user.ID, SessionMeta{UserAgent: "test-agent", IP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("IssueSession error = %v", err)
	}
	if token == "" {
		t.Fatal("签发令牌不得为空")
	}
	if session.TokenHash == token || strings.Contains(session.TokenHash, token) {
		t.Fatal("会话行不得保存明文令牌")
	}
	if session.TokenHash != HashSessionToken(token) {
		t.Fatalf("会话行应保存令牌哈希，得到 %q", session.TokenHash)
	}
	if _, ok := repo.sessions[token]; ok {
		t.Fatal("仓储中不得出现明文令牌作为键")
	}
	if _, ok := repo.sessions[session.TokenHash]; !ok {
		t.Fatal("仓储中应存在令牌哈希")
	}
}

func TestValidateSessionReturnsUserUntilExpired(t *testing.T) {
	svc, _ := testService(t)
	user := mustRegister(t, svc, "dave", "password-1")
	token, _, err := svc.IssueSession(user.ID, SessionMeta{})
	if err != nil {
		t.Fatalf("IssueSession error = %v", err)
	}

	resolved, err := svc.ValidateSession(token)
	if err != nil {
		t.Fatalf("有效会话应通过，得到 %v", err)
	}
	if resolved.ID != user.ID {
		t.Fatalf("会话解析用户不符: %q vs %q", resolved.ID, user.ID)
	}
}

func TestValidateSessionRejectsExpiredAndRevoked(t *testing.T) {
	repo := newMemoryRepository()
	clock := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	svc := New(repo, Config{SessionTTL: time.Hour, Now: func() time.Time { return clock }})
	user := mustRegister(t, svc, "erin", "password-1")

	expiredToken, _, err := svc.IssueSession(user.ID, SessionMeta{})
	if err != nil {
		t.Fatalf("IssueSession error = %v", err)
	}
	clock = clock.Add(2 * time.Hour)
	if _, err := svc.ValidateSession(expiredToken); err == nil {
		t.Fatal("过期会话必须被拒绝")
	}

	clock = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	revokedToken, _, err := svc.IssueSession(user.ID, SessionMeta{})
	if err != nil {
		t.Fatalf("IssueSession error = %v", err)
	}
	if err := svc.RevokeSession(revokedToken); err != nil {
		t.Fatalf("RevokeSession error = %v", err)
	}
	if _, err := svc.ValidateSession(revokedToken); err == nil {
		t.Fatal("撤销后的会话必须立即失效")
	}
	if err := svc.RevokeSession(revokedToken); err != nil {
		t.Fatalf("重复登出必须幂等，得到 %v", err)
	}
	if err := svc.RevokeSession("unknown-token"); err != nil {
		t.Fatalf("未知令牌登出必须幂等，得到 %v", err)
	}
}

func TestValidateSessionRejectsUnknownAndEmptyToken(t *testing.T) {
	svc, _ := testService(t)
	if _, err := svc.ValidateSession(""); err == nil {
		t.Fatal("空令牌必须被拒绝")
	}
	if _, err := svc.ValidateSession("does-not-exist"); err == nil {
		t.Fatal("未知令牌必须被拒绝")
	}
}

func TestNewSessionTokenIsUniqueAndHashed(t *testing.T) {
	first, firstHash, err := NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken error = %v", err)
	}
	second, secondHash, err := NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken error = %v", err)
	}
	if first == second || firstHash == secondHash {
		t.Fatal("每次签发的令牌必须不同")
	}
	if HashSessionToken(first) != firstHash {
		t.Fatal("令牌哈希必须可复现")
	}
	if first == firstHash {
		t.Fatal("令牌哈希不得等于明文令牌")
	}
}

func TestUninitializedServiceFailsClosed(t *testing.T) {
	svc := New(nil, Config{})
	if _, err := svc.Register(RegisterInput{Username: "x", Password: "password-1"}); err == nil {
		t.Fatal("未初始化的服务注册必须失败")
	}
	if _, err := svc.Authenticate("x", "password-1"); err == nil {
		t.Fatal("未初始化的服务登录必须失败")
	}
	if _, err := svc.ValidateSession("token"); err == nil {
		t.Fatal("未初始化的服务校验会话必须失败")
	}
	if err := svc.RevokeSession("token"); err != nil {
		t.Fatalf("未初始化的服务登出应为 no-op，得到 %v", err)
	}
}
