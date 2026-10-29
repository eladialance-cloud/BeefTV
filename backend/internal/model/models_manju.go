package model

import "time"

const (
	ManjuScriptModeStepwise = "STEPWISE"
	ManjuAssetModeM0        = "M0"
	ManjuAssetModeM1        = "M1"
	ManjuAssetModeM2        = "M2"

	ManjuAssetRunStatusActive     = "ACTIVE"
	ManjuAssetRunStatusCompleted  = "COMPLETED"
	ManjuAssetRunStatusSuperseded = "SUPERSEDED"

	ManjuAssetStageStatusNotStarted           = "NOT_STARTED"
	ManjuAssetStageStatusInProgress           = "IN_PROGRESS"
	ManjuAssetStageStatusAwaitingConfirmation = "AWAITING_CONFIRMATION"
	ManjuAssetStageStatusLocked               = "LOCKED"
	ManjuAssetStageStatusStale                = "STALE"
	ManjuAssetStageStatusBlocked              = "BLOCKED"

	ManjuAssetStageExtraction = "entity_extraction"
	ManjuAssetStageManifest   = "asset_manifest"
	ManjuAssetStageCompile    = "asset_compile"

	ManjuAssetArtifactExtraction = "ENTITY_INVENTORY.md"
	ManjuAssetArtifactManifest   = "ASSET_MANIFEST.md"
	ManjuAssetArtifactM0         = "APPEARANCE_ARCHIVE_M0.md"
	ManjuAssetArtifactM1         = "ASSET_NUMBERING_AND_WIRING_M1.md"
	ManjuAssetArtifactM2         = "ASSET_CATALOG_M2.md"

	ManjuAssetReferenceStatusPending = "PENDING"
	ManjuAssetReferenceStatusReady   = "READY"

	ManjuProductionRunStatusActive     = "ACTIVE"
	ManjuProductionRunStatusCompleted  = "COMPLETED"
	ManjuProductionRunStatusSuperseded = "SUPERSEDED"

	ManjuProductionStageStatusNotStarted           = "NOT_STARTED"
	ManjuProductionStageStatusInProgress           = "IN_PROGRESS"
	ManjuProductionStageStatusAwaitingConfirmation = "AWAITING_CONFIRMATION"
	ManjuProductionStageStatusLocked               = "LOCKED"
	ManjuProductionStageStatusBlocked              = "BLOCKED"
	ManjuProductionStageStatusStale                = "STALE"

	ManjuProductionStageDecisionAnchor = "decision_anchor"
	ManjuProductionStageStoryboard     = "director_storyboard"
	ManjuProductionStageFeed           = "feed_package"
	ManjuProductionStageQC             = "qc_handoff"

	ManjuProductionArtifactEmotionAnchor = "EMOTION_ANCHOR_TABLE.md"
	ManjuProductionArtifactStoryboard    = "DIRECTOR_STORYBOARD.md"
	ManjuProductionArtifactFeed          = "FEED_PACKAGE.md"
	ManjuProductionArtifactPrevis        = "PREVIS_STORYBOARD.md"
	ManjuProductionArtifactQC            = "QC_HANDOFF.md"

	ManjuDeliverableKindMarkdown = "markdown"
	ManjuDeliverableKindBoard    = "board"
	ManjuDeliverableKindPrevis   = "previs"
	ManjuDeliverableKindManifest = "manifest"

	ManjuProductionMachineCheckStatusPass               = "PASS"
	ManjuProductionMachineCheckStatusLegacyNonCompliant = "LEGACY_NON_COMPLIANT"

	ManjuProductionPrevisStatusQueued  = "QUEUED"
	ManjuProductionPrevisStatusRunning = "RUNNING"
	ManjuProductionPrevisStatusReady   = "READY"
	ManjuProductionPrevisStatusFailed  = "FAILED"

	ManjuProductionVideoStatusQueued     = "QUEUED"
	ManjuProductionVideoStatusRunning    = "RUNNING"
	ManjuProductionVideoStatusReady      = "READY"
	ManjuProductionVideoStatusFailed     = "FAILED"
	ManjuProductionImageStatusQueued     = "QUEUED"
	ManjuProductionImageStatusRunning    = "RUNNING"
	ManjuProductionImageStatusReady      = "READY"
	ManjuProductionImageStatusFailed     = "FAILED"
	ManjuProductionImageStatusSuperseded = "SUPERSEDED"
	ManjuScriptModeFullPipeline          = "FULL_SCRIPT_PIPELINE"

	ManjuCanvasStageP2Assets = "p2_assets"
	ManjuCanvasStageP3Images = "p3_images"
	ManjuCanvasStageP4Videos = "p4_videos"
	ManjuCanvasImportActive  = "ACTIVE"
	ManjuCanvasImportStale   = "STALE"

	ManjuScriptBranchOriginal   = "original"
	ManjuScriptBranchAdaptation = "adaptation"
	ManjuScriptBranchBenchmark  = "benchmark"
	ManjuScriptBranchPolish     = "polish"

	ManjuScriptRunStatusActive    = "ACTIVE"
	ManjuScriptRunStatusCompleted = "COMPLETED"
	ManjuScriptRunStatusAbandoned = "ABANDONED"

	ManjuScriptStageStatusNotStarted       = "NOT_STARTED"
	ManjuScriptStageStatusInProgress       = "IN_PROGRESS"
	ManjuScriptStageStatusAwaitingConfirm  = "AWAITING_CONFIRMATION"
	ManjuScriptStageStatusAwaitingDecision = "AWAITING_USER_DECISION"
	ManjuScriptStageStatusLocked           = "LOCKED"
	ManjuScriptStageStatusStale            = "STALE"
	ManjuScriptStageStatusBlocked          = "BLOCKED"

	ManjuScriptStageSourceRegistration = "source_registration"
	ManjuScriptStageReconIntelligence  = "recon_intelligence"
	ManjuScriptStageIPDevelopment      = "ip_development"
	ManjuScriptStageSeriesBible        = "series_bible"
	ManjuScriptStageEpisodePlanning    = "episode_planning"
	ManjuScriptStageScriptDraft        = "script_draft"
	ManjuScriptStageRoundtableReview   = "roundtable_review"
	ManjuScriptStagePolishDraft        = "polish_draft"
	ManjuScriptStagePolishFindings     = "polish_findings"
	ManjuScriptStagePolishConfirmation = "polish_confirmation"

	ManjuScriptArtifactSourceManifest = "SOURCE_MANIFEST.json"
	ManjuScriptArtifactReconCard      = "RECON_INTELLIGENCE_CARD.md"
	ManjuScriptArtifactRoundtable     = "ROUNDTABLE_REVIEW.md"
	ManjuScriptArtifactStageOutput    = "STAGE_OUTPUT.md"
	ManjuScriptArtifactPolishedScript = "POLISHED_SCRIPT.md"
	ManjuScriptArtifactPolishFindings = "POLISH_FINDINGS.md"
	ManjuScriptArtifactPolishConfirm  = "POLISH_CONFIRM_CARD.md"

	ManjuMemberRoleOwner    = "owner"
	ManjuMemberRoleProducer = "producer"
	ManjuMemberRoleWriter   = "writer"
	ManjuMemberRoleDirector = "director"
	ManjuMemberRoleReviewer = "reviewer"
	ManjuMemberRoleViewer   = "viewer"

	ManjuMemberStatusActive   = "active"
	ManjuMemberStatusDisabled = "disabled"

	ManjuLockStatusLocked     = "locked"
	ManjuLockStatusSuperseded = "superseded"

	ManjuArtifactVisibilityUser     = "user"
	ManjuArtifactVisibilityInternal = "internal"
	ManjuArtifactVisibilityAudit    = "audit"

	ManjuModelExchangeStatusQueued    = "QUEUED"
	ManjuModelExchangeStatusRunning   = "RUNNING"
	ManjuModelExchangeStatusSucceeded = "SUCCEEDED"
	ManjuModelExchangeStatusFailed    = "FAILED"
)

