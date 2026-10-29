package hostedapi

import (
	"strings"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func registerManjuCanvasRoutes(group *gin.RouterGroup, svc ManjuService, resolve ActorResolver) {
	actor := func(c *gin.Context) (*model.User, bool) {
		user, err := resolve(c)
		if err != nil {
			writeError(c, err)
			return nil, false
		}
		if user == nil || strings.TrimSpace(user.ID) == "" {
			writeError(c, kernel.Unauthorized("请先登录"))
			return nil, false
		}
		return user, true
	}

	group.GET("/projects/:projectId/canvas-packages/:stage", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.CanvasPackage(user.ID, c.Param("projectId"), c.Param("stage"), c.Query("runId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.GET("/projects/:projectId/canvas-imports", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		items, err := svc.CanvasImports(user.ID, c.Param("projectId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})

	group.GET("/projects/:projectId/canvas-imports/:stage/references", func(c *gin.Context) {
		c.Header("X-Manju-Reference-Name-Version", "1")
		c.Header("X-Manju-Reference-Prompt-Version", "1")
		user, ok := actor(c)
		if !ok {
			return
		}
		items, err := svc.CanvasImportReferences(user.ID, c.Param("projectId"), c.Param("stage"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})

	group.POST("/projects/:projectId/canvas-imports", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.CanvasImportInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("画布导入映射参数格式错误"))
			return
		}
		item, err := svc.SaveCanvasImport(user.ID, c.Param("projectId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})
}
