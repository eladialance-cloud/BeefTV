package hostedadmin

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/auth"
	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

const siteConfigKey = "site_config"

type AccountManager interface {
	CreateManagedUser(input auth.RegisterInput) (*model.User, error)
	ResetPassword(userID string, password string) error
}

type CreditGranter interface {
	Grant(userID string, amount int64, operatorID string, reason string, idempotencyKey string) (*credits.Result, error)
}

type Service struct {
	repo     *repository.Repository
	accounts AccountManager
	credits  CreditGranter
	now      func() time.Time
}

func New(repo *repository.Repository, accounts AccountManager, creditService CreditGranter) *Service {
	return &Service{
		repo: repo, accounts: accounts, credits: creditService,
		now: func() time.Time { return time.Now().UTC() },
	}
}

type CreateUserInput struct {
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	Password       string `json:"password"`
	InitialCredits int64  `json:"initialCredits"`
}

type UpdateUserInput struct {
	Role   model.UserRole   `json:"role"`
	Status model.UserStatus `json:"status"`
}

type SiteConfig struct {
	SiteName        string `json:"siteName"`
	LogoURL         string `json:"logoUrl"`
	FilingNo        string `json:"filingNo"`
	SupportURL      string `json:"supportUrl"`
	CreditName      string `json:"creditName"`
	Currency        string `json:"currency"`
	FooterText      string `json:"footerText"`
	Announcement    string `json:"announcement"`
	MaintenanceMode bool   `json:"maintenanceMode"`
}

func defaultSiteConfig() SiteConfig {
	return SiteConfig{
		SiteName: "VisiDeep AI", CreditName: "积分", Currency: "CNY",
		FooterText: "VisiDeep AI", Announcement: "",
	}
}

func (s *Service) requireAdmin(actor *model.User) error {
	if actor == nil {
		return kernel.Unauthorized("请先登录")
	}
	if actor.Role != model.UserRoleAdmin || actor.Status != model.UserStatusActive {
		return kernel.Forbidden("需要管理员权限")
	}
	return nil
}

func (s *Service) audit(actor *model.User, action string, targetType string, targetID string, summary string, metadata any) error {
	encoded := ""
	if metadata != nil {
		data, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		encoded = string(data)
	}
	return s.repo.AppendAdminAudit(&model.AdminAuditEvent{
		ID: kernel.NewID(), ActorUserID: actor.ID, Action: action,
		TargetType: targetType, TargetID: targetID, Summary: summary,
		MetadataJSON: encoded, CreatedAt: s.now(),
	})
}

func (s *Service) ListUsers(filter repository.AdminUserFilter) ([]repository.AdminUserRow, int64, error) {
	return s.repo.AdminUsersPage(filter)
}

func (s *Service) CreateUser(actor *model.User, input CreateUserInput) (*repository.AdminUserRow, error) {
	if err := s.requireAdmin(actor); err != nil {
		return nil, err
	}
	if input.InitialCredits < 0 {
		return nil, kernel.BadAuthRequest("初始积分不能为负数")
	}
	user, err := s.accounts.CreateManagedUser(auth.RegisterInput{
		Username: input.Username, DisplayName: input.DisplayName, Password: input.Password,
	})
	if err != nil {
		return nil, err
	}
	if input.InitialCredits > 0 {
		if _, err := s.credits.Grant(user.ID, credits.FromWholeCredits(input.InitialCredits), actor.ID, "管理员创建账号赠送", "admin-user-create:"+user.ID); err != nil {
			_ = s.audit(actor, "user.create", "user", user.ID, "创建用户成功，但初始积分发放失败", map[string]any{"error": err.Error()})
			return nil, err
		}
	}
	if err := s.audit(actor, "user.create", "user", user.ID, "创建用户 "+user.Username, map[string]any{
		"username": user.Username, "initialCredits": input.InitialCredits,
	}); err != nil {
		return nil, err
	}
	row, found, err := s.repo.AdminUserByID(user.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("新用户回读失败")
	}
	return row, nil
}

