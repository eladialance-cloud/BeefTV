package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"infinite-canvas/backend/internal/app"
)

func TestHostedAssistantHostResolvesTurnOwner(t *testing.T) {
	env := newAssistantTestEnv(t, nil)
	owner, err := env.service.LocalWorkspaceOwner()
	if err != nil {
		t.Fatal(err)
	}
	turnID := "0123456789abcdef"
	if _, err := env.service.BeginAssistantTurn(owner.ID, env.canvasID, turnID, app.AssistantTurnInput{PermissionMode: "canvas"}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodGet, "http://app.beeftv.example/api/assistant/runtime/turns/"+turnID, nil)
	request.Header.Set(assistantHostTokenHeader, assistantTestHostToken)
	request.Header.Set(assistantTurnHeader, turnID)
	context.Request = request
	MarkHostedRequest(context)

	user, err := currentUser(context, env.service)
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != owner.ID {
		t.Fatalf("host resolved user = %q, want %q", user.ID, owner.ID)
	}
}