// ManjuProjectMember grants a hosted user access to a project. The project
// owner remains on Project.UserID and is never stored as a collaborator row.
type ManjuProjectMember struct {
	ID        string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID string    `json:"projectId" gorm:"size:36;index;uniqueIndex:idx_manju_project_member,priority:1"`
	UserID    string    `json:"userId" gorm:"size:36;index;uniqueIndex:idx_manju_project_member,priority:2"`
	Role      string    `json:"role" gorm:"size:24;index"`
	Status    string    `json:"status" gorm:"size:24;index"`
	InvitedBy string    `json:"invitedBy" gorm:"size:36;index"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (ManjuProjectMember) TableName() string { return "manju_project_members" }

// ManjuLockReceipt is an immutable P0 decision snapshot. A later decision is a
// new revision; the previous row is retained and marked superseded.
type ManjuLockReceipt struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID   string    `json:"projectId" gorm:"size:36;index;uniqueIndex:idx_manju_lock_revision,priority:1"`
	Revision    int64     `json:"revision" gorm:"uniqueIndex:idx_manju_lock_revision,priority:2"`
	Status      string    `json:"status" gorm:"size:24;index"`
	PayloadJSON string    `json:"payloadJson" gorm:"type:text"`
	ContentHash string    `json:"contentHash" gorm:"size:64"`
	CreatedBy   string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt   time.Time `json:"createdAt" gorm:"index"`
}

func (ManjuLockReceipt) TableName() string { return "manju_lock_receipts" }

// ManjuProjectStateSnapshot is an immutable V7.4.26 project_state.json
// revision. Patches are merged only after the corresponding stage is
// confirmed, so P4 can compile from a stable cross-stage ledger.
type ManjuProjectStateSnapshot struct {
	ID            string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID     string    `json:"projectId" gorm:"size:36;index"`
	LockRevision  int64     `json:"lockRevision" gorm:"index"`
	StageKey      string    `json:"stageKey" gorm:"size:64;index"`
	ArtifactID    string    `json:"artifactId,omitempty" gorm:"size:36;index"`
	SchemaVersion string    `json:"schemaVersion" gorm:"size:24;index"`
	StateJSON     string    `json:"stateJson" gorm:"type:text"`
	ContentHash   string    `json:"contentHash" gorm:"size:64;index"`
	CreatedBy     string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt     time.Time `json:"createdAt" gorm:"index"`
}

func (ManjuProjectStateSnapshot) TableName() string { return "manju_project_states" }

// ManjuRuleSnapshot pins one immutable rule-tree revision to a project lock.
// ManifestJSON contains every file path and SHA256; model requests can therefore
// be replayed against the exact same rule bytes.
type ManjuRuleSnapshot struct {
	ID           string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID    string    `json:"projectId" gorm:"size:36;index;uniqueIndex:idx_manju_rule_snapshot_lock,priority:1"`
	LockRevision int64     `json:"lockRevision" gorm:"uniqueIndex:idx_manju_rule_snapshot_lock,priority:2"`
	RulesVersion string    `json:"rulesVersion" gorm:"size:32;index"`
	SourceCommit string    `json:"sourceCommit" gorm:"size:64;index"`
	MasterPath   string    `json:"masterPath" gorm:"size:240"`
	MasterSHA256 string    `json:"masterSha256" gorm:"size:64"`
	TreeSHA256   string    `json:"treeSha256" gorm:"size:64;index"`
	ManifestJSON string    `json:"manifestJson" gorm:"type:text"`
	CreatedBy    string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt    time.Time `json:"createdAt" gorm:"index"`
}

func (ManjuRuleSnapshot) TableName() string { return "manju_rule_snapshots" }

// ManjuRuleReadReceipt records exact rule files supplied to one stage.
// StartByte/EndByte use an end-exclusive byte range and may cover a whole file.
type ManjuRuleReadReceipt struct {
	ID            string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID     string    `json:"projectId" gorm:"size:36;index"`
	RunID         string    `json:"runId" gorm:"size:36;index"`
	StageID       string    `json:"stageId" gorm:"size:36;index"`
	SnapshotID    string    `json:"snapshotId" gorm:"size:36;index"`
	StageKey      string    `json:"stageKey" gorm:"size:64;index"`
	RulePath      string    `json:"rulePath" gorm:"size:320;index"`
	StartByte     int64     `json:"startByte"`
	EndByte       int64     `json:"endByte"`
	ContentSHA256 string    `json:"contentSha256" gorm:"size:64;index"`
	RequestID     string    `json:"requestId" gorm:"size:120;index"`
	ToolName      string    `json:"toolName" gorm:"size:48"`
	CreatedAt     time.Time `json:"createdAt" gorm:"index"`
}

func (ManjuRuleReadReceipt) TableName() string { return "manju_rule_read_receipts" }

// ManjuModelExchange preserves the complete prompt/tool payload and raw model
// response before any parsing, validation or display projection.
type ManjuModelExchange struct {
	ID             string     `json:"id" gorm:"primaryKey;size:36"`
	ProjectID      string     `json:"projectId" gorm:"size:36;index"`
	RunID          string     `json:"runId" gorm:"size:36;index"`
	StageID        string     `json:"stageId" gorm:"size:36;index"`
	TaskID         string     `json:"taskId,omitempty" gorm:"size:36;index"`
	StageKey       string     `json:"stageKey" gorm:"size:64;index"`
	RequestID      string     `json:"requestId" gorm:"size:120;index;uniqueIndex"`
	RequestJSON    string     `json:"requestJson" gorm:"type:text"`
	RequestSHA256  string     `json:"requestSha256" gorm:"size:64;index"`
	RawResponse    string     `json:"rawResponse" gorm:"type:text"`
	ResponseSHA256 string     `json:"responseSha256" gorm:"size:64;index"`
	Status         string     `json:"status" gorm:"size:24;index"`
	Error          string     `json:"error,omitempty" gorm:"type:text"`
	CreatedAt      time.Time  `json:"createdAt" gorm:"index"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
}

