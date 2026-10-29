package protocol

import (
	"context"
	"testing"
)

func TestToAPIVideoUsesUnifiedCreatePollAndResultMapping(t *testing.T) {
	adapter := officialPackageAdapter(t, "toapis-video.beeftv-plugin", "toapis-video")
	spec, err := adapter.BuildCreate(context.Background(), RequestContext{Request: GenerationRequest{
		Model:        "MiniMax-H3",
		Prompt:       "cinematic turn",
		Duration:     5,
		AspectRatio:  "16:9",
		Resolution:   "2K",
		GenerateAudio: true,
		Images: []MediaReference{
			{URL: "https://cdn.example/first.png", Role: "first_frame", Order: 1},
			{URL: "https://cdn.example/reference.png", Role: "reference_image", Order: 2},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Method != "POST" || spec.Path != "/v1/videos/generations" {
		t.Fatalf("create = %s %s", spec.Method, spec.Path)
	}
	body, ok := spec.Body.(map[string]any)
	if !ok {
		t.Fatalf("body = %#v", spec.Body)
	}
	if body["aspect_ratio"] != "16:9" || body["resolution"] != "2K" || len(body["image_with_roles"].([]any)) != 2 {
		t.Fatalf("body = %#v", body)
	}

	created, err := adapter.ParseCreate(context.Background(), []byte(`{"id":"tsk_vid_test","status":"queued"}`))
	if err != nil || created.TaskID != "tsk_vid_test" || created.Status != StatusPending {
		t.Fatalf("created = %#v err=%v", created, err)
	}
	poll, err := adapter.BuildPoll(context.Background(), PollContext{TaskID: created.TaskID})
	if err != nil || poll.Method != "GET" || poll.Path != "/v1/videos/generations/tsk_vid_test" {
		t.Fatalf("poll = %#v err=%v", poll, err)
	}
	completed, err := adapter.ParsePoll(context.Background(), PollContext{TaskID: created.TaskID}, []byte(`{"id":"tsk_vid_test","status":"completed","result":{"data":[{"url":"https://files.example/result.mp4"}]}}`))
	if err != nil || completed.Status != StatusSucceeded || completed.Result == nil || len(completed.Result.Videos) != 1 {
		t.Fatalf("completed = %#v err=%v", completed, err)
	}
}
