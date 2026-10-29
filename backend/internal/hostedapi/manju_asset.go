package hostedapi

import (
	"strings"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func registerManjuAssetRoutes(group *gin.RouterGroup, svc ManjuService, resolve ActorResolver) {
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

	group.POST("/projects/:projectId/asset-runs", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.AssetRunInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("数字资产任务参数格式错误"))
			return
		}
		item, err := svc.CreateAssetRun(user.ID, c.Param("projectId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.GET("/projects/:projectId/asset-runs", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		items, err := svc.AssetRuns(user.ID, c.Param("projectId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})

	group.GET("/projects/:projectId/asset-runs/:runId", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.AssetRunDetail(user.ID, c.Param("projectId"), c.Param("runId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/asset-runs/:runId/stages/:stageKey/run", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.AssetStageRunInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("资产阶段执行参数格式错误"))
			return
		}
		item, err := svc.RunAssetStage(user.ID, c.Param("projectId"), c.Param("runId"), c.Param("stageKey"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/asset-runs/:runId/regenerate", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.AssetRegenerationInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("P2 重新生成参数格式错误"))
			return
		}
		item, err := svc.RegenerateAssetRun(user.ID, c.Param("projectId"), c.Param("runId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/asset-runs/:runId/stages/:stageKey/regenerate-artifact", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.AssetStageRunInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("P2 工件重新生成参数格式错误"))
			return
		}
		item, err := svc.RegenerateAssetArtifact(
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

	group.POST("/projects/:projectId/asset-runs/:runId/stages/:stageKey/manual-correct", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ManualArtifactCorrectionInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("P2 人工修正参数格式错误"))
			return
		}
		item, err := svc.ManualCorrectAssetArtifact(
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

	group.POST("/projects/:projectId/asset-runs/:runId/confirm-manifest", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.ConfirmAssetManifest(user.ID, c.Param("projectId"), c.Param("runId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/asset-runs/:runId/confirm", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.AssetConfirmInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("资产最终确认参数格式错误"))
			return
		}
		item, err := svc.ConfirmAssetRun(user.ID, c.Param("projectId"), c.Param("runId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/asset-runs/:runId/references", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.AssetReferenceInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("资产参考图登记参数格式错误"))
			return
		}
		item, err := svc.RegisterAssetReference(user.ID, c.Param("projectId"), c.Param("runId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})
}
