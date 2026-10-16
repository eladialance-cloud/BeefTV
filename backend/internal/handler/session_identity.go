package handler

import (
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// The hosted session is published through plain gin context keys so this
// package never imports internal/auth or internal/hostedapi. The deskop path
// is untouched: when no hosted marker is present, currentUser keeps returning
// the local workspace owner exactly as before.
const (
	hostedAuthContextKey  = "canvas.hosted.auth"
	sessionUserContextKey = "canvas.session.user"
)

// MarkHostedRequest tells currentUser that this request arrived through the
// hosted auth middleware. A hosted request must resolve a session user and may
// never fall back to the shared workspace owner.
func MarkHostedRequest(c *gin.Context) {
	c.Set(hostedAuthContextKey, true)
}

// SetSessionUser publishes the authenticated session user for this request.
func SetSessionUser(c *gin.Context, user *model.User) {
	if user == nil {
		return
	}
	c.Set(sessionUserContextKey, user)
}

// SessionUser returns the session user published by the hosted middleware.
func SessionUser(c *gin.Context) (*model.User, bool) {
	value, exists := c.Get(sessionUserContextKey)
	if !exists {
		return nil, false
	}
	user, ok := value.(*model.User)
	if !ok || user == nil {
		return nil, false
	}
	return user, true
}

// IsHostedRequest reports whether the hosted auth middleware marked this
// request. It is exported so hosted-side tests can assert the marker without
// importing anything from this package's internals.
func IsHostedRequest(c *gin.Context) bool {
	value, exists := c.Get(hostedAuthContextKey)
	if !exists {
		return false
	}
	flag, ok := value.(bool)
	return ok && flag
}
