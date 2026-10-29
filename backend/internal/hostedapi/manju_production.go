package hostedapi

import (
	"strings"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func registerManjuProductionRoutes(group *gin.RouterGroup, svc ManjuService, resolve ActorResolver) {
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

	group.POST("/projects/:projectId/production-runs", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ProductionRunInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("P3-P5 生产链参数格式错误"))
			return
		}
		item, err := svc.CreateProductionRun(user.ID, c.Param("projectId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.GET("/projects/:projectId/production-runs", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		items, err := svc.ProductionRuns(user.ID, c.Param("projectId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})

	group.GET("/projects/:projectId/production-runs/:runId", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.ProductionRunDetail(user.ID, c.Param("projectId"), c.Param("runId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/production-runs/:runId/stages/:stageKey/run", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ProductionStageRunInput
		if err := bindStrictJSON(c, &input); err != nil {
			writeError(c, kernel.BadAuthRequest("生产阶段执行参数格式错误"))
			return
		}
		item, err := svc.RunProductionStage(user.ID, c.Param("projectId"), c.Param("runId"), c.Param("stageKey"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/production-runs/:runId/stages/:stageKey/regenerate", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.RegenerateProductionStage(user.ID, c.Param("projectId"), c.Param("runId"), c.Param("stageKey"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/production-runs/:runId/stages/:stageKey/regenerate-artifact", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ProductionStageRunInput
		if err := bindStrictJSON(c, &input); err != nil {
			writeError(c, kernel.BadAuthRequest("工件重新生成参数格式错误"))
			return
		}
		item, err := svc.RegenerateProductionArtifact(
			user.ID,
			c.Param("projectId"),
			c.Param("runId"),
			c.Param("stageKey"),
			input,
		)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/production-runs/:runId/stages/:stageKey/manual-correct", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ManualArtifactCorrectionInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("人工修正参数格式错误"))
			return
		}
		item, err := svc.ManualCorrectProductionArtifact(
			user.ID,
			c.Param("projectId"),
			c.Param("runId"),
			c.Param("stageKey"),
			input,
		)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/production-runs/:runId/storyboard-compile-preview", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ManualArtifactCorrectionInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("编译预览参数格式错误"))
			return
		}
		item, err := svc.PreviewProductionStoryboardCompile(
			user.ID, c.Param("projectId"), c.Param("runId"), input.Content,
		)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/production-runs/:runId/previs-storyboard", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.GenerateProductionPrevis(user.ID, c.Param("projectId"), c.Param("runId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/production-runs/:runId/storyboard-images", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.StartProductionStoryboardImages(user.ID, c.Param("projectId"), c.Param("runId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/production-runs/:runId/videos", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.StartProductionVideos(user.ID, c.Param("projectId"), c.Param("runId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/production-runs/:runId/stages/:stageKey/confirm", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ProductionConfirmInput
		if err := bindOptionalJSON(c, &input); err != nil {
			writeError(c, kernel.BadAuthRequest("生产阶段确认参数格式错误"))
			return
		}
		item, err := svc.ConfirmProductionStage(user.ID, c.Param("projectId"), c.Param("runId"), c.Param("stageKey"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})
}