func (ManjuModelExchange) TableName() string { return "manju_model_exchanges" }

// ManjuArtifactLedger indexes every generated artifact without replacing the
// phase-specific immutable artifact tables. Visibility controls which records
// the user-facing workflow may project.
type ManjuArtifactLedger struct {
	ID              string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID       string    `json:"projectId" gorm:"size:36;index"`
	RunID           string    `json:"runId" gorm:"size:36;index"`
	StageID         string    `json:"stageId" gorm:"size:36;index"`
	ArtifactID      string    `json:"artifactId" gorm:"size:36;index;uniqueIndex:idx_manju_artifact_ledger_artifact"`
	Phase           string    `json:"phase" gorm:"size:8;index"`
	Kind            string    `json:"kind" gorm:"size:120;index"`
	Version         int64     `json:"version"`
	Visibility      string    `json:"visibility" gorm:"size:16;index"`
	ContentSHA256   string    `json:"contentSha256" gorm:"size:64;index"`
	ModelExchangeID string    `json:"modelExchangeId,omitempty" gorm:"size:36;index"`
	CreatedBy       string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt       time.Time `json:"createdAt" gorm:"index"`
}

func (ManjuArtifactLedger) TableName() string { return "manju_artifact_ledger" }

// ManjuProjectModelBinding is the project-level routing choice for each
// generation capability. Only enabled system catalog models may be stored.
type ManjuProjectModelBinding struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID      string    `json:"projectId" gorm:"size:36;uniqueIndex;not null"`
	TextChannelID  string    `json:"textChannelId" gorm:"size:36"`
	TextModelKey   string    `json:"textModelKey" gorm:"size:120"`
	ImageChannelID string    `json:"imageChannelId" gorm:"size:36"`
	ImageModelKey  string    `json:"imageModelKey" gorm:"size:120"`
	VideoChannelID string    `json:"videoChannelId" gorm:"size:36"`
	VideoModelKey  string    `json:"videoModelKey" gorm:"size:120"`
	UpdatedBy      string    `json:"updatedBy" gorm:"size:36;index"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (ManjuProjectModelBinding) TableName() string { return "manju_project_model_bindings" }

// ManjuScriptRun is the durable P1 envelope. It is tied to one immutable P0
// lock revision and one concrete script branch.
type ManjuScriptRun struct {
	ID                   string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID            string    `json:"projectId" gorm:"size:36;index"`
	Branch               string    `json:"branch" gorm:"size:24;index"`
	Mode                 string    `json:"mode" gorm:"size:32;index"`
	InputKind            string    `json:"inputKind" gorm:"size:32;index"`
	TrendResolution      string    `json:"trendResolution" gorm:"size:24;index"`
	Status               string    `json:"status" gorm:"size:24;index"`
	CurrentStage         string    `json:"currentStage" gorm:"size:48;index"`
	LockRevision         int64     `json:"lockRevision"`
	SourceID             string    `json:"sourceId" gorm:"size:36;index"`
	UnitTitle            string    `json:"unitTitle" gorm:"size:240"`
	SourceDocumentID     string    `json:"sourceDocumentId" gorm:"size:36;index"`
	SourceChapterIDsJSON string    `json:"-" gorm:"type:text"`
	SourceRangeLabel     string    `json:"sourceRangeLabel" gorm:"size:1000"`
	SourceSetHash        string    `json:"sourceSetHash" gorm:"size:64;index"`
	BaseRunID            string    `json:"baseRunId" gorm:"size:36;index"`
	ReuseGlobalStages    bool      `json:"reuseGlobalStages"`
	CreatedBy            string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt            time.Time `json:"createdAt" gorm:"index"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

