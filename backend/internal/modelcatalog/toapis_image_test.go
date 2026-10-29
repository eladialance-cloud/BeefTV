package modelcatalog

import "testing"

func TestToAPIImageCapabilitiesUseRatiosAndModelSpecificResolutions(t *testing.T) {
	gemini := DefaultImageCapabilityConfig("toapis-image", "gemini-3.1-flash-image-preview")
	if gemini.Size.Parameter != "aspect_ratio" || gemini.Size.Default != "1:1" {
		t.Fatalf("gemini image size = %#v, want aspect ratio 1:1", gemini.Size)
	}
	if gemini.Quality.Default != "1k" || !containsCapabilityString(gemini.Quality.Values, "4k") {
		t.Fatalf("gemini image quality = %#v, want 1k default and 4k support", gemini.Quality)
	}
	if gemini.References.MaxImages != 14 || gemini.References.MaxImageBytes != 10*1024*1024 {
		t.Fatalf("gemini image references = %#v", gemini.References)
	}

	seedream := DefaultImageCapabilityConfig("toapis-image", "doubao-seedream-5-0")
	if seedream.Size.Parameter != "aspect_ratio" || seedream.Size.Default != "1:1" {
		t.Fatalf("seedream image size = %#v, want aspect ratio 1:1", seedream.Size)
	}
	if seedream.Quality.Default != "2k" || len(seedream.Quality.Values) != 2 || !containsCapabilityString(seedream.Quality.Values, "3k") {
		t.Fatalf("seedream image quality = %#v, want 2k/3k support", seedream.Quality)
	}
	if seedream.References.MaxImages != 10 || seedream.MaxOutputs != 15 {
		t.Fatalf("seedream image limits = maxImages:%d maxOutputs:%d", seedream.References.MaxImages, seedream.MaxOutputs)
	}

}

func TestToAPIVideoCapabilitiesUseUnifiedTaskContract(t *testing.T) {
	video := DefaultModelCapabilityConfigForModel("toapis-video", "seedance-2")
	if video.Video == nil {
		t.Fatal("missing video capability")
	}
	if video.Video.Duration.Min != 4 || video.Video.Duration.Max != 15 || video.Video.Duration.Default != 5 {
		t.Fatalf("duration = %#v", video.Video.Duration)
	}
	if video.Video.References.MaxImages != 9 || video.Video.References.MaxVideos != 3 || video.Video.References.MaxAudios != 3 {
		t.Fatalf("references = %#v", video.Video.References)
	}
	if !video.Video.GenerateAudio.Supported || !containsCapabilityString(video.Video.Operations, "reference_to_video") {
		t.Fatalf("video options = %#v", video.Video)
	}
}

func TestToAPIVideoCapabilitiesRepairLegacyPersistedSeedanceDefaults(t *testing.T) {
	for _, name := range []string{"seedance-2", "seedance-2-5"} {
		legacy := DefaultModelCapabilityConfigForModel("", name)
		legacy.Video.References.MaxImages = 9
		legacy.Video.References.MaxVideos = 0
		legacy.Video.References.MaxAudios = 0
		legacy.Video.References.MaxVideoBytes = 0
		legacy.Video.References.MaxAudioBytes = 0
		legacy.Video.References.MaxVideoDuration = 0
		legacy.Video.References.MaxAudioDuration = 0
		legacy.Video.Operations = []string{"text_to_video", "image_to_video"}
		legacy.Video.DefaultOperation = "text_to_video"

		normalized, err := NormalizeModelCapabilityConfigForModel("video", "toapis-video", name, legacy)
		if err != nil {
			t.Fatalf("%s normalization error: %v", name, err)
		}
		if normalized == nil || normalized.Video == nil {
			t.Fatalf("%s normalized video is nil", name)
		}
		wantVideos, wantAudios := 3, 3
		if name == "seedance-2-5" {
			wantVideos, wantAudios = 10, 10
		}
		if normalized.Video.References.MaxVideos != wantVideos || normalized.Video.References.MaxAudios != wantAudios {
			t.Fatalf("%s reference counts = %#v", name, normalized.Video.References)
		}
		if !normalized.Video.GenerateAudio.Supported {
			t.Fatalf("%s generated-audio control was not repaired", name)
		}
		if !containsCapabilityString(normalized.Video.Operations, "reference_to_video") {
			t.Fatalf("%s operations = %v", name, normalized.Video.Operations)
		}
		if containsCapabilityString(normalized.Video.Operations, "audio_to_video") != (name == "seedance-2-5") {
			t.Fatalf("%s audio-only operation mismatch: %v", name, normalized.Video.Operations)
		}
	}
}

func TestToAPIVideoCapabilitiesPreserveExplicitAudioRestrictions(t *testing.T) {
	profile := DefaultModelCapabilityConfigForModel("toapis-video", "seedance-2")
	profile.Video.References.MaxAudios = 0
	profile.Video.Operations = []string{"text_to_video", "image_to_video"}

	normalized, err := NormalizeModelCapabilityConfigForModel("video", "toapis-video", "seedance-2", profile)
	if err != nil {
		t.Fatalf("normalization error: %v", err)
	}
	if normalized.Video.References.MaxAudios != 0 {
		t.Fatalf("explicit maxAudios was expanded: %#v", normalized.Video.References)
	}
	if containsCapabilityString(normalized.Video.Operations, "reference_to_video") {
		t.Fatalf("explicit operations were expanded: %v", normalized.Video.Operations)
	}
}
