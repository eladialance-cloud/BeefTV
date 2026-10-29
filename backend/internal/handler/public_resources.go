package handler

import (
	"infinite-canvas/backend/internal/app"

	"github.com/gin-gonic/gin"
)

// RegisterHostedPublicResourceRoutes exposes signed, read-only resource URLs
// used by upstream providers that cannot consume private workspace files.
// These routes are deliberately registered only by the server composition root.
func RegisterHostedPublicResourceRoutes(r *gin.RouterGroup, svc *app.Service) {
	serve := func(c *gin.Context) {
		stream, err := svc.OpenPublicResourceRange(
			c.Param("id"),
			c.Query("expires"),
			c.Query("signature"),
			c.GetHeader("Range"),
		)
		if err != nil {
			failService(c, err)
			return
		}
		defer stream.Body.Close()

		mimeType := "application/octet-stream"
		if stream.Resource != nil && stream.Resource.MimeType != "" {
			mimeType = stream.Resource.MimeType
		}
		headers := map[string]string{
			"Cache-Control": "private, no-store",
		}
		if stream.ContentRange != "" {
			headers["Content-Range"] = stream.ContentRange
		}
		if stream.AcceptRanges != "" {
			headers["Accept-Ranges"] = stream.AcceptRanges
		}
		c.DataFromReader(stream.StatusCode, stream.ContentLength, mimeType, stream.Body, headers)
	}

	r.GET("/public/resources/:id/file", serve)
	r.GET("/public/resources/:id/file/:filename", serve)
}
