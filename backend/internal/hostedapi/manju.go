package hostedapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func bindStrictJSON(c *gin.Context, target any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("JSON body must contain exactly one value")
		}
		return err
	}
	return nil
}

// ManjuService is the hosted project surface used by the manju studio. Session
// identity is passed explicitly; request paths and bodies never carry an actor.
type ManjuService interface {
	ListProjects(userID string) ([]manju.ProjectSummary, error)
	StyleCatalog() manju.StyleCatalogView
	RuleLibraryStatus(userID, projectID string) (*manju.RuleLibraryStatus, error)
	UserDeliverables(userID, projectID string) ([]manju.UserDeliverableView, error)
	ProductionRunDeliverables(userID, projectID, runID string) ([]manju.UserDeliverableView, error)
	ProjectDetail(userID, projectID string) (*manju.ProjectDetail, error)
	DeleteProject(actorID, projectID string) error
	SaveLock(actorID, projectID string, input manju.LockInput) (*manju.LockReceipt, error)
	SaveModels(actorID, projectID string, input manju.ModelBindingInput) (*manju.ProjectModelBinding, error)
	InviteMember(actorID, projectID string, input manju.MemberInput) (*model.ManjuProjectMember, error)
	RemoveMember(actorID, projectID, memberID string) error
	CreateSourceDocument(actorID, projectID string, input manju.SourceDocumentInput) (*manju.SourceDocumentDetail, error)
	SourceDocuments(actorID, projectID string) ([]manju.SourceDocumentView, error)
	SourceDocumentDetail(actorID, projectID, documentID string) (*manju.SourceDocumentDetail, error)
	SourceLibraryState(actorID, projectID string) (*manju.SourceLibraryState, error)
	CreateScriptRun(actorID, projectID string, input manju.ScriptRunInput) (*manju.ScriptRunDetail, error)
	ScriptRuns(actorID, projectID string) ([]manju.ScriptRunView, error)
	RegisterScriptSource(actorID, projectID, runID string, input manju.ScriptSourceInput) (*manju.ScriptRunDetail, error)
	ConfirmScriptStage(actorID, projectID, runID, stageKey string) (*manju.ScriptRunDetail, error)
	DecideRoundtable(actorID, projectID, runID string, input manju.RoundtableDecisionInput) (*manju.ScriptRunDetail, error)
	RunScriptStage(actorID, projectID, runID, stageKey string, input manju.ScriptStageRunInput) (*manju.ScriptRunDetail, error)
	RegenerateScriptArtifact(actorID, projectID, runID, stageKey string, input manju.ScriptStageRunInput) (*manju.ScriptRunDetail, error)
	ManualCorrectScriptArtifact(actorID, projectID, runID, stageKey string, input manju.ManualArtifactCorrectionInput) (*manju.ScriptRunDetail, error)
	ScriptRunDetail(actorID, projectID, runID string) (*manju.ScriptRunDetail, error)
	CreateAssetRun(actorID, projectID string, input manju.AssetRunInput) (*manju.AssetRunDetail, error)
	RegenerateAssetRun(actorID, projectID, runID string, input manju.AssetRegenerationInput) (*manju.AssetRunDetail, error)
	RegenerateAssetArtifact(actorID, projectID, runID, stageKey string, input manju.AssetStageRunInput) (*manju.AssetRunDetail, error)
	ManualCorrectAssetArtifact(actorID, projectID, runID, stageKey string, input manju.ManualArtifactCorrectionInput) (*manju.AssetRunDetail, error)
	AssetRuns(actorID, projectID string) ([]manju.AssetRunView, error)
	AssetRunDetail(actorID, projectID, runID string) (*manju.AssetRunDetail, error)
	RunAssetStage(actorID, projectID, runID, stageKey string, input manju.AssetStageRunInput) (*manju.AssetRunDetail, error)
	ConfirmAssetManifest(actorID, projectID, runID string) (*manju.AssetRunDetail, error)
	ConfirmAssetRun(actorID, projectID, runID string, input manju.AssetConfirmInput) (*manju.AssetRunDetail, error)
	RegisterAssetReference(actorID, projectID, runID string, input manju.AssetReferenceInput) (*manju.AssetRunDetail, error)
	CreateProductionRun(actorID, projectID string, input manju.ProductionRunInput) (*manju.ProductionRunDetail, error)
	ProductionRuns(actorID, projectID string) ([]manju.ProductionRunView, error)
	ProductionRunDetail(actorID, projectID, runID string) (*manju.ProductionRunDetail, error)
	RunProductionStage(actorID, projectID, runID, stageKey string, input manju.ProductionStageRunInput) (*manju.ProductionRunDetail, error)
	RegenerateProductionStage(actorID, projectID, runID, stageKey string) (*manju.ProductionRunDetail, error)
	RegenerateProductionArtifact(actorID, projectID, runID, stageKey string, input manju.ProductionStageRunInput) (*manju.ProductionRunDetail, error)
	ManualCorrectProductionArtifact(actorID, projectID, runID, stageKey string, input manju.ManualArtifactCorrectionInput) (*manju.ProductionRunDetail, error)
	PreviewProductionStoryboardCompile(actorID, projectID, runID, content string) (*manju.ManjuCompilePreview, error)
	GenerateProductionPrevis(actorID, projectID, runID string) (*manju.ProductionRunDetail, error)
	StartProductionStoryboardImages(actorID, projectID, runID string) (*manju.ProductionRunDetail, error)
	StartProductionVideos(actorID, projectID, runID string) (*manju.ProductionRunDetail, error)
	ConfirmProductionStage(actorID, projectID, runID, stageKey string, input manju.ProductionConfirmInput) (*manju.ProductionRunDetail, error)
	CanvasPackage(actorID, projectID, stage, runID string) (*manju.CanvasPackage, error)
	SaveCanvasImport(actorID, projectID string, input manju.CanvasImportInput) (*manju.CanvasImportView, error)
	CanvasImports(actorID, projectID string) ([]manju.CanvasImportView, error)
	CanvasImportReferences(actorID, projectID, stage string) ([]manju.CanvasImportReferenceView, error)
}

