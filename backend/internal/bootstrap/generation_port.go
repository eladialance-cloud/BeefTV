package bootstrap

import (
	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/localapp"
	"infinite-canvas/backend/internal/model"
	localtask "infinite-canvas/backend/internal/task"
)

// creationAwareGenerationPort keeps the public task route on the application
// admission path so private workflow prompt templates are compiled first.
type creationAwareGenerationPort struct {
	service *app.Service
}

func (p creationAwareGenerationPort) CreateTask(userID string, request localtask.CreateRequest) (*model.Task, error) {
	return p.service.CreateLocalTask(userID, request)
}

var _ localapp.GenerationPort = creationAwareGenerationPort{}
