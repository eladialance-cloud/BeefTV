package app

import (
	"encoding/json"
	"strings"

	"infinite-canvas/backend/internal/assistant"
	"infinite-canvas/backend/internal/beefapi"
	"infinite-canvas/backend/internal/modelcatalog"
	"infinite-canvas/backend/internal/workspace"
)

const (
	AssistantReasonModelNotConfigured  = assistant.ReasonModelNotConfigured
	AssistantReasonCredentialMissing   = assistant.ReasonCredentialMissing
	AssistantReasonProtocolUnsupported = assistant.ReasonProtocolUnsupported
)

type AssistantUnavailableError = assistant.UnavailableError
type AssistantProvider = assistant.Provider

func assistantUnavailable(reason, message string) error {
	return modelcatalog.AssistantUnavailable(reason, message)
}

type assistantModelProfile = modelcatalog.AssistantModelProfile
type assistantChannel = modelcatalog.AssistantChannel
type assistantConfigSnapshot = modelcatalog.AssistantConfigSnapshot

func (s *Service) assistantConfig() (assistantConfigSnapshot, error) {
	if s.IsHostedProfile() {
		return s.hostedAssistantConfig()
	}
	var snapshot assistantConfigSnapshot
	store, err := workspace.NewProviderConfig(s.dataDir)
	if err != nil {
		return snapshot, err
	}
	effective, _, err := store.LoadEffectiveModelConfig()
	if err != nil {
		return snapshot, assistantUnavailable(AssistantReasonModelNotConfigured, "本地模型配置不可读")
	}
	body, err := json.Marshal(effective.Config)
	if err != nil {
		return snapshot, err
	}
	if len(body) == 0 {
		return snapshot, assistantUnavailable(AssistantReasonModelNotConfigured, "本地模型配置为空")
	}
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return snapshot, assistantUnavailable(AssistantReasonModelNotConfigured, "本地模型配置无法解析")
	}
	snapshot.Revision = effective.Revision
	return snapshot, nil
}

// hostedAssistantConfig mirrors the public system catalog into the assistant
// snapshot. Hosted/admin model edits are database state; they must not depend
// on a desktop local-model-config.json that the browser cannot write.
func (s *Service) hostedAssistantConfig() (assistantConfigSnapshot, error) {
	var snapshot assistantConfigSnapshot
	if s == nil || s.repo == nil {
		return snapshot, assistantUnavailable(AssistantReasonModelNotConfigured, "模型目录尚未初始化")
	}
	channels, err := s.repo.SystemChannels(true)
	if err != nil {
		return snapshot, err
	}
	for _, row := range channels {
		if !row.Enabled {
			continue
		}
		if err := s.decryptSystemChannelSecrets(&row); err != nil {
			return snapshot, err
		}
		items, err := s.repo.ChannelModels(row.ID, false)
		if err != nil {
			return snapshot, err
		}
		if len(items) == 0 {
			continue
		}
		channel := assistantChannel{
			ID: row.ID, Name: row.PublicName(), BaseURL: row.BaseURL, APIKey: row.APIKey,
			Enabled: true, Scope: "system", APIFormat: row.APIFormat,
		}
		for _, item := range items {
			capability := strings.TrimSpace(item.Capability)
			if capability == "" {
				capability = s.capabilityForProtocol(item.Protocol)
			}
			channel.Models = append(channel.Models, item.ModelKey)
			channel.ModelProfiles = append(channel.ModelProfiles, assistantModelProfile{
				Model: item.ModelKey, Capability: capability, Protocol: string(item.Protocol),
			})
		}
		snapshot.Channels = append(snapshot.Channels, channel)
	}

	for _, channel := range snapshot.Channels {
		for _, profile := range channel.ModelProfiles {
			modelKey := channel.ID + "::" + profile.Model
			switch profile.Capability {
			case "text":
				if snapshot.TextModel != "" {
					continue
				}
				protocol, ok := modelcatalog.ChannelModelProtocol(channel, profile.Model)
				if ok && protocol != "" {
					snapshot.AssistantModel = modelKey
					snapshot.TextModel = modelKey
				}
			case "image":
				if snapshot.ImageModel == "" {
					snapshot.ImageModel = modelKey
				}
			case "video":
				if snapshot.VideoModel == "" {
					snapshot.VideoModel = modelKey
				}
			}
		}
	}
	return snapshot, nil
}

func (s *Service) ResolveAssistantProvider() (AssistantProvider, error) {
	snapshot, err := s.assistantConfig()
	if err != nil {
		return AssistantProvider{}, err
	}
	for index, channel := range snapshot.Channels {
		if strings.TrimSpace(channel.Name) == "" && channel.ID == beefapi.ChannelID {
			snapshot.Channels[index].Name = "BeefAPI"
		}
	}
	return modelcatalog.ResolveAssistantProvider(snapshot, s.assistantManagedCredentialLookup())
}

func (s *Service) assistantManagedCredentialLookup() modelcatalog.ManagedCredentialLookup {
	return func(channelID, credentialRef, baseURL string) (string, string, bool) {
		if !(beefapi.IsManagedChannel(channelID, credentialRef, baseURL) || channelID == beefapi.ChannelID) {
			return "", "", false
		}
		apiKey, resolvedBaseURL, _, _, lookupErr := s.lookupBeefAPICredential()
		if lookupErr != nil || strings.TrimSpace(apiKey) == "" {
			return "", "", false
		}
		return apiKey, resolvedBaseURL, true
	}
}

func assistantReasonMessage(reason string) string {
	return modelcatalog.AssistantReasonMessage(reason)
}

func (s *Service) resolveAssistantChannelModel(snapshot assistantConfigSnapshot, modelKey string) (AssistantProvider, string) {
	for index, channel := range snapshot.Channels {
		if strings.TrimSpace(channel.Name) == "" && channel.ID == beefapi.ChannelID {
			snapshot.Channels[index].Name = "BeefAPI"
		}
	}
	return modelcatalog.ResolveAssistantChannelModel(snapshot, modelKey, s.assistantManagedCredentialLookup())
}

func findAssistantChannel(channels []assistantChannel, channelID, modelID string) (assistantChannel, bool) {
	return modelcatalog.FindAssistantChannel(channels, channelID, modelID)
}

func channelModelProtocol(channel assistantChannel, modelID string) (string, bool) {
	return modelcatalog.ChannelModelProtocol(channel, modelID)
}

func assistantChannelName(channel assistantChannel) string {
	if name := modelcatalog.AssistantChannelName(channel); name != "" && name != channel.ID {
		return name
	}
	if channel.ID == beefapi.ChannelID {
		return "BeefAPI"
	}
	return modelcatalog.AssistantChannelName(channel)
}

func (s *Service) AssistantGenerationModel(kind string) (display string, modelKey string) {
	display, modelKey, _, _, _ = s.ResolveAssistantGenerationModel(kind, "")
	return display, modelKey
}

func (s *Service) AssistantGenerationModelSnapshot(kind string) (display string, modelKey string, revision int64, err error) {
	display, modelKey, revision, _, err = s.ResolveAssistantGenerationModel(kind, "")
	return
}

func (s *Service) ResolveAssistantGenerationModel(kind, selectedModel string) (display string, modelKey string, revision int64, kindMismatch bool, err error) {
	snapshot, err := s.assistantConfig()
	if err != nil {
		return "", "", 0, false, err
	}
	choice := modelcatalog.ResolveAssistantGenerationModel(snapshot, kind, selectedModel)
	return choice.Display, choice.ModelKey, snapshot.Revision, choice.KindMismatch, nil
}