func RegisterManjuRoutes(r *gin.RouterGroup, svc ManjuService, resolve ActorResolver) {
	group := r.Group("/manju")
	registerManjuAssetRoutes(group, svc, resolve)
	registerManjuProductionRoutes(group, svc, resolve)
	registerManjuCanvasRoutes(group, svc, resolve)

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

	group.GET("/projects", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		items, err := svc.ListProjects(user.ID)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})

	group.GET("/style-catalog", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		writeOK(c, svc.StyleCatalog())
	})

	group.GET("/projects/:projectId/rule-library", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.RuleLibraryStatus(user.ID, c.Param("projectId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.GET("/projects/:projectId/deliverables", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		items, err := svc.UserDeliverables(user.ID, c.Param("projectId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		writeOK(c, gin.H{"items": items})
	})

	group.GET("/projects/:projectId/production-runs/:runId/deliverables", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		items, err := svc.ProductionRunDeliverables(user.ID, c.Param("projectId"), c.Param("runId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		writeOK(c, gin.H{"items": items})
	})

	group.GET("/projects/:projectId", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.ProjectDetail(user.ID, c.Param("projectId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.DELETE("/projects/:projectId", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		projectID := c.Param("projectId")
		if err := svc.DeleteProject(user.ID, projectID); err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"deleted": true, "id": projectID})
	})

	group.PUT("/projects/:projectId/lock", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.LockInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("P0 立项锁参数格式错误"))
			return
		}
		item, err := svc.SaveLock(user.ID, c.Param("projectId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/members", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.MemberInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("成员参数格式错误"))
			return
		}
		item, err := svc.InviteMember(user.ID, c.Param("projectId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.PUT("/projects/:projectId/models", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ModelBindingInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("模型绑定参数格式错误"))
			return
		}
		item, err := svc.SaveModels(user.ID, c.Param("projectId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.GET("/projects/:projectId/source-documents", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		items, err := svc.SourceDocuments(user.ID, c.Param("projectId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})

	group.GET("/projects/:projectId/source-library-state", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.SourceLibraryState(user.ID, c.Param("projectId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/source-documents", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<20)
		var input manju.SourceDocumentInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("原著文件读取失败或超过 16MB"))
			return
		}
		item, err := svc.CreateSourceDocument(user.ID, c.Param("projectId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.GET("/projects/:projectId/source-documents/:documentId", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.SourceDocumentDetail(user.ID, c.Param("projectId"), c.Param("documentId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/script-runs", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ScriptRunInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("剧本任务参数格式错误"))
			return
		}
		item, err := svc.CreateScriptRun(user.ID, c.Param("projectId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.GET("/projects/:projectId/script-runs", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		items, err := svc.ScriptRuns(user.ID, c.Param("projectId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})

	group.GET("/projects/:projectId/script-runs/:runId", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.ScriptRunDetail(user.ID, c.Param("projectId"), c.Param("runId"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/script-runs/:runId/source", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ScriptSourceInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("来源登记参数格式错误"))
			return
		}
		item, err := svc.RegisterScriptSource(user.ID, c.Param("projectId"), c.Param("runId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/script-runs/:runId/stages/:stageKey/run", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ScriptStageRunInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("阶段执行参数格式错误"))
			return
		}
		item, err := svc.RunScriptStage(user.ID, c.Param("projectId"), c.Param("runId"), c.Param("stageKey"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/script-runs/:runId/stages/:stageKey/regenerate-artifact", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ScriptStageRunInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("P1 单工件重新生成参数格式错误"))
			return
		}
		item, err := svc.RegenerateScriptArtifact(user.ID, c.Param("projectId"), c.Param("runId"), c.Param("stageKey"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/script-runs/:runId/stages/:stageKey/manual-correct", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.ManualArtifactCorrectionInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("P1 人工修正参数格式错误"))
			return
		}
		item, err := svc.ManualCorrectScriptArtifact(
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

	group.POST("/projects/:projectId/script-runs/:runId/stages/:stageKey/confirm", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.ConfirmScriptStage(user.ID, c.Param("projectId"), c.Param("runId"), c.Param("stageKey"))
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/projects/:projectId/script-runs/:runId/stages/roundtable_review/decide", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input manju.RoundtableDecisionInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("圆桌决策参数格式错误"))
			return
		}
		item, err := svc.DecideRoundtable(user.ID, c.Param("projectId"), c.Param("runId"), input)
		if err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.DELETE("/projects/:projectId/members/:memberId", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		if err := svc.RemoveMember(user.ID, c.Param("projectId"), c.Param("memberId")); err != nil {
			writeManjuError(c, err)
			return
		}
		writeOK(c, gin.H{"deleted": true})
	})
}

func writeManjuError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, manju.ErrNotFound):
		writeError(c, kernel.NotFound("漫剧项目不存在"))
	case errors.Is(err, manju.ErrForbidden):
		writeError(c, kernel.Forbidden("无漫剧项目操作权限"))
	case errors.Is(err, manju.ErrHasActiveTasks):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "项目仍有进行中的生成任务，请等待任务完成或取消后再删除"))
	case errors.Is(err, manju.ErrExplicitLockRequired):
		writeError(c, kernel.BadAuthRequest("P0 硬锁必须由用户逐项明确确认"))
	case errors.Is(err, manju.ErrSeamRequired):
		writeError(c, kernel.BadAuthRequest("目标时长需要多段生成，请先选择 D 段缝衔接方式"))
	case errors.Is(err, manju.ErrInvalidLock):
		writeError(c, kernel.BadAuthRequest("P0 立项锁参数无效"))
	case errors.Is(err, manju.ErrInvalidModelBinding):
		writeError(c, kernel.BadAuthRequest("只能绑定画布后台已启用且能力匹配的模型"))
	case errors.Is(err, manju.ErrInvalidMember):
		writeError(c, kernel.BadAuthRequest("项目成员参数无效"))
	case errors.Is(err, manju.ErrMemberNotFound):
		writeError(c, kernel.NotFound("项目成员不存在"))
	case errors.Is(err, manju.ErrP0Required):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "请先完成并锁定 P0 立项锁"))
	case errors.Is(err, manju.ErrScriptRunNotFound):
		writeError(c, kernel.NotFound("剧本任务不存在"))
	case errors.Is(err, manju.ErrScriptStageNotFound):
		writeError(c, kernel.NotFound("剧本阶段不存在"))
	case errors.Is(err, manju.ErrTrendResolutionRequired):
		writeError(c, kernel.BadAuthRequest("热点速成项目必须先选择按原创或对标执行"))
	case errors.Is(err, manju.ErrInvalidScriptRun):
		writeError(c, kernel.BadAuthRequest("剧本任务参数无效"))
	case errors.Is(err, manju.ErrScriptUnitInProgress):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "当前改编单元尚未完成，请完成并锁定本章后再创建下一章"))
	case errors.Is(err, manju.ErrSourceLibraryChanged):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "原著库已新增或修改内容，不能继续继承旧的 IP 与故事圣经，请关闭继承后重新生成全局阶段"))
	case errors.Is(err, manju.ErrInvalidScriptSource):
		writeError(c, kernel.BadAuthRequest(err.Error()))
	case errors.Is(err, manju.ErrScriptSourceNotFound):
		writeError(c, kernel.NotFound("原著来源不存在"))
	case errors.Is(err, manju.ErrRoundtableDecisionRequired):
		writeError(c, kernel.BadAuthRequest("请明确选择接受当前稿或精修重写，圆桌不能直接锁定"))
	case errors.Is(err, manju.ErrRoundtableQualityRiskRequired):
		writeError(c, kernel.BadAuthRequest("圆桌等级为 B/NOT_READY，接受当前稿必须明确确认质量风险"))
	case errors.Is(err, manju.ErrRoundtableDecisionEndpointRequired):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "圆桌审核只能通过专用决策接口处理"))
	case errors.Is(err, manju.ErrRoundtableGradeInvalid):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "圆桌审核等级缺失、冲突或格式不受支持，请重新生成审核"))
	case errors.Is(err, manju.ErrScriptStageNotReady):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "当前阶段尚不可确认或启动"))
	case errors.Is(err, manju.ErrScriptTaskUnavailable):
		writeError(c, kernel.NewAppError(kernel.CodeUnavailable, "漫剧文本任务服务尚未装配"))
	case errors.Is(err, manju.ErrAssetRunNotFound):
		writeError(c, kernel.NotFound("数字资产任务不存在"))
	case errors.Is(err, manju.ErrAssetStageNotFound):
		writeError(c, kernel.NotFound("数字资产阶段不存在"))
	case errors.Is(err, manju.ErrAssetScriptNotReady):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "请先完成与当前 P0 一致的 P1 剧本任务"))
	case errors.Is(err, manju.ErrAssetStageNotReady):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "当前数字资产阶段尚不可执行或确认"))
	case errors.Is(err, manju.ErrInvalidAssetRun):
		writeError(c, kernel.BadAuthRequest("数字资产任务参数无效"))
	case errors.Is(err, manju.ErrInvalidAssetReference):
		writeError(c, kernel.BadAuthRequest("M2 参考资产登记无效；只接受已入库模式且必须提供资产编码与资源 ID"))
	case errors.Is(err, manju.ErrAssetReferencesMissing):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "M2 资产交付编译前必须先登记全部参考图："+strings.TrimPrefix(err.Error(), manju.ErrAssetReferencesMissing.Error()+": ")))
	case errors.Is(err, manju.ErrProductionAssetNotReady):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "请先完成并锁定与当前 P0 一致的 P2 数字资产任务"))
	case errors.Is(err, manju.ErrProductionRunNotFound):
		writeError(c, kernel.NotFound("P3-P5 生产链不存在"))
	case errors.Is(err, manju.ErrProductionStageNotFound):
		writeError(c, kernel.NotFound("P3-P5 生产阶段不存在"))
	case errors.Is(err, manju.ErrProductionStageNotReady):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "当前生产阶段尚不可执行或确认，请先完成并锁定上一阶段"))
	case errors.Is(err, manju.ErrProductionMachineCheckProofInvalid):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "P4 缺少与当前锁定 P3 绑定的完整四段机检凭证，不能确认锁定"))
	case errors.Is(err, manju.ErrStoryboardImagesNotReady):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "故事板图片尚未全部生成并取得真实资源，P3 不能锁定"))
	case errors.Is(err, manju.ErrStoryboardImageInvalid):
		writeError(c, kernel.BadAuthRequest("故事板表无法解析为逐段图片任务"))
	case errors.Is(err, manju.ErrProductionVideosNotReady):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "P4 真实视频尚未全部生成并取得资源，投喂稿不能锁定"))
	case errors.Is(err, manju.ErrProductionPrevisRequired):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "P4.5 预演故事板尚未生成并通过 check_p45.py，P4 不能锁定"))
	case errors.Is(err, manju.ErrInvalidCanvasPackage):
		writeError(c, kernel.BadAuthRequest("画布生产包参数无效"))
	case errors.Is(err, manju.ErrCanvasPackageUnready):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, err.Error()))
	case errors.Is(err, manju.ErrCanvasImportInvalid):
		writeError(c, kernel.BadAuthRequest("画布必须属于当前漫剧项目并由当前用户创建"))
	case errors.Is(err, manju.ErrCanvasImportNotFound):
		writeError(c, kernel.NotFound("漫剧画布导入映射不存在"))
	case errors.Is(err, manju.ErrCanvasImportStale):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "上游工件已更新，请重新导入画布生产包"))
	case errors.Is(err, manju.ErrCanvasImportNotReady):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "画布中仍有任务未取得真实资源，当前阶段不能锁定"))
	case errors.Is(err, manju.ErrProductionVideoInvalid):
		writeError(c, kernel.BadAuthRequest("P4 投喂稿无法解析为逐段视频任务，请检查段号、时长和画幅"))
	case errors.Is(err, manju.ErrProductionVideoStarted):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "P4 视频批次已在生成中；只有失败段可以重试"))
	case errors.Is(err, manju.ErrInvalidProductionRun):
		writeError(c, kernel.BadAuthRequest("P3-P5 生产链参数无效；同一项目只能有一条进行中的生产链"))
	case errors.Is(err, manju.ErrProjectStateUnavailable):
		writeError(c, kernel.NewAppError(kernel.CodeConflict, "V7.4.26 project_state.json 尚未建立，请先完成 P0-P3 状态回写"))
	case errors.Is(err, manju.ErrInvalidProjectState):
		writeError(c, kernel.BadAuthRequest(err.Error()))
	case errors.Is(err, manju.ErrProductionArtifactInvalid):
		writeError(c, kernel.BadAuthRequest("生产阶段成果不符合阶段合同"))
	default:
		writeError(c, err)
	}
}