func (ManjuScriptRun) TableName() string { return "manju_script_runs" }

// ManjuSourceDocument is the project-level novel/script source library.
// Chapters are immutable rows so a later unit can select a stable range even
// after another source document is uploaded.
type ManjuSourceDocument struct {
	ID                 string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID          string    `json:"projectId" gorm:"size:36;index"`
	Title              string    `json:"title" gorm:"size:240"`
	AuthorizationScope string    `json:"authorizationScope" gorm:"type:text"`
	FileName           string    `json:"fileName" gorm:"size:240"`
	ContentHash        string    `json:"contentHash" gorm:"size:64;index"`
	CharacterCount     int64     `json:"characterCount"`
	ChapterCount       int       `json:"chapterCount"`
	CreatedBy          string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt          time.Time `json:"createdAt" gorm:"index"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

func (ManjuSourceDocument) TableName() string { return "manju_source_documents" }

type ManjuSourceChapter struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID      string    `json:"projectId" gorm:"size:36;index"`
	DocumentID     string    `json:"documentId" gorm:"size:36;index;uniqueIndex:idx_manju_source_chapter_position,priority:1"`
	Position       int       `json:"position" gorm:"uniqueIndex:idx_manju_source_chapter_position,priority:2"`
	Title          string    `json:"title" gorm:"size:240"`
	RangeLabel     string    `json:"rangeLabel" gorm:"size:240"`
	ContentText    string    `json:"-" gorm:"type:text"`
	ContentHash    string    `json:"contentHash" gorm:"size:64;index"`
	CharacterCount int64     `json:"characterCount"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (ManjuSourceChapter) TableName() string { return "manju_source_chapters" }

type ManjuScriptStage struct {
	ID          string     `json:"id" gorm:"primaryKey;size:36"`
	RunID       string     `json:"runId" gorm:"size:36;index;uniqueIndex:idx_manju_script_stage_key,priority:1"`
	Key         string     `json:"key" gorm:"size:48;uniqueIndex:idx_manju_script_stage_key,priority:2"`
	Title       string     `json:"title" gorm:"size:120"`
	Position    int        `json:"position" gorm:"index"`
	Status      string     `json:"status" gorm:"size:32;index"`
	Version     int64      `json:"version"`
	TaskID      string     `json:"taskId,omitempty" gorm:"size:36;index"`
	ArtifactID  string     `json:"artifactId,omitempty" gorm:"size:36;index"`
	InputJSON   string     `json:"-" gorm:"type:text"`
	Error       string     `json:"error,omitempty" gorm:"type:text"`
	ConfirmedBy string     `json:"confirmedBy,omitempty" gorm:"size:36"`
	ConfirmedAt *time.Time `json:"confirmedAt,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func (ManjuScriptStage) TableName() string { return "manju_script_stages" }

// ManjuScriptSource is the source ledger used by the P1 branch. Content is
// addressed by its SHA-256 hash; API views never expose the raw source text.
type ManjuScriptSource struct {
	ID                 string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID          string    `json:"projectId" gorm:"size:36;index"`
	RunID              string    `json:"runId" gorm:"size:36;uniqueIndex"`
	Branch             string    `json:"branch" gorm:"size:24;index"`
	Title              string    `json:"title" gorm:"size:240"`
	AuthorizationScope string    `json:"authorizationScope" gorm:"type:text"`
	RangeLabel         string    `json:"rangeLabel" gorm:"size:240"`
	ContentText        string    `json:"-" gorm:"type:text"`
	ContentHash        string    `json:"contentHash" gorm:"size:64;index"`
	CharacterCount     int64     `json:"characterCount"`
	ExtractionStatus   string    `json:"extractionStatus" gorm:"size:32;index"`
	MetadataJSON       string    `json:"metadataJson,omitempty" gorm:"type:text"`
	CreatedBy          string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

func (ManjuScriptSource) TableName() string { return "manju_script_sources" }

type ManjuScriptArtifact struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID   string    `json:"projectId" gorm:"size:36;index"`
	RunID       string    `json:"runId" gorm:"size:36;index"`
	StageID     string    `json:"stageId" gorm:"size:36;index"`
	Kind        string    `json:"kind" gorm:"size:80;index"`
	Version     int64     `json:"version"`
	Content     string    `json:"content" gorm:"type:text"`
	ContentHash string    `json:"contentHash" gorm:"size:64"`
	TaskID      string    `json:"taskId,omitempty" gorm:"size:36;index"`
	CreatedBy   string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (ManjuScriptArtifact) TableName() string { return "manju_script_artifacts" }

// ManjuRoundtableDecision is the append-only user decision ledger for each
// roundtable review round. Review artifacts stay immutable; the decision record
// captures the selected action, risk acknowledgement and exact review identity.
type ManjuRoundtableDecision struct {
	ID                 string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID          string    `json:"projectId" gorm:"size:36;index"`
	RunID              string    `json:"runId" gorm:"size:36;index;uniqueIndex:idx_manju_roundtable_decision_round,priority:1"`
	StageID            string    `json:"stageId" gorm:"size:36;index"`
	Round              int64     `json:"round" gorm:"uniqueIndex:idx_manju_roundtable_decision_round,priority:2"`
	Version            int64     `json:"version"`
	Decision           string    `json:"decision" gorm:"size:32;index"`
	ActorID            string    `json:"actorId" gorm:"size:36;index"`
	Note               string    `json:"note,omitempty" gorm:"type:text"`
	ReviewArtifactID   string    `json:"reviewArtifactId" gorm:"size:36;index"`
	ReviewArtifactHash string    `json:"reviewArtifactHash" gorm:"size:64;index"`
	Grade              string    `json:"grade" gorm:"size:16;index"`
	RiskAccepted       bool      `json:"riskAccepted"`
	CreatedAt          time.Time `json:"createdAt" gorm:"index"`
}

func (ManjuRoundtableDecision) TableName() string { return "manju_roundtable_decisions" }

// ManjuAssetRun is the durable P2 envelope. It is tied to one completed P1
// run and the same immutable P0 lock revision that P1 used.
type ManjuAssetRun struct {
	ID                  string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID           string    `json:"projectId" gorm:"size:36;index"`
	ScriptRunID         string    `json:"scriptRunId" gorm:"size:36;index"`
	BaseAssetRunID      string    `json:"baseAssetRunId,omitempty" gorm:"size:36;index"`
	ReuseAssets         bool      `json:"reuseAssets"`
	InheritedAssetCount int       `json:"inheritedAssetCount"`
	AssetMode           string    `json:"assetMode" gorm:"size:8;index"`
	Status              string    `json:"status" gorm:"size:24;index"`
	CurrentStage        string    `json:"currentStage" gorm:"size:48;index"`
	LockRevision        int64     `json:"lockRevision"`
	CreatedBy           string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt           time.Time `json:"createdAt" gorm:"index"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

func (ManjuAssetRun) TableName() string { return "manju_asset_runs" }

type ManjuAssetStage struct {
	ID          string     `json:"id" gorm:"primaryKey;size:36"`
	RunID       string     `json:"runId" gorm:"size:36;index;uniqueIndex:idx_manju_asset_stage_key,priority:1"`
	Key         string     `json:"key" gorm:"size:48;uniqueIndex:idx_manju_asset_stage_key,priority:2"`
	Title       string     `json:"title" gorm:"size:120"`
	Position    int        `json:"position" gorm:"index"`
	Status      string     `json:"status" gorm:"size:32;index"`
	Version     int64      `json:"version"`
	TaskID      string     `json:"taskId,omitempty" gorm:"size:36;index"`
	ArtifactID  string     `json:"artifactId,omitempty" gorm:"size:36;index"`
	InputJSON   string     `json:"-" gorm:"type:text"`
	Error       string     `json:"error,omitempty" gorm:"type:text"`
	ConfirmedBy string     `json:"confirmedBy,omitempty" gorm:"size:36"`
	ConfirmedAt *time.Time `json:"confirmedAt,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func (ManjuAssetStage) TableName() string { return "manju_asset_stages" }

type ManjuAssetArtifact struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID   string    `json:"projectId" gorm:"size:36;index"`
	RunID       string    `json:"runId" gorm:"size:36;index"`
	StageID     string    `json:"stageId" gorm:"size:36;index"`
	Kind        string    `json:"kind" gorm:"size:96;index"`
	Version     int64     `json:"version"`
	Content     string    `json:"content" gorm:"type:text"`
	ContentHash string    `json:"contentHash" gorm:"size:64"`
	TaskID      string    `json:"taskId,omitempty" gorm:"size:36;index"`
	CreatedBy   string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (ManjuAssetArtifact) TableName() string { return "manju_asset_artifacts" }

// ManjuAssetReference records an image the user explicitly returned for an
// existing asset code. M2 image-level verification is impossible without it.
type ManjuAssetReference struct {
	ID         string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID  string    `json:"projectId" gorm:"size:36;index"`
	RunID      string    `json:"runId" gorm:"size:36;index;uniqueIndex:idx_manju_asset_reference_code,priority:1"`
	Code       string    `json:"code" gorm:"size:80;uniqueIndex:idx_manju_asset_reference_code,priority:2"`
	AssetType  string    `json:"assetType" gorm:"size:16;index"`
	ResourceID string    `json:"resourceId" gorm:"size:120;index"`
	URL        string    `json:"url,omitempty" gorm:"type:text"`
	Status     string    `json:"status" gorm:"size:24;index"`
	Notes      string    `json:"notes,omitempty" gorm:"type:text"`
	CreatedBy  string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (ManjuAssetReference) TableName() string { return "manju_asset_references" }

// ManjuProductionRun is the durable P3-P5 envelope. It consumes one completed
// P2 run and remains tied to the same immutable P0 lock revision.
type ManjuProductionRun struct {
	ID           string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID    string    `json:"projectId" gorm:"size:36;index"`
	AssetRunID   string    `json:"assetRunId" gorm:"size:36;index"`
	Status       string    `json:"status" gorm:"size:24;index"`
	CurrentStage string    `json:"currentStage" gorm:"size:48;index"`
	LockRevision int64     `json:"lockRevision"`
	CreatedBy    string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt    time.Time `json:"createdAt" gorm:"index"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (ManjuProductionRun) TableName() string { return "manju_production_runs" }

type ManjuProductionStage struct {
	ID          string     `json:"id" gorm:"primaryKey;size:36"`
	RunID       string     `json:"runId" gorm:"size:36;index;uniqueIndex:idx_manju_production_stage_key,priority:1"`
	Key         string     `json:"key" gorm:"size:48;uniqueIndex:idx_manju_production_stage_key,priority:2"`
	Title       string     `json:"title" gorm:"size:120"`
	Position    int        `json:"position" gorm:"index"`
	Status      string     `json:"status" gorm:"size:32;index"`
	Version     int64      `json:"version"`
	TaskID      string     `json:"taskId,omitempty" gorm:"size:36;index"`
	ArtifactID  string     `json:"artifactId,omitempty" gorm:"size:36;index"`
	InputJSON   string     `json:"-" gorm:"type:text"`
	Error       string     `json:"error,omitempty" gorm:"type:text"`
	ConfirmedBy string     `json:"confirmedBy,omitempty" gorm:"size:36"`
	ConfirmedAt *time.Time `json:"confirmedAt,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func (ManjuProductionStage) TableName() string { return "manju_production_stages" }

type ManjuProductionArtifact struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID   string    `json:"projectId" gorm:"size:36;index"`
	RunID       string    `json:"runId" gorm:"size:36;index"`
	StageID     string    `json:"stageId" gorm:"size:36;index"`
	Kind        string    `json:"kind" gorm:"size:96;index"`
	Version     int64     `json:"version"`
	Content     string    `json:"content" gorm:"type:text"`
	ContentHash string    `json:"contentHash" gorm:"size:64"`
	TaskID      string    `json:"taskId,omitempty" gorm:"size:36;index"`
	CreatedBy   string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (ManjuProductionArtifact) TableName() string { return "manju_production_artifacts" }

// ManjuDeliverable is the immutable P5 delivery lock. A production run owns at
// most one row per kind; regenerated runs receive new rows while history stays.
type ManjuDeliverable struct {
	ID              string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID       string    `json:"projectId" gorm:"size:36;index"`
	LockRevision    int64     `json:"lockRevision" gorm:"index"`
	ProductionRunID string    `json:"productionRunId" gorm:"size:36;uniqueIndex:idx_manju_deliverable_run_kind,priority:1"`
	Kind            string    `json:"kind" gorm:"size:32;uniqueIndex:idx_manju_deliverable_run_kind,priority:2"`
	FileName        string    `json:"fileName" gorm:"size:255"`
	MimeType        string    `json:"mimeType" gorm:"size:128"`
	Content         string    `json:"content" gorm:"type:text"`
	ContentHash     string    `json:"contentHash" gorm:"size:64"`
	CreatedAt       time.Time `json:"createdAt"`
}

func (ManjuDeliverable) TableName() string { return "manju_deliverables" }

// ManjuStageDraft preserves failed model or manual content so a user can
// correct it without losing the original output.
type ManjuStageDraft struct {
	ID             string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID      string    `json:"projectId" gorm:"size:36;index"`
	Phase          string    `json:"phase" gorm:"size:8;index"`
	RunID          string    `json:"runId" gorm:"size:36;index;uniqueIndex:idx_manju_stage_draft_version,priority:1"`
	StageID        string    `json:"stageId" gorm:"size:36;index;uniqueIndex:idx_manju_stage_draft_version,priority:2"`
	Version        int       `json:"version" gorm:"uniqueIndex:idx_manju_stage_draft_version,priority:3"`
	Content        string    `json:"content" gorm:"type:text"`
	ContentHash    string    `json:"contentHash" gorm:"size:64"`
	ValidationJSON string    `json:"validationJson" gorm:"type:text"`
	Source         string    `json:"source" gorm:"size:32"`
	CreatedBy      string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt      time.Time `json:"createdAt" gorm:"index"`
}

func (ManjuStageDraft) TableName() string { return "manju_stage_drafts" }

// ManjuArtifactCorrection is an immutable audit row for a successful manual
// correction that produced a new official artifact.
type ManjuArtifactCorrection struct {
	ID               string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID        string    `json:"projectId" gorm:"size:36;index"`
	Phase            string    `json:"phase" gorm:"size:8;index"`
	RunID            string    `json:"runId" gorm:"size:36;index"`
	StageID          string    `json:"stageId" gorm:"size:36;index"`
	SourceArtifactID string    `json:"sourceArtifactId" gorm:"size:36;index"`
	ResultArtifactID string    `json:"resultArtifactId" gorm:"size:36;index"`
	DraftID          string    `json:"draftId" gorm:"size:36;index"`
	ContentHash      string    `json:"contentHash" gorm:"size:64"`
	ValidationHash   string    `json:"validationHash" gorm:"size:64"`
	Note             string    `json:"note" gorm:"type:text"`
	CorrectedBy      string    `json:"correctedBy" gorm:"size:36;index"`
	CreatedAt        time.Time `json:"createdAt" gorm:"index"`
}

func (ManjuArtifactCorrection) TableName() string { return "manju_artifact_corrections" }

type ManjuProductionPrevis struct {
	ID         string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID  string    `json:"projectId" gorm:"size:36;index"`
	RunID      string    `json:"runId" gorm:"size:36;uniqueIndex"`
	StageID    string    `json:"stageId" gorm:"size:36;index"`
	TaskID     string    `json:"taskId,omitempty" gorm:"size:36;index"`
	ArtifactID string    `json:"artifactId,omitempty" gorm:"size:36;index"`
	Status     string    `json:"status" gorm:"size:24;index"`
	Version    int64     `json:"version"`
	Error      string    `json:"error,omitempty" gorm:"type:text"`
	CreatedBy  string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (ManjuProductionPrevis) TableName() string { return "manju_production_previs" }

// ManjuProductionImage is one generated storyboard image for a P3 segment.
// The resource ID is written only after a durable generation output and its
// owning Resource row have been verified.
type ManjuProductionImage struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID   string    `json:"projectId" gorm:"size:36;index"`
	RunID       string    `json:"runId" gorm:"size:36;index;uniqueIndex:idx_manju_production_image_position,priority:1"`
	StageID     string    `json:"stageId" gorm:"size:36;index"`
	ArtifactID  string    `json:"artifactId,omitempty" gorm:"size:36;index"`
	Position    int       `json:"position" gorm:"uniqueIndex:idx_manju_production_image_position,priority:2"`
	SegmentCode string    `json:"segmentCode" gorm:"size:48;index"`
	ShotLabel   string    `json:"shotLabel" gorm:"size:80"`
	Prompt      string    `json:"prompt" gorm:"type:text"`
	Attempt     int       `json:"attempt"`
	Status      string    `json:"status" gorm:"size:24;index"`
	TaskID      string    `json:"taskId,omitempty" gorm:"size:36;index"`
	ResourceID  string    `json:"resourceId,omitempty" gorm:"size:120;index"`
	URL         string    `json:"url,omitempty" gorm:"type:text"`
	Error       string    `json:"error,omitempty" gorm:"type:text"`
	CreatedBy   string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (ManjuProductionImage) TableName() string { return "manju_production_images" }

// ManjuProductionVideo is one generated P4 segment video. READY is written
// only after the generation output and its owning video Resource reconcile.
type ManjuProductionVideo struct {
	ID              string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID       string    `json:"projectId" gorm:"size:36;index"`
	RunID           string    `json:"runId" gorm:"size:36;index;uniqueIndex:idx_manju_production_video_position,priority:1"`
	StageID         string    `json:"stageId" gorm:"size:36;index"`
	Position        int       `json:"position" gorm:"uniqueIndex:idx_manju_production_video_position,priority:2"`
	SegmentCode     string    `json:"segmentCode" gorm:"size:48;index"`
	DurationSeconds int       `json:"durationSeconds"`
	Prompt          string    `json:"prompt" gorm:"type:text"`
	Attempt         int       `json:"attempt"`
	Status          string    `json:"status" gorm:"size:24;index"`
	TaskID          string    `json:"taskId,omitempty" gorm:"size:36;index"`
	ResourceID      string    `json:"resourceId,omitempty" gorm:"size:120;index"`
	URL             string    `json:"url,omitempty" gorm:"type:text"`
	MimeType        string    `json:"mimeType,omitempty" gorm:"size:120"`
	Error           string    `json:"error,omitempty" gorm:"type:text"`
	CreatedBy       string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (ManjuProductionVideo) TableName() string { return "manju_production_videos" }

// ManjuCanvasImport links one immutable Manju production package to the
// concrete canvas where the user executes image or video generation.
// Readiness is verified against the owned CanvasProject payload, never from
// a client-submitted completion flag.
type ManjuCanvasImport struct {
	ID              string    `json:"id" gorm:"primaryKey;size:36"`
	ProjectID       string    `json:"projectId" gorm:"size:36;index;uniqueIndex:idx_manju_canvas_import_stage,priority:1"`
	Stage           string    `json:"stage" gorm:"size:32;index;uniqueIndex:idx_manju_canvas_import_stage,priority:2"`
	AssetRunID      string    `json:"assetRunId,omitempty" gorm:"size:36;index"`
	ProductionRunID string    `json:"productionRunId,omitempty" gorm:"size:36;index"`
	CanvasID        string    `json:"canvasId" gorm:"size:80;index"`
	PackageHash     string    `json:"packageHash" gorm:"size:64;index"`
	TaskCount       int       `json:"taskCount"`
	Status          string    `json:"status" gorm:"size:24;index"`
	CreatedBy       string    `json:"createdBy" gorm:"size:36;index"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (ManjuCanvasImport) TableName() string { return "manju_canvas_imports" }
