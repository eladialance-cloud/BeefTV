package app

import (
	"infinite-canvas/backend/internal/model"
	localtask "infinite-canvas/backend/internal/task"
)

// CreateLocalTask is the domain-typed admission entry used by standalone HTTP
// registration. Trusted AdmissionID and PrepareOnly stay on the domain command;
// public JSON cannot set them.
func (s *Service) CreateLocalTask(userID string, request localtask.CreateRequest) (*model.Task, error) {
	resolved, err := s.applyCreationPrompt(CreateTaskRequest{
		ProjectID:      request.ProjectID,
		Type:           request.Type,
		Operation:      request.Operation,
		Prompt:         request.Prompt,
		Provider:       request.Provider,
		Model:          request.Model,
		LogicalModelID: request.LogicalModelID,
		Input:          request.Input,
		TraceID:        request.TraceID,
		RequestID:      request.RequestID,
	})
	if err != nil {
		return nil, err
	}
	request.ProjectID = resolved.ProjectID
	request.Type = resolved.Type
	request.Operation = resolved.Operation
	request.Prompt = resolved.Prompt
	request.Provider = resolved.Provider
	request.Model = resolved.Model
	request.LogicalModelID = resolved.LogicalModelID
	request.Input = resolved.Input
	request.TraceID = resolved.TraceID
	request.RequestID = resolved.RequestID
	return s.taskDomain().CreateTask(userID, request)
}
