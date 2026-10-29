package repository

import (
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
)

type AdminUserFilter struct {
	Keyword  string
	Role     string
	Status   string
	Page     int
	PageSize int
}

type AdminUserRow struct {
	ID          string           `json:"id"`
	Username    string           `json:"username"`
	DisplayName string           `json:"displayName"`
	Role        model.UserRole   `json:"role"`
	Status      model.UserStatus `json:"status"`
	Balance     int64            `json:"balance"`
	Frozen      int64            `json:"frozen"`
	Available   int64            `json:"available"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
}

func normalizeAdminLimit(page int, pageSize int) (int, int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 30
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize, (page - 1) * pageSize
}

func (r *Repository) AdminUsersPage(filter AdminUserFilter) ([]AdminUserRow, int64, error) {
	_, pageSize, offset := normalizeAdminLimit(filter.Page, filter.PageSize)
	query := r.db.Table("users AS u")
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("LOWER(u.username) LIKE ? OR LOWER(u.display_name) LIKE ? OR LOWER(u.id) LIKE ?", like, like, like)
	}
	if role := strings.TrimSpace(filter.Role); role != "" {
		query = query.Where("u.role = ?", role)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("u.status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []AdminUserRow
	err := query.Select(`
		u.id, u.username, u.display_name, u.role, u.status, u.created_at, u.updated_at,
		COALESCE(ca.balance, 0) AS balance,
		COALESCE(ca.frozen, 0) AS frozen,
		COALESCE(ca.balance, 0) - COALESCE(ca.frozen, 0) AS available`).
		Joins("LEFT JOIN credit_accounts ca ON ca.user_id = u.id").
		Order("u.created_at DESC, u.id DESC").
		Offset(offset).Limit(pageSize).Scan(&rows).Error
	return rows, total, err
}

func (r *Repository) AdminUserByID(id string) (*AdminUserRow, bool, error) {
	var row AdminUserRow
	result := r.db.Table("users AS u").
		Select(`
			u.id, u.username, u.display_name, u.role, u.status, u.created_at, u.updated_at,
			COALESCE(ca.balance, 0) AS balance,
			COALESCE(ca.frozen, 0) AS frozen,
			COALESCE(ca.balance, 0) - COALESCE(ca.frozen, 0) AS available`).
		Joins("LEFT JOIN credit_accounts ca ON ca.user_id = u.id").
		Where("u.id = ?", id).Limit(1).Scan(&row)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return &row, true, nil
}

func (r *Repository) AdminUpdateUser(id string, role model.UserRole, status model.UserStatus, updatedAt time.Time) (bool, error) {
	result := r.db.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"role": role, "status": status, "updated_at": updatedAt,
	})
	return result.RowsAffected > 0, result.Error
}

func (r *Repository) AdminActiveAdminCount() (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).
		Where("role = ? AND status = ?", model.UserRoleAdmin, model.UserStatusActive).
		Count(&count).Error
	return count, err
}

type AdminProjectFilter struct {
	Kind     string
	Keyword  string
	Status   string
	Page     int
	PageSize int
}

type AdminProjectRow struct {
	ID               string    `json:"id"`
	Kind             string    `json:"kind"`
	Name             string    `json:"name"`
	UserID           string    `json:"userId"`
	Username         string    `json:"username"`
	DisplayName      string    `json:"displayName"`
	Status           string    `json:"status"`
	ModerationStatus string    `json:"moderationStatus"`
	ModerationReason string    `json:"moderationReason"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (r *Repository) AdminProjectsPage(filter AdminProjectFilter) ([]AdminProjectRow, int64, error) {
	_, pageSize, offset := normalizeAdminLimit(filter.Page, filter.PageSize)
	kind := strings.TrimSpace(filter.Kind)
	if kind == "" {
		kind = "domain"
	}
	if kind == "canvas" {
		return r.adminCanvasProjectsPage(filter, pageSize, offset)
	}
	query := r.db.Table("projects AS p").Joins("LEFT JOIN users u ON u.id = p.user_id")
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("LOWER(p.name) LIKE ? OR LOWER(p.user_id) LIKE ? OR LOWER(u.username) LIKE ?", like, like, like)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("p.status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []AdminProjectRow
	err := query.Select(`
		p.id, 'domain' AS kind, p.name, p.user_id, COALESCE(u.username,'') AS username,
		COALESCE(u.display_name,'') AS display_name, p.status, p.status AS moderation_status,
		'' AS moderation_reason, p.created_at, p.updated_at`).
		Order("p.updated_at DESC, p.id DESC").Offset(offset).Limit(pageSize).Scan(&rows).Error
	return rows, total, err
}

func (r *Repository) adminCanvasProjectsPage(filter AdminProjectFilter, pageSize int, offset int) ([]AdminProjectRow, int64, error) {
	query := r.db.Table("canvas_projects AS p").Joins("LEFT JOIN users u ON u.id = p.user_id")
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("LOWER(p.title) LIKE ? OR LOWER(p.user_id) LIKE ? OR LOWER(u.username) LIKE ?", like, like, like)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		if status == model.AdminModerationStatusBlocked {
			query = query.Where("EXISTS (SELECT 1 FROM admin_content_moderations m WHERE m.target_type = ? AND m.target_id = p.id AND m.status = ?)", model.AdminModerationTargetCanvas, model.AdminModerationStatusBlocked)
		} else if status == "active" {
			query = query.Where("NOT EXISTS (SELECT 1 FROM admin_content_moderations m WHERE m.target_type = ? AND m.target_id = p.id AND m.status = ?)", model.AdminModerationTargetCanvas, model.AdminModerationStatusBlocked)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []AdminProjectRow
	err := query.Select(`
		p.id, 'canvas' AS kind, p.title AS name, p.user_id, COALESCE(u.username,'') AS username,
		COALESCE(u.display_name,'') AS display_name,
		CASE WHEN COALESCE(m.status,'active') = 'blocked' THEN 'blocked' ELSE 'active' END AS status,
		COALESCE(m.status,'active') AS moderation_status, COALESCE(m.reason,'') AS moderation_reason,
		p.created_at, p.updated_at`).
		Joins("LEFT JOIN admin_content_moderations m ON m.target_type = ? AND m.target_id = p.id", model.AdminModerationTargetCanvas).
		Order("p.updated_at DESC, p.id DESC").Offset(offset).Limit(pageSize).Scan(&rows).Error
	return rows, total, err
}

func (r *Repository) AdminSetProjectStatus(id string, status model.ProjectStatus, updatedAt time.Time) (bool, error) {
	result := r.db.Model(&model.Project{}).Where("id = ?", id).Updates(map[string]any{
		"status": status, "updated_at": updatedAt,
	})
	return result.RowsAffected > 0, result.Error
}

type AdminAssetFilter struct {
	Keyword  string
	Kind     string
	Status   string
	Page     int
	PageSize int
}

type AdminAssetRow struct {
	ID               string    `json:"id"`
	UserID           string    `json:"userId"`
	Username         string    `json:"username"`
	DisplayName      string    `json:"displayName"`
	Title            string    `json:"title"`
	Kind             string    `json:"kind"`
	Category         string    `json:"category"`
	Status           string    `json:"status"`
	ModerationStatus string    `json:"moderationStatus"`
	ModerationReason string    `json:"moderationReason"`
	SizeBytes        int64     `json:"sizeBytes"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (r *Repository) AdminAssetsPage(filter AdminAssetFilter) ([]AdminAssetRow, int64, error) {
	_, pageSize, offset := normalizeAdminLimit(filter.Page, filter.PageSize)
	query := r.db.Table("assets AS a").Joins("LEFT JOIN users u ON u.id = a.user_id")
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("LOWER(a.title) LIKE ? OR LOWER(a.user_id) LIKE ? OR LOWER(u.username) LIKE ?", like, like, like)
	}
	if kind := strings.TrimSpace(filter.Kind); kind != "" {
		query = query.Where("a.kind = ?", kind)
	}
	if status := strings.TrimSpace(filter.Status); status == model.AdminModerationStatusBlocked || status == "active" {
		if status == model.AdminModerationStatusBlocked {
			query = query.Where("EXISTS (SELECT 1 FROM admin_content_moderations m WHERE m.target_type = ? AND m.target_id = a.id AND m.status = ?)", model.AdminModerationTargetAsset, model.AdminModerationStatusBlocked)
		} else {
			query = query.Where("NOT EXISTS (SELECT 1 FROM admin_content_moderations m WHERE m.target_type = ? AND m.target_id = a.id AND m.status = ?)", model.AdminModerationTargetAsset, model.AdminModerationStatusBlocked)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []AdminAssetRow
	err := query.Select(`
		a.id, a.user_id, COALESCE(u.username,'') AS username, COALESCE(u.display_name,'') AS display_name,
		a.title, a.kind, a.category, a.status,
		COALESCE(m.status,'active') AS moderation_status, COALESCE(m.reason,'') AS moderation_reason,
		COALESCE((
			SELECT SUM(r.size) FROM resources r
			WHERE r.id IN (
				SELECT DISTINCT ar.resource_id FROM asset_versions av
				JOIN asset_representations ar ON ar.asset_version_id = av.id
				WHERE av.asset_id = a.id AND ar.resource_id <> ''
			)
		), 0) AS size_bytes,
		a.created_at, a.updated_at`).
		Joins("LEFT JOIN admin_content_moderations m ON m.target_type = ? AND m.target_id = a.id", model.AdminModerationTargetAsset).
		Order("a.updated_at DESC, a.id DESC").Offset(offset).Limit(pageSize).Scan(&rows).Error
	return rows, total, err
}

type AdminLedgerFilter struct {
	Keyword  string
	Kind     string
	Model    string
	Token    string
	Start    *time.Time
	End      *time.Time
	Page     int
	PageSize int
}

type AdminLedgerRow struct {
	ID                string    `json:"id"`
	UserID            string    `json:"userId"`
	Username          string    `json:"username"`
	DisplayName       string    `json:"displayName"`
	Kind              string    `json:"kind"`
	Stage             string    `json:"stage"`
	TaskID            string    `json:"taskId"`
	Amount            int64     `json:"amount"`
	BalanceBefore     int64     `json:"balanceBefore"`
	BalanceAfter      int64     `json:"balanceAfter"`
	OperatorID        string    `json:"operatorId"`
	Reason            string    `json:"reason"`
	Model             string    `json:"model"`
	TokenName         string    `json:"tokenName"`
	ChannelName       string    `json:"channelName"`
	ProjectName       string    `json:"projectName"`
	TaskType          string    `json:"taskType"`
	TaskStatus        string    `json:"taskStatus"`
	TaskOperation     string    `json:"taskOperation"`
	ProviderRequestID string    `json:"providerRequestId"`
	CostStatus        string    `json:"costStatus"`
	ActualCredits     int64     `json:"actualCredits"`
	Capability        string    `json:"-" gorm:"column:capability"`
	CreatedAt         time.Time `json:"createdAt"`
}

func (r *Repository) AdminLedgerPage(filter AdminLedgerFilter) ([]AdminLedgerRow, int64, error) {
	_, pageSize, offset := normalizeAdminLimit(filter.Page, filter.PageSize)
	query := r.db.Table("credit_ledger_entries AS e").
		Joins("LEFT JOIN users u ON u.id = e.user_id").
		Joins("LEFT JOIN tasks t ON t.id = e.task_id").
		Joins("LEFT JOIN projects p ON p.id = t.project_id").
		Joins("LEFT JOIN channel_models cm ON cm.id = t.channel_model_id").
		Joins("LEFT JOIN model_channels mc ON mc.id = cm.channel_id").
		Joins("LEFT JOIN task_cost_charges cc ON cc.id = (SELECT latest.id FROM task_cost_charges latest WHERE latest.task_id = e.task_id ORDER BY latest.attempt DESC LIMIT 1)")
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where(
			`LOWER(e.user_id) LIKE ? OR LOWER(u.username) LIKE ? OR LOWER(u.display_name) LIKE ?
			 OR LOWER(e.task_id) LIKE ? OR LOWER(COALESCE(p.name,'')) LIKE ?
			 OR LOWER(COALESCE(cm.display_name,'')) LIKE ? OR LOWER(COALESCE(cm.model_key,'')) LIKE ?`,
			like, like, like, like, like, like, like,
		)
	}
	if kind := strings.TrimSpace(filter.Kind); kind != "" {
		query = query.Where("e.kind = ?", kind)
	}
	if modelFilter := strings.TrimSpace(filter.Model); modelFilter != "" {
		like := "%" + strings.ToLower(modelFilter) + "%"
		query = query.Where(
			"LOWER(COALESCE(cm.display_name,'')) LIKE ? OR LOWER(COALESCE(cm.model_key,'')) LIKE ? OR LOWER(COALESCE(t.model,'')) LIKE ?",
			like, like, like,
		)
	}
	if token := normalizeAdminLedgerToken(filter.Token); token != "" {
		query = query.Where(adminLedgerTokenExpression()+" = ?", token)
	}
	if filter.Start != nil {
		query = query.Where("e.created_at >= ?", *filter.Start)
	}
	if filter.End != nil {
		query = query.Where("e.created_at < ?", *filter.End)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []AdminLedgerRow
	err := query.Select(`
		e.id, e.user_id, COALESCE(u.username,'') AS username, COALESCE(u.display_name,'') AS display_name,
		e.kind, e.stage, e.task_id, e.amount, e.balance_before, e.balance_after,
		e.operator_id, e.reason, e.created_at,
		COALESCE(NULLIF(cm.display_name,''), NULLIF(cm.model_key,''), NULLIF(t.model,''), '') AS model,
		COALESCE(NULLIF(mc.public_alias,''), NULLIF(mc.name,''), '') AS channel_name,
		COALESCE(NULLIF(p.name,''), '') AS project_name,
		COALESCE(NULLIF(t.type,''), '') AS task_type,
		COALESCE(NULLIF(t.status,''), '') AS task_status,
		COALESCE(NULLIF(t.operation,''), '') AS task_operation,
		COALESCE(NULLIF(t.provider_request_id,''), '') AS provider_request_id,
		COALESCE(NULLIF(cc.status,''), '') AS cost_status,
		COALESCE(cc.actual_credits, 0) AS actual_credits,
		COALESCE(NULLIF(cc.capability,''), NULLIF(cm.capability,''), NULLIF(t.type,''), '') AS capability`).
		Order("e.created_at DESC, e.id DESC").Offset(offset).Limit(pageSize).Scan(&rows).Error
	for index := range rows {
		rows[index].TokenName = adminLedgerTokenName(rows[index].Capability, rows[index].TaskType, rows[index].TaskID)
	}
	return rows, total, err
}

func adminLedgerTokenExpression() string {
	return `CASE
		WHEN COALESCE(e.task_id,'') = '' THEN 'account'
		WHEN LOWER(COALESCE(NULLIF(cc.capability,''), NULLIF(cm.capability,''), t.type, '')) LIKE '%video%' THEN 'video'
		WHEN LOWER(COALESCE(NULLIF(cc.capability,''), NULLIF(cm.capability,''), t.type, '')) LIKE '%image%' THEN 'image'
		WHEN LOWER(COALESCE(NULLIF(cc.capability,''), NULLIF(cm.capability,''), t.type, '')) LIKE '%audio%'
			OR LOWER(COALESCE(NULLIF(cc.capability,''), NULLIF(cm.capability,''), t.type, '')) LIKE '%voice%'
			OR LOWER(COALESCE(NULLIF(cc.capability,''), NULLIF(cm.capability,''), t.type, '')) LIKE '%speech%'
			OR LOWER(COALESCE(NULLIF(cc.capability,''), NULLIF(cm.capability,''), t.type, '')) LIKE '%music%' THEN 'audio'
		ELSE 'text'
	END`
}

func normalizeAdminLedgerToken(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text", "文本":
		return "text"
	case "image", "图片":
		return "image"
	case "video", "视频":
		return "video"
	case "audio", "voice", "speech", "语音":
		return "audio"
	case "account", "账户":
		return "account"
	default:
		return ""
	}
}

func adminLedgerTokenName(capability, taskType, taskID string) string {
	if strings.TrimSpace(taskID) == "" {
		return "账户"
	}
	value := strings.ToLower(strings.TrimSpace(capability + " " + taskType))
	switch {
	case strings.Contains(value, "video"):
		return "视频"
	case strings.Contains(value, "image"):
		return "图片"
	case strings.Contains(value, "audio"), strings.Contains(value, "voice"), strings.Contains(value, "speech"), strings.Contains(value, "music"):
		return "语音"
	default:
		return "文本"
	}
}

type AdminAuditFilter struct {
	Keyword  string
	Action   string
	Start    *time.Time
	End      *time.Time
	Page     int
	PageSize int
}

type AdminAuditRow struct {
	ID          string    `json:"id"`
	ActorUserID string    `json:"actorUserId"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	Action      string    `json:"action"`
	TargetType  string    `json:"targetType"`
	TargetID    string    `json:"targetId"`
	Summary     string    `json:"summary"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (r *Repository) AdminAuditPage(filter AdminAuditFilter) ([]AdminAuditRow, int64, error) {
	_, pageSize, offset := normalizeAdminLimit(filter.Page, filter.PageSize)
	query := r.db.Table("admin_audit_events AS e").Joins("LEFT JOIN users u ON u.id = e.actor_user_id")
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where("LOWER(e.actor_user_id) LIKE ? OR LOWER(u.username) LIKE ? OR LOWER(e.target_id) LIKE ?", like, like, like)
	}
	if action := strings.TrimSpace(filter.Action); action != "" {
		query = query.Where("e.action = ?", action)
	}
	if filter.Start != nil {
		query = query.Where("e.created_at >= ?", *filter.Start)
	}
	if filter.End != nil {
		query = query.Where("e.created_at < ?", *filter.End)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []AdminAuditRow
	err := query.Select(`
		e.id, e.actor_user_id, COALESCE(u.username,'') AS username, COALESCE(u.display_name,'') AS display_name,
		e.action, e.target_type, e.target_id, e.summary, e.created_at`).
		Order("e.created_at DESC, e.id DESC").Offset(offset).Limit(pageSize).Scan(&rows).Error
	return rows, total, err
}

type AdminOverviewMetrics struct {
	NewUsers        int64   `json:"newUsers"`
	ActiveUsers     int64   `json:"activeUsers"`
	Projects        int64   `json:"projects"`
	Canvases        int64   `json:"canvases"`
	Tasks           int64   `json:"tasks"`
	TaskSuccessRate float64 `json:"taskSuccessRate"`
	ConsumedCredits int64   `json:"consumedCredits"`
	RechargeCredits int64   `json:"rechargeCredits"`
	RevenueFen      int64   `json:"revenueFen"`
	PendingOrders   int64   `json:"pendingOrders"`
	Assets          int64   `json:"assets"`
	StorageBytes    int64   `json:"storageBytes"`
}

func (r *Repository) AdminOverview(start time.Time, end time.Time) (AdminOverviewMetrics, error) {
	var out AdminOverviewMetrics
	count := func(model any, where string, args ...any) (int64, error) {
		var total int64
		err := r.db.Model(model).Where(where, args...).Count(&total).Error
		return total, err
	}
	var err error
	if out.NewUsers, err = count(&model.User{}, "created_at >= ? AND created_at < ?", start, end); err != nil {
		return out, err
	}
	if out.ActiveUsers, err = count(&model.UserDailyActivity{}, "last_active_at >= ? AND last_active_at < ?", start, end); err != nil {
		return out, err
	}
	if out.Projects, err = count(&model.Project{}, "created_at >= ? AND created_at < ?", start, end); err != nil {
		return out, err
	}
	if out.Canvases, err = count(&model.CanvasProject{}, "created_at >= ? AND created_at < ?", start, end); err != nil {
		return out, err
	}
	if out.Tasks, err = count(&model.Task{}, "created_at >= ? AND created_at < ?", start, end); err != nil {
		return out, err
	}
	if out.Assets, err = count(&model.Asset{}, "created_at >= ? AND created_at < ?", start, end); err != nil {
		return out, err
	}
	var succeeded int64
	if err = r.db.Model(&model.Task{}).Where("created_at >= ? AND created_at < ? AND status = ?", start, end, model.TaskStatusSucceeded).Count(&succeeded).Error; err != nil {
		return out, err
	}
	if out.Tasks > 0 {
		out.TaskSuccessRate = float64(succeeded) / float64(out.Tasks)
	}
	if err = r.db.Model(&model.CreditLedgerEntry{}).
		Where("created_at >= ? AND created_at < ? AND kind = ?", start, end, "consume").
		Select("COALESCE(SUM(amount), 0)").Scan(&out.ConsumedCredits).Error; err != nil {
		return out, err
	}
	if err = r.db.Model(&model.CreditLedgerEntry{}).
		Where("created_at >= ? AND created_at < ? AND kind = ?", start, end, "recharge").
		Select("COALESCE(SUM(amount), 0)").Scan(&out.RechargeCredits).Error; err != nil {
		return out, err
	}
	if err = r.db.Model(&model.PaymentOrder{}).
		Where("created_at >= ? AND created_at < ? AND status IN ?", start, end, []string{model.PaymentOrderPaid, model.PaymentOrderRefunded}).
		Select("COALESCE(SUM(amount_fen), 0)").Scan(&out.RevenueFen).Error; err != nil {
		return out, err
	}
	if out.PendingOrders, err = count(&model.PaymentOrder{}, "status = ?", model.PaymentOrderPendingPayment); err != nil {
		return out, err
	}
	if err = r.db.Model(&model.Resource{}).Select("COALESCE(SUM(size), 0)").Scan(&out.StorageBytes).Error; err != nil {
		return out, err
	}
	return out, nil
}
