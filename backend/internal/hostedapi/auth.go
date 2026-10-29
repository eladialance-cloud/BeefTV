package hostedapi

import (
	"log"
	"net/http"
	"strings"

	"infinite-canvas/backend/internal/auth"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// SessionCookieName is the hosted login cookie. It carries the clear session
// token; the server only ever stores its SHA-256 hash.
const SessionCookieName = "beeftv_session"

// sessionCookieMaxAge mirrors the default session TTL. The authoritative expiry
// check still happens server-side in auth.Service.ValidateSession.
const sessionCookieMaxAge = 30 * 24 * 60 * 60

// AuthService is the slice of internal/auth the HTTP surface needs. Keeping it
// an interface lets the routes be tested without a database.
type AuthService interface {
	Register(input auth.RegisterInput) (*model.User, error)
	Authenticate(identifier string, password string) (*model.User, error)
	IssueSession(userID string, meta auth.SessionMeta) (string, *model.AuthSession, error)
	ValidateSession(token string) (*model.User, error)
	RevokeSession(token string) error
	RegistrationPolicyMode() (mode string, bonusCredits int64, err error)
}

// RegisterAuthRoutes mounts the hosted-only account surface: /register, /login,
// /logout and /auth/me. Callers must register this only in the server profile;
// the desktop API test asserts these paths never appear on cmd/desktop.
// AuthOptions carries composition-root policy, never request data.
type AuthOptions struct {
	// BootstrapAdminUsername promotes the account registering under this exact
	// username to admin. Empty disables the bootstrap path. Operators should set
	// a non-guessable name and register it immediately, because the first
	// successful registration claims the name.
	BootstrapAdminUsername string
	// RegistrationBonus is best-effort after the account exists. The credit
	// service uses a stable idempotency key, so retrying cannot double-credit.
	RegistrationBonus func(user *model.User, amount int64) error
}

func RegisterAuthRoutes(r *gin.RouterGroup, svc AuthService, options AuthOptions) {
	r.POST("/register", func(c *gin.Context) {
		var body struct {
			Username    string `json:"username"`
			DisplayName string `json:"displayName"`
			Password    string `json:"password"`
			InviteCode  string `json:"inviteCode"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			writeError(c, kernel.BadAuthRequest("注册参数格式错误"))
			return
		}
		role := model.UserRoleUser
		if bootstrap := strings.TrimSpace(options.BootstrapAdminUsername); bootstrap != "" && strings.EqualFold(bootstrap, strings.TrimSpace(body.Username)) {
			role = model.UserRoleAdmin
		}
		user, err := svc.Register(auth.RegisterInput{
			Username: body.Username, DisplayName: body.DisplayName, Password: body.Password, InviteCode: body.InviteCode, Role: role,
		})
		if err != nil {
			writeError(c, err)
			return
		}
		if _, bonus, policyErr := svc.RegistrationPolicyMode(); policyErr == nil && bonus > 0 && options.RegistrationBonus != nil {
			if bonusErr := options.RegistrationBonus(user, bonus); bonusErr != nil {
				log.Printf("registration bonus failed user=%s amount=%d: %v", user.ID, bonus, bonusErr)
			}
		}
		if !issueSessionCookie(c, svc, user) {
			return
		}
		writeOK(c, gin.H{"user": user})
	})

	r.POST("/login", func(c *gin.Context) {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			writeError(c, kernel.BadAuthRequest("登录参数格式错误"))
			return
		}
		user, err := svc.Authenticate(body.Username, body.Password)
		if err != nil {
			writeError(c, err)
			return
		}
		if !issueSessionCookie(c, svc, user) {
			return
		}
		writeOK(c, gin.H{"user": user})
	})

	r.POST("/logout", func(c *gin.Context) {
		if token := sessionToken(c); token != "" {
			if err := svc.RevokeSession(token); err != nil {
				writeError(c, err)
				return
			}
		}
		clearSessionCookie(c)
		writeOK(c, gin.H{"loggedOut": true})
	})

	r.GET("/auth/me", func(c *gin.Context) {
		user, err := SessionUser(c, svc)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"user": user})
	})
}

// SessionUser resolves the current login from the session cookie. It is the
// seam Task 3 uses to make handler.currentUser read the hosted session.
func SessionUser(c *gin.Context, svc AuthService) (*model.User, error) {
	token := sessionToken(c)
	if token == "" {
		return nil, kernel.Unauthorized(auth.ErrSessionInvalid.Error())
	}
	return svc.ValidateSession(token)
}

// SessionUserResolver adapts SessionUser to the actor-resolver shape the admin
// routes already accept, so hosted wiring reuses one definition of "who is
// acting".
func SessionUserResolver(svc AuthService) func(*gin.Context) (*model.User, error) {
	return func(c *gin.Context) (*model.User, error) { return SessionUser(c, svc) }
}

func issueSessionCookie(c *gin.Context, svc AuthService, user *model.User) bool {
	token, _, err := svc.IssueSession(user.ID, auth.SessionMeta{
		UserAgent: c.Request.UserAgent(),
		IP:        c.ClientIP(),
	})
	if err != nil {
		writeError(c, err)
		return false
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, token, sessionCookieMaxAge, "/", "", requestIsSecure(c), true)
	return true
}

func clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, "", -1, "/", "", requestIsSecure(c), true)
}

func sessionToken(c *gin.Context) string {
	value, err := c.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

// requestIsSecure marks the cookie Secure whenever the browser reached us over
// TLS, directly or through a TLS-terminating proxy. Plain-HTTP development
// keeps working without a separate switch.
func requestIsSecure(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https")
}
