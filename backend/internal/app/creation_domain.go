package app

import (
	"encoding/json"

	"infinite-canvas/backend/internal/canvas"
	"infinite-canvas/backend/internal/creation"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	localtask "infinite-canvas/backend/internal/task"
)

func (s *Service) creationDomain() *creation.Service {
	return creation.New(s.repo, creation.Dependencies{
		Tasks:   creationTasksAdapter{s},
		Secrets: creationSecretsAdapter{s},
		Quota:   creationQuotaAdapter{s},
		Media:   creationMediaAdapter{s},
		Kinds:   creationKindsAdapter{},
		Now:     nil,
		NewID:   kernel.NewID,
	})
}

func toCreationCommand(req CreationRequest) creation.Command {
	return creation.Command{
		Guard:                req.CreationGuard,
		ClientKey:            req.ClientKey,
		CanvasID:             req.CanvasID,
		Revision:             req.Revision,
		ExpectedEpoch:        req.ExpectedEpoch,
		State:                req.State,
		Status:               req.Status,
		ProposalVersion:      req.ProposalVersion,
		Proposal:             req.Proposal,
		Ops:                  req.Ops,
		ItemKey:              req.ItemKey,
		Task:                 toCreationTaskRequest(req.Request),
		SubmissionIDs:        req.SubmissionIDs,
		SubmissionID:         req.SubmissionID,
		ExpectedSnapshotHash: req.ExpectedSnapshotHash,
		Document:             req.Document,
	}
}

func toCreationTaskRequest(req CreateTaskRequest) creation.TaskRequest {
	out := creation.TaskRequest{
		ProjectID:      req.ProjectID,
		Type:           req.Type,
		Operation:      req.Operation,
		Prompt:         req.Prompt,
		Provider:       req.Provider,
		Model:          req.Model,
		LogicalModelID: req.LogicalModelID,
		Input:          req.Input,
		TraceID:        req.TraceID,
		RequestID:      req.RequestID,
	}
	return out
}

type creationTasksAdapter struct{ s *Service }

func (a creationTasksAdapter) Prepare(userID string, req creation.TaskRequest) (*creation.PreparedTask, error) {
	task, err := a.s.CreateLocalTask(userID, localtask.CreateRequest{
		ProjectID:      req.ProjectID,
		Type:           req.Type,
		Operation:      req.Operation,
		Prompt:         req.Prompt,
		Provider:       req.Provider,
		Model:          req.Model,
		LogicalModelID: req.LogicalModelID,
		Input:          req.Input,
		TraceID:        req.TraceID,
		RequestID:      req.RequestID,
		AdmissionID:    req.AdmissionID,
		PrepareOnly:    true,
	})
	if err != nil {
		return nil, err
	}
	var input map[string]any
	if err = json.Unmarshal([]byte(task.InputJSON), &input); err != nil {
		return nil, err
	}
	config, _ := input["config"].(map[string]any)
	return &creation.PreparedTask{Task: task, Input: input, Config: config}, nil
}

func (a creationTasksAdapter) Admit(userID string, repo *repository.Repository, task *model.Task) (*model.Task, error) {
	policy, err := a.s.runtimePolicyWithRepo(repo)
	if err != nil {
		return nil, err
	}
	if err = createTaskWithStorageQuotaRepository(repo, task, policy); err != nil {
		return nil, err
	}
	return task, nil
}

// ReserveCost 在创作事务之外完成报价与冻结；Admit 只负责事务内写任务，
// 这样 SQLite 单连接不会被嵌套的积分事务卡死。
func (a creationTasksAdapter) ReserveCost(userID string, task *model.Task) error {
	_, err := a.s.TaskService().ReserveTaskCost(userID, task, false)
	return err
}

func (a creationTasksAdapter) ReleaseCost(userID, taskID string) error {
	return a.s.TaskService().ReleaseTaskCost(userID, taskID, "创作执行项未入库")
}

type creationSecretsAdapter struct{ s *Service }

func (a creationSecretsAdapter) Protect(input map[string]any) error {
	return a.s.protectTaskSecrets(input)
}

type creationQuotaAdapter struct{ s *Service }

func (a creationQuotaAdapter) ValidateRun(userID string, repo *repository.Repository, creating bool, delta int64) error {
	return a.s.validateCreationStorage(repo, userID, creating, delta)
}

func (a creationQuotaAdapter) ValidateCanvas(userID string, repo *repository.Repository, creating bool, delta int64) error {
	policy, err := a.s.runtimePolicyWithRepo(repo)
	if err != nil {
		return err
	}
	usage, err := repo.UserStorageUsage(userID)
	if err != nil {
		return err
	}
	return validateStructuredStorageQuotaWithPolicy(usage, "canvas", creating, delta, policy.Resource)
}

type creationMediaAdapter struct{ s *Service }

func (a creationMediaAdapter) ValidateDocument(userID string, repo *repository.Repository, raw json.RawMessage) error {
	if repo == nil {
		return kernel.NewAppError(kernel.CodeInternal, "画布媒体校验缺少事务仓储")
	}
	return canvas.New(repo, newCanvasHostWithRepo(a.s, repo)).ValidateCanvasMediaAssets(userID, raw)
}

type creationKindsAdapter struct{}

func (creationKindsAdapter) UsesWorkflow(input map[string]any) bool {
	return taskInputUsesWorkflowProvider(input)
}

func (creationKindsAdapter) UsesTextReplay(input map[string]any) bool {
	return isTextReplayTaskRequest(input)
}

func (s *Service) validateCreationStorage(repo *repository.Repository, userID string, creating bool, delta int64) error {
	usage, err := repo.UserStorageUsage(userID)
	if err != nil {
		return err
	}
	_, bytes, err := repo.CreationStorageUsage(userID)
	if err != nil {
		return err
	}
	policy, err := s.runtimePolicyWithRepo(repo)
	if err != nil {
		return err
	}
	return validateStructuredStorageQuotaWithPolicy(usage, "canvas", creating, bytes+delta, policy.Resource)
}