func (s *Service) UpdateUser(actor *model.User, targetID string, input UpdateUserInput) (*repository.AdminUserRow, error) {
	if err := s.requireAdmin(actor); err != nil {
		return nil, err
	}
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return nil, kernel.BadAuthRequest("缺少目标用户")
	}
	if input.Role != model.UserRoleAdmin && input.Role != model.UserRoleUser {
		return nil, kernel.BadAuthRequest("用户角色不合法")
	}
	if input.Status != model.UserStatusActive && input.Status != model.UserStatusDisabled {
		return nil, kernel.BadAuthRequest("用户状态不合法")
	}
	target, found, err := s.repo.AdminUserByID(targetID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, kernel.NotFound("用户不存在")
	}
	if targetID == actor.ID && (input.Role != model.UserRoleAdmin || input.Status != model.UserStatusActive) {
		return nil, kernel.BadAuthRequest("不能降级或停用当前管理员账号")
	}
	if target.Role == model.UserRoleAdmin && target.Status == model.UserStatusActive &&
		(input.Role != model.UserRoleAdmin || input.Status != model.UserStatusActive) {
		count, err := s.repo.AdminActiveAdminCount()
		if err != nil {
			return nil, err
		}
		if count <= 1 {
			return nil, kernel.BadAuthRequest("系统必须保留至少一个可用管理员")
		}
	}
	updated, err := s.repo.AdminUpdateUser(targetID, input.Role, input.Status, s.now())
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, kernel.NotFound("用户不存在")
	}
	if err := s.audit(actor, "user.update", "user", targetID, "更新用户角色或状态", map[string]any{
		"beforeRole": target.Role, "beforeStatus": target.Status,
		"afterRole": input.Role, "afterStatus": input.Status,
	}); err != nil {
		return nil, err
	}
	row, found, err := s.repo.AdminUserByID(targetID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, kernel.NotFound("用户不存在")
	}
	return row, nil
}

func (s *Service) ResetUserPassword(actor *model.User, targetID string, password string) error {
	if err := s.requireAdmin(actor); err != nil {
		return err
	}
	if err := s.accounts.ResetPassword(targetID, password); err != nil {
		return err
	}
	return s.audit(actor, "user.reset_password", "user", targetID, "重置用户密码", nil)
}

func (s *Service) ListProjects(filter repository.AdminProjectFilter) ([]repository.AdminProjectRow, int64, error) {
	return s.repo.AdminProjectsPage(filter)
}

func (s *Service) SetProjectArchived(actor *model.User, kind string, id string, archived bool, reason string) error {
	if err := s.requireAdmin(actor); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return kernel.BadAuthRequest("缺少项目 ID")
	}
	reason = strings.TrimSpace(reason)
	if archived && reason == "" {
		return kernel.BadAuthRequest("归档项目必须填写原因")
	}
	switch strings.TrimSpace(kind) {
	case "", "domain":
		status := model.ProjectStatusActive
		action := "project.restore"
		if archived {
			status = model.ProjectStatusArchived
			action = "project.archive"
		}
		ok, err := s.repo.AdminSetProjectStatus(id, status, s.now())
		if err != nil {
			return err
		}
		if !ok {
			return kernel.NotFound("项目不存在")
		}
		return s.audit(actor, action, "project", id, "变更项目状态", map[string]any{"reason": reason, "status": status})
	case "canvas":
		exists, err := s.repo.AdminCanvasExists(id)
		if err != nil {
			return err
		}
		if !exists {
			return kernel.NotFound("画布不存在")
		}
		status := model.AdminModerationStatusActive
		action := "canvas.restore"
		if archived {
			status = model.AdminModerationStatusBlocked
			action = "canvas.block"
		}
		if err := s.repo.AdminSetModeration(model.AdminModerationTargetCanvas, id, status, reason, actor.ID, s.now()); err != nil {
			return err
		}
		return s.audit(actor, action, "canvas", id, "变更画布可见性", map[string]any{"reason": reason, "status": status})
	default:
		return kernel.BadAuthRequest("项目类型不合法")
	}
}

func (s *Service) ListAssets(filter repository.AdminAssetFilter) ([]repository.AdminAssetRow, int64, error) {
	return s.repo.AdminAssetsPage(filter)
}

