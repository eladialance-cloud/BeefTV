package app

import (
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestEnrichAPICallLogRecordsStablePaymentRequiredCode(t *testing.T) {
	log := &model.ApiCallLog{Status: model.ApiCallStatusFailed, StatusCode: 402}
	(&Service{}).EnrichAPICallLog(log, []byte(`{"error":{"message":"insufficient balance api-key=secret"}}`))
	if log.ErrorCode != "quota_unknown" {
		t.Fatalf("ErrorCode = %q, want quota_unknown", log.ErrorCode)
	}
	if log.Error == "" || log.Error == "insufficient balance api-key=secret" || strings.Contains(log.Error, "secret") || strings.Contains(log.Error, "api-key") {
		t.Fatalf("unsafe or empty user-facing error: %q", log.Error)
	}
}

func TestCallLogKeepsSpecificFailureWithoutRawProviderEcho(t *testing.T) {
	for _, status := range []int{200, 400, 402, 451} {
		log := &model.ApiCallLog{Status: model.ApiCallStatusFailed, StatusCode: status}
		(&Service{}).EnrichAPICallLog(log, []byte(`{"error":{"code":"prompt_blocked","message":"content safety policy prompt=PRIVATE api_key=PRIVATE"}}`))
		if log.ErrorCode != "prompt_blocked" || !strings.Contains(log.Error, "内容安全审核") || strings.Contains(log.Error, "PRIVATE") {
			t.Fatalf("status %d: unsafe or generic diagnostics: %s %s", status, log.ErrorCode, log.Error)
		}
	}
}

func TestCreateEndpointPathIsNotAProviderTaskID(t *testing.T) {
	log := &model.ApiCallLog{
		Status:      model.ApiCallStatusSucceeded,
		StatusCode:  200,
		Capability:  "video",
		RequestKind: "create",
		Path:        "/v1/videos/generations",
	}
	(&Service{}).EnrichAPICallLog(log, nil)
	if log.ProviderRequestID != "" {
		t.Fatalf("create path became provider task id: %q", log.ProviderRequestID)
	}
}

func TestFailedCreateRequestIDIsNotAProviderTaskID(t *testing.T) {
	log := &model.ApiCallLog{
		Status:      model.ApiCallStatusFailed,
		StatusCode:  500,
		Capability:  "video",
		RequestKind: "create",
		Path:        "/v1/videos/generations",
	}
	(&Service{}).EnrichAPICallLog(log, []byte(`{"request_id":"upstream-request","error":{"code":"build_request_failed"}}`))
	if log.ProviderRequestID != "" {
		t.Fatalf("failed create request id became provider task id: %q", log.ProviderRequestID)
	}
}

func TestSuccessfulCreateResponseStillCapturesProviderTaskID(t *testing.T) {
	log := &model.ApiCallLog{
		Status:      model.ApiCallStatusSucceeded,
		StatusCode:  200,
		Capability:  "video",
		RequestKind: "create",
		Path:        "/v1/videos/generations",
	}
	(&Service{}).EnrichAPICallLog(log, []byte(`{"id":"tsk_vid_real","status":"pending"}`))
	if log.ProviderRequestID != "tsk_vid_real" {
		t.Fatalf("provider task id = %q", log.ProviderRequestID)
	}
}

func TestPollPathCanInferProviderTaskID(t *testing.T) {
	log := &model.ApiCallLog{
		Status:      model.ApiCallStatusSucceeded,
		StatusCode:  200,
		Capability:  "video",
		RequestKind: "poll",
		Path:        "/v1/videos/generations/tsk_vid_real",
	}
	(&Service{}).EnrichAPICallLog(log, nil)
	if log.ProviderRequestID != "tsk_vid_real" {
		t.Fatalf("poll provider task id = %q", log.ProviderRequestID)
	}
}
