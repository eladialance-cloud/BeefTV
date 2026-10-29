package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"infinite-canvas/backend/internal/generation"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/modelcatalog"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

type AdminChannelModelImportRequest struct {
	Models []string `json:"models"`
}

type AdminChannelModelTestResult struct {
	DurationMs int64 `json:"durationMs"`
}

func (s *Service) EnsureSystemChannelModels() error {
	channels, err := s.repo.SystemChannels(true)
	if err != nil {
		return err
	}
	for index := range channels {
		items, err := s.repo.ChannelModels(channels[index].ID, true)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			if err := s.syncInitialChannelModels(&channels[index], channelModelNames(channels[index])); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) AdminChannelModels(actor *model.User, channelID string) ([]model.ChannelModel, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	if _, err := s.adminSystemChannel(channelID); err != nil {
		return nil, err
	}
	items, err := s.ensureChannelModels(channelID, true)
	if err != nil {
		return nil, err
	}
	return hydrateChannelModelUpstreamPricing(modelcatalog.HydrateChannelModelCapabilityConfig(items)), nil
}

func (s *Service) SystemChannelModel(channelID string, modelKey string) (*model.ChannelModel, error) {
	return s.repo.ChannelModelByKey(channelID, strings.TrimPrefix(strings.TrimSpace(modelKey), "models/"))
}

func (s *Service) SystemChannelHasProtocol(channelID string, protocol model.ChannelInterfaceType) (bool, error) {
	items, err := s.repo.ChannelModels(channelID, false)
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if item.Protocol == protocol {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) FetchAdminChannelModels(ctx context.Context, actor *model.User, channelID string) (*AdminChannelModelFetchResult, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	channel, catalog, err := s.fetchAdminChannelCatalog(ctx, actor, channelID)
	if err != nil {
		return nil, err
	}
	models := modelcatalog.CatalogModelIDs(catalog)
	existing, err := s.repo.ChannelModels(channelID, true)
	if err != nil {
		return nil, err
	}
	missing, err := modelcatalog.MissingFetchedChannelModels(channelID, models, existing, retiredChannelModelKeys(channel.RetiredModelsJSON), s.repo.NextPrefixedID)
	if err != nil {
		return nil, err
	}
	applyChannelCatalogMetadata(*channel, missing, catalog)
	added, err := s.repo.CreateMissingChannelModels(missing)
	if err != nil {
		return nil, err
	}
	if err := s.refreshChannelCatalogMetadata(*channel, catalog, existing); err != nil {
		return nil, err
	}
	if added > 0 {
		s.invalidateRouteCatalog()
	}
	return &AdminChannelModelFetchResult{Models: models, Added: added, Catalog: catalog}, nil
}

func (s *Service) PreviewAdminChannelModels(ctx context.Context, actor *model.User, channelID string) ([]string, error) {
	catalog, err := s.PreviewAdminChannelModelCatalog(ctx, actor, channelID)
	if err != nil {
		return nil, err
	}
	return modelcatalog.CatalogModelIDs(catalog), nil
}

func (s *Service) PreviewAdminChannelModelCatalog(ctx context.Context, actor *model.User, channelID string) ([]ChannelModelCatalogItem, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	_, catalog, err := s.fetchAdminChannelCatalog(ctx, actor, channelID)
	if err != nil {
		return nil, err
	}
	return catalog, nil
}

func (s *Service) ImportAdminChannelModels(ctx context.Context, actor *model.User, channelID string, selected []string) (*AdminChannelModelFetchResult, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	channel, catalog, err := s.fetchAdminChannelCatalog(ctx, actor, channelID)
	if err != nil {
		return nil, err
	}
	models := modelcatalog.CatalogModelIDs(catalog)
	chosen, err := modelcatalog.SelectCatalogModels(selected, models)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.ChannelModels(channelID, true)
	if err != nil {
		return nil, err
	}
	missing, err := modelcatalog.MissingImportedChannelModels(channelID, chosen, existing, retiredChannelModelKeys(channel.RetiredModelsJSON), s.repo.NextPrefixedID)
	if err != nil {
		return nil, err
	}
	applyChannelCatalogMetadata(*channel, missing, catalog)
	added, err := s.repo.CreateMissingChannelModels(missing)
	if err != nil {
		return nil, err
	}
	if err := s.refreshChannelCatalogMetadata(*channel, catalog, existing); err != nil {
		return nil, err
	}
	if added > 0 {
		s.invalidateRouteCatalog()
	}
	return &AdminChannelModelFetchResult{Models: chosen, Added: added, Catalog: catalog}, nil
}

func (s *Service) fetchAdminChannelCatalog(ctx context.Context, actor *model.User, channelID string) (*model.ModelChannel, []ChannelModelCatalogItem, error) {
	channel, err := s.adminSystemChannel(channelID)
	if err != nil {
		return nil, nil, err
	}
	headers, err := ParseOutboundHeadersJSON(channel.HeadersJSON)
	if err != nil {
		return nil, nil, err
	}
	catalog, err := s.FetchChannelModelCatalog(ctx, actor, ChannelModelsRequest{BaseURL: channel.BaseURL, APIKey: channel.APIKey, APIFormat: channel.APIFormat, Headers: headers})
	if err != nil {
		return nil, nil, err
	}
	return channel, catalog, nil
}

func (s *Service) SaveAdminChannelModel(actor *model.User, channelID string, id string, req ChannelModelRequest) (*model.ChannelModel, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	channel, err := s.repo.AdminSystemChannel(channelID)
	if err != nil {
		return nil, err
	}
	modelKey, providerModelKey, capability, protocol, err := s.normalizeChannelModelContract(channel, req)
	if err != nil {
		return nil, err
	}
	conflict, conflictErr := s.repo.ChannelModelByKeyIncludingDisabled(channelID, modelKey)
	if conflictErr != nil && !errors.Is(conflictErr, gorm.ErrRecordNotFound) {
		return nil, conflictErr
	}
	if conflict != nil && conflict.ID != strings.TrimSpace(id) {
		return nil, BadAuthRequest("该渠道已存在模型 " + modelKey + "，请直接编辑已有模型")
	}
	var existing *model.ChannelModel
	if strings.TrimSpace(id) != "" {
		existing, err = s.repo.ChannelModelByID(channelID, id)
		if err != nil {
			return nil, err
		}
	}
	// 管理后台只需要选择模型和协议；文本、图片、视频的底层能力参数
	// 在没有显式输入时使用协议默认值，编辑已有模型则保留原配置。
	if req.CapabilityConfig == nil && (capability == "text" || capability == "image" || capability == "video") {
		req.CapabilityConfig = adminChannelModelDefaultCapabilityConfig(existing, protocol, providerModelKey)
	}
	item, tiers, err := modelcatalog.PrepareChannelModelSave(channelID, id, req, modelKey, providerModelKey, capability, protocol, existing, s.repo.NextPrefixedID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveChannelModelWithVariants(item, tiers); err != nil {
		return nil, err
	}
	item.Variants = tiers
	s.invalidateRouteCatalog()
	if err := s.syncChannelModelNames(channel); err != nil {
		return nil, err
	}
	if err := s.syncLogicalModelsFromChannelModel(actor, item); err != nil {
		return nil, err
	}
	hydrated := hydrateChannelModelUpstreamPricing([]model.ChannelModel{*item})
	return &hydrated[0], nil
}

func adminChannelModelDefaultCapabilityConfig(existing *model.ChannelModel, protocol model.ChannelInterfaceType, providerModelKey string) *ModelCapabilityConfig {
	if existing != nil && strings.TrimSpace(existing.CapabilityConfigJSON) != "" {
		if config, err := modelcatalog.DecodeModelCapabilityConfig(existing.CapabilityConfigJSON); err == nil && config != nil {
			return config
		}
	}
	return modelcatalog.DefaultModelCapabilityConfigForModel(string(protocol), providerModelKey)
}

func (s *Service) syncLogicalModelsFromChannelModel(actor *model.User, channelModel *model.ChannelModel) error {
	_ = actor
	_ = channelModel
	s.invalidateRouteCatalog()
	return nil
}

func (s *Service) TestAdminChannelModel(ctx context.Context, actor *model.User, channelID string, req ChannelModelRequest) (*AdminChannelModelTestResult, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	channel, err := s.adminSystemChannel(channelID)
	if err != nil {
		return nil, err
	}
	modelKey, providerModelKey, capability, protocol, err := s.normalizeChannelModelContract(channel, req)
	if err != nil {
		return nil, err
	}
	if capability == "text" || capability == "image" || capability == "video" {
		if _, err := NormalizeModelCapabilityConfigForModel(capability, string(protocol), providerModelKey, req.CapabilityConfig); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(channel.BaseURL) == "" || strings.TrimSpace(channel.APIKey) == "" {
		return nil, BadAuthRequest("请先在渠道中配置 Base URL 和 API Key")
	}
	if _, err := ValidateOutboundURL(channel.BaseURL); err != nil {
		return nil, err
	}
	headers, err := ParseOutboundHeadersJSON(channel.HeadersJSON)
	if err != nil {
		return nil, err
	}

	prompt := map[string]string{
		"text":  "Reply with OK.",
		"image": "A simple gray circle on a white background.",
		"video": "A static gray circle on a white background.",
		"audio": "Model test.",
	}[capability]
	videoSeconds := "6"
	videoSecondsValue := 6
	if protocol == model.ChannelInterfaceVolcengineJiMengVideo {
		videoSeconds = "5"
		videoSecondsValue = 5
	}
	imageSize, imageQuality := "", ""
	var imageProfile *ImageCapabilityConfig
	videoRatio, videoResolution := videoTestDefaults(nil)
	var videoProfile *VideoCapabilityConfig
	switch capability {
	case "image":
		profile, normalizeErr := NormalizeModelCapabilityConfigForModel(capability, string(protocol), providerModelKey, req.CapabilityConfig)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		imageProfile = profile.Image
		imageSize, imageQuality = imageTestDefaults(imageProfile)
	case "video":
		profile, normalizeErr := NormalizeModelCapabilityConfigForModel(capability, string(protocol), providerModelKey, req.CapabilityConfig)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		videoProfile = profile.Video
		videoRatio, videoResolution = videoTestDefaults(videoProfile)
	}
	input := canvasGenerationInput{
		Mode:   capability,
		Prompt: prompt,
		Config: providerConfig{
			ChannelID:          channel.ID,
			APIFormat:          channel.APIFormat,
			InterfaceType:      string(protocol),
			BaseURL:            channel.BaseURL,
			APIKey:             channel.APIKey,
			SecretKey:          channel.SecretKey,
			Headers:            headers,
			Model:              providerModelKey,
			ChannelModelKey:    modelKey,
			Size:               map[string]string{"image": imageSize, "video": videoRatio}[capability],
			Quality:            imageQuality,
			Count:              "1",
			VideoSeconds:       videoSeconds,
			VQuality:           videoResolution,
			VideoGenerateAudio: "false",
			VideoWatermark:     "false",
			AudioVoice:         "alloy",
			AudioFormat:        "mp3",
			AudioSpeed:         "1",
		},
		Metadata: map[string]interface{}{},
	}
	if capability == "image" {
		input.ImageCapability = imageProfile
	}
	if capability == "video" {
		input.VideoCapability = videoProfile
	}

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	testCtx = s.bindGenerationRuntime(testCtx, generation.CallMeta{
		UserID: actor.ID, ChannelID: channel.ID, Capability: capability,
		Operation: "admin_model_test", Model: modelKey, VideoSeconds: videoSecondsValue,
	})
	testCtx = context.WithValue(testCtx, providerAnalyticsKey{}, providerAnalyticsContext{
		UserID: actor.ID, ChannelID: channel.ID, Capability: capability,
		Operation: "admin_model_test", Model: modelKey, VideoSeconds: videoSecondsValue,
	})
	testCtx = withProtocolRegistry(testCtx, s.protocolRegistry())
	startedAt := time.Now()
	switch capability {
	case "text":
		_, err = runTextTask(testCtx, input)
	case "image":
		_, err = runImageTask(testCtx, input)
	case "video":
		_, err = runVideoTask(testCtx, input)
	case "audio":
		_, err = runAudioTask(testCtx, input)
	}
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		return nil, WrapAppError(status, "模型测试失败："+providerUserFacingErrorMessage(err), err)
	}
	return &AdminChannelModelTestResult{DurationMs: time.Since(startedAt).Milliseconds()}, nil
}

func (s *Service) DeleteAdminChannelModel(actor *model.User, channelID string, id string) error {
	_, err := s.DeleteAdminChannelModels(actor, channelID, []string{id})
	return err
}

func (s *Service) DeleteAdminChannelModels(actor *model.User, channelID string, ids []string) (int64, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return 0, err
	}
	if _, err := s.repo.AdminSystemChannel(channelID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, BadAuthRequest("系统渠道不存在或已删除")
		}
		return 0, err
	}
	modelIDs, err := normalizeAdminChannelModelDeleteIDs(ids)
	if err != nil {
		return 0, err
	}
	items, err := s.repo.ChannelModels(channelID, true)
	if err != nil {
		return 0, err
	}
	if err := modelcatalog.ValidateChannelModelDeleteSelection(modelIDs, items); err != nil {
		return 0, err
	}
	deleted, err := s.repo.DeleteChannelModels(channelID, modelIDs, time.Now())
	if errors.Is(err, repository.ErrChannelModelInUse) {
		return 0, BadAuthRequest("所选渠道模型中有模型仍被前台模型供应线路或进行中任务使用，本次未删除任何模型")
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, BadAuthRequest("所选渠道模型中存在已删除或不属于当前渠道的记录，请刷新后重试")
	}
	if err == nil {
		s.invalidateRouteCatalog()
	}
	return deleted, err
}

func (s *Service) syncInitialChannelModels(channel *model.ModelChannel, names []string) error {
	existing, err := s.repo.ChannelModels(channel.ID, true)
	if err != nil {
		return err
	}
	plan, err := modelcatalog.PlanInitialChannelModelSync(*channel, names, existing, s.repo.NextPrefixedID)
	if err != nil {
		return err
	}
	for index := range plan.Create {
		if err := s.repo.SaveChannelModel(&plan.Create[index]); err != nil {
			return err
		}
	}
	for index := range plan.Disable {
		if err := s.repo.SaveChannelModel(&plan.Disable[index]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureChannelModels(channelID string, includeDisabled bool) ([]model.ChannelModel, error) {
	items, err := s.repo.ChannelModels(channelID, includeDisabled)
	if err != nil || len(items) > 0 {
		return items, err
	}
	channel, err := s.repo.AdminSystemChannel(channelID)
	if err != nil {
		return nil, err
	}
	if err := s.syncInitialChannelModels(channel, channelModelNames(*channel)); err != nil {
		return nil, err
	}
	return s.repo.ChannelModels(channelID, includeDisabled)
}

func (s *Service) syncChannelModelNames(channel *model.ModelChannel) error {
	return s.repo.SyncChannelModelNames(channel.ID, time.Now())
}

func (s *Service) capabilityForProtocol(protocol model.ChannelInterfaceType) string {
	metadata, ok := s.channelProtocolMetadata(string(protocol))
	if !ok {
		return ""
	}
	return protocolCapabilityFromMetadata(metadata)
}

func hydrateChannelModelUpstreamPricing(items []model.ChannelModel) []model.ChannelModel {
	for index := range items {
		items[index].UpstreamPricing = nil
		raw := strings.TrimSpace(items[index].UpstreamPricingJSON)
		if raw == "" || raw == "null" {
			continue
		}
		var value map[string]any
		if json.Unmarshal([]byte(raw), &value) == nil && len(value) > 0 {
			items[index].UpstreamPricing = value
		}
	}
	return items
}

func applyChannelCatalogMetadata(channel model.ModelChannel, rows []model.ChannelModel, catalog []ChannelModelCatalogItem) {
	byKey := make(map[string]ChannelModelCatalogItem, len(catalog))
	for _, item := range catalog {
		byKey[channelModelCatalogKey(item.ID)] = item
	}
	for index := range rows {
		item, ok := byKey[channelModelCatalogKey(rows[index].ModelKey)]
		if !ok {
			item, ok = byKey[channelModelCatalogKey(rows[index].ProviderModelKey)]
		}
		if ok {
			applyChannelCatalogItemMetadata(&rows[index], channel, item)
		}
	}
}

func (s *Service) refreshChannelCatalogMetadata(channel model.ModelChannel, catalog []ChannelModelCatalogItem, existing []model.ChannelModel) error {
	byKey := make(map[string]ChannelModelCatalogItem, len(catalog))
	for _, item := range catalog {
		byKey[channelModelCatalogKey(item.ID)] = item
	}
	for index := range existing {
		item, ok := byKey[channelModelCatalogKey(existing[index].ModelKey)]
		if !ok {
			item, ok = byKey[channelModelCatalogKey(existing[index].ProviderModelKey)]
		}
		if !ok || !applyChannelCatalogItemMetadata(&existing[index], channel, item) {
			continue
		}
		if err := s.repo.SaveChannelModel(&existing[index]); err != nil {
			return err
		}
	}
	return nil
}

func applyChannelCatalogItemMetadata(channelModel *model.ChannelModel, channel model.ModelChannel, item ChannelModelCatalogItem) bool {
	if channelModel == nil {
		return false
	}
	changed := false
	displayName := strings.TrimSpace(item.DisplayName)
	if displayName != "" && (strings.TrimSpace(channelModel.DisplayName) == "" || channelModel.DisplayName == channelModel.ModelKey) && channelModel.DisplayName != displayName {
		channelModel.DisplayName = displayName
		changed = true
	}
	capability := inferCatalogCapability(item)
	if strings.TrimSpace(channelModel.Capability) == "" && capability != "" {
		channelModel.Capability = capability
		changed = true
	}
	if strings.TrimSpace(string(channelModel.Protocol)) == "" && capability != "" {
		channelModel.Protocol = inferCatalogProtocol(channel, capability, item)
		changed = true
	}
	if strings.TrimSpace(channelModel.CapabilityConfigJSON) == "" && capability != "" && strings.TrimSpace(string(channelModel.Protocol)) != "" {
		if encoded := catalogCapabilityConfigJSON(capability, string(channelModel.Protocol), item); encoded != "" {
			channelModel.CapabilityConfigJSON = encoded
			changed = true
		}
	}
	pricingJSON := ""
	if raw := strings.TrimSpace(string(item.VideoPricing)); raw != "" && raw != "null" && json.Valid([]byte(raw)) {
		pricingJSON = raw
	}
	if channelModel.UpstreamPricingJSON != pricingJSON {
		channelModel.UpstreamPricingJSON = pricingJSON
		channelModel.UpstreamPricing = nil
		changed = true
	}
	return changed
}

func inferCatalogCapability(item ChannelModelCatalogItem) string {
	if capability := modelcatalog.NormalizeCatalogModelType(item.ModelType); capability != "" {
		return capability
	}
	endpoints := strings.ToLower(strings.Join(item.SupportedEndpointTypes, " "))
	switch {
	case strings.Contains(endpoints, "video"):
		return "video"
	case strings.Contains(endpoints, "image"):
		return "image"
	case strings.Contains(endpoints, "audio"), strings.Contains(endpoints, "speech"), strings.Contains(endpoints, "music"):
		return "audio"
	case strings.Contains(endpoints, "response"), strings.Contains(endpoints, "chat"), strings.Contains(endpoints, "gemini"), strings.Contains(endpoints, "anthropic"), strings.Contains(endpoints, "messages"):
		return "text"
	}
	name := strings.ToLower(item.ID)
	switch {
	case strings.Contains(name, "seedance"), strings.Contains(name, "veo"), strings.Contains(name, "video"), strings.Contains(name, "-t2v"), strings.Contains(name, "-i2v"):
		return "video"
	case strings.Contains(name, "image"), strings.Contains(name, "flux"), strings.Contains(name, "banana"):
		return "image"
	case strings.Contains(name, "speech"), strings.Contains(name, "tts"), strings.Contains(name, "music"), strings.Contains(name, "audio"):
		return "audio"
	case strings.Contains(name, "gpt"), strings.Contains(name, "claude"), strings.Contains(name, "gemini"), strings.Contains(name, "deepseek"), strings.Contains(name, "qwen"), strings.Contains(name, "llama"):
		return "text"
	default:
		return ""
	}
}

func inferCatalogProtocol(channel model.ModelChannel, capability string, item ChannelModelCatalogItem) model.ChannelInterfaceType {
	endpoints := strings.ToLower(strings.Join(item.SupportedEndpointTypes, " "))
	switch capability {
	case "text":
		if strings.Contains(endpoints, "response") {
			return model.ChannelInterfaceOpenAIResponse
		}
		if strings.Contains(endpoints, "anthropic") || strings.Contains(endpoints, "messages") {
			return model.ChannelInterfaceClaudeAPI
		}
		return model.ChannelInterfaceChatCompletion
	case "image":
		if channel.APIFormat == "gemini" || strings.Contains(endpoints, "gemini") {
			return model.ChannelInterfaceGeminiImage
		}
		return model.ChannelInterfaceOpenAIImage
	case "video":
		if strings.Contains(endpoints, "gemini") {
			return model.ChannelInterfaceGeminiVeo
		}
		if strings.Contains(endpoints, "volcengine") || strings.Contains(endpoints, "ark") {
			return model.ChannelInterfaceVolcengineArkVideo
		}
		return model.ChannelInterfaceNewAPIChannel2
	case "audio":
		return model.ChannelInterfaceOpenAIAudio
	default:
		return ""
	}
}

func catalogCapabilityConfigJSON(capability string, protocol string, item ChannelModelCatalogItem) string {
	profile := modelcatalog.DefaultModelCapabilityConfigForModel(protocol, item.ID)
	if profile == nil {
		return ""
	}
	if capability == "video" && len(strings.TrimSpace(string(item.VideoCapabilities))) > 0 {
		var video VideoCapabilityConfig
		if json.Unmarshal(item.VideoCapabilities, &video) == nil {
			candidate := &ModelCapabilityConfig{Version: 1, Video: &video}
			if normalized, err := modelcatalog.NormalizeModelCapabilityConfigForModel(capability, protocol, item.ID, candidate); err == nil {
				profile = normalized
			}
		}
	}
	normalized, err := modelcatalog.NormalizeModelCapabilityConfigForModel(capability, protocol, item.ID, profile)
	if err != nil || normalized == nil {
		return ""
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	return string(encoded)
}