func (s *Service) ModerateAsset(actor *model.User, id string, blocked bool, reason string) error {
	if err := s.requireAdmin(actor); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return kernel.BadAuthRequest("缺少素材 ID")
	}
	reason = strings.TrimSpace(reason)
	if blocked && reason == "" {
		return kernel.BadAuthRequest("下架素材必须填写原因")
	}
	exists, err := s.repo.AdminAssetExists(id)
	if err != nil {
		return err
	}
	if !exists {
		return kernel.NotFound("素材不存在")
	}
	status := model.AdminModerationStatusActive
	action := "asset.restore"
	if blocked {
		status = model.AdminModerationStatusBlocked
		action = "asset.block"
	}
	if err := s.repo.AdminSetModeration(model.AdminModerationTargetAsset, id, status, reason, actor.ID, s.now()); err != nil {
		return err
	}
	return s.audit(actor, action, "asset", id, "变更素材可见性", map[string]any{"reason": reason, "status": status})
}

func (s *Service) ListLedger(filter repository.AdminLedgerFilter) ([]repository.AdminLedgerRow, int64, error) {
	return s.repo.AdminLedgerPage(filter)
}

func (s *Service) ListAudit(filter repository.AdminAuditFilter) ([]repository.AdminAuditRow, int64, error) {
	return s.repo.AdminAuditPage(filter)
}

func (s *Service) Overview(days int) (repository.AdminOverviewMetrics, error) {
	if days <= 0 || days > 90 {
		days = 7
	}
	now := s.now()
	return s.repo.AdminOverview(now.Add(-time.Duration(days)*24*time.Hour), now)
}

func (s *Service) SiteConfig() (SiteConfig, error) {
	cfg := defaultSiteConfig()
	setting, err := s.repo.SystemSetting(siteConfigKey)
	if err != nil {
		return cfg, nil
	}
	if strings.TrimSpace(setting.ValueJSON) != "" {
		_ = json.Unmarshal([]byte(setting.ValueJSON), &cfg)
	}
	if strings.TrimSpace(cfg.SiteName) == "" {
		cfg.SiteName = defaultSiteConfig().SiteName
	}
	if strings.TrimSpace(cfg.CreditName) == "" {
		cfg.CreditName = defaultSiteConfig().CreditName
	}
	if strings.TrimSpace(cfg.Currency) == "" {
		cfg.Currency = defaultSiteConfig().Currency
	}
	return cfg, nil
}

func (s *Service) SaveSiteConfig(actor *model.User, cfg SiteConfig) (SiteConfig, error) {
	if err := s.requireAdmin(actor); err != nil {
		return SiteConfig{}, err
	}
	cfg.SiteName = strings.TrimSpace(cfg.SiteName)
	cfg.LogoURL = strings.TrimSpace(cfg.LogoURL)
	cfg.FilingNo = strings.TrimSpace(cfg.FilingNo)
	cfg.SupportURL = strings.TrimSpace(cfg.SupportURL)
	cfg.CreditName = strings.TrimSpace(cfg.CreditName)
	cfg.Currency = strings.TrimSpace(cfg.Currency)
	cfg.FooterText = strings.TrimSpace(cfg.FooterText)
	cfg.Announcement = strings.TrimSpace(cfg.Announcement)
	if cfg.SiteName == "" {
		return SiteConfig{}, kernel.BadAuthRequest("站点名称不能为空")
	}
	if cfg.CreditName == "" {
		cfg.CreditName = defaultSiteConfig().CreditName
	}
	if cfg.Currency == "" {
		cfg.Currency = defaultSiteConfig().Currency
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return SiteConfig{}, err
	}
	if err := s.repo.SaveSystemSetting(&model.SystemSetting{
		Key: siteConfigKey, ValueJSON: string(encoded), UpdatedBy: actor.ID,
	}); err != nil {
		return SiteConfig{}, err
	}
	if err := s.audit(actor, "site.update", "site", siteConfigKey, "更新站点配置", cfg); err != nil {
		return SiteConfig{}, err
	}
	return cfg, nil
}
