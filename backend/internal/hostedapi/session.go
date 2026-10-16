package hostedapi

import (
	handlercore "infinite-canvas/backend/internal/handler"

	"github.com/gin-gonic/gin"
)

// SessionMiddleware marks every request as hosted and resolves an optional
// login from the session cookie. It never rejects a request on its own: public
// routes (health, register, login) keep working, while protected routes fail
// inside currentUser. Marking the request is what makes a hosted deployment
// reject the shared workspace-owner fallback.
func SessionMiddleware(svc AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		handlercore.MarkHostedRequest(c)
		if token := sessionToken(c); token != "" {
			if user, err := svc.ValidateSession(token); err == nil {
				handlercore.SetSessionUser(c, user)
			}
		}
		c.Next()
	}
}
