package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFitToAPIImageUploadCompressesOversizeImage(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 2800, 2800))
	random := rand.New(rand.NewSource(42))
	for y := 0; y < source.Bounds().Dy(); y++ {
		for x := 0; x < source.Bounds().Dx(); x++ {
			source.SetNRGBA(x, y, color.NRGBA{
				R: uint8(random.Intn(256)),
				G: uint8(random.Intn(256)),
				B: uint8(random.Intn(256)),
				A: 255,
			})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	if encoded.Len() <= toAPIImageUploadLimitBytes {
		t.Fatalf("fixture size = %d, expected over limit", encoded.Len())
	}

	data, mimeType, err := fitToAPIImageUpload(encoded.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > toAPIImageUploadLimitBytes {
		t.Fatalf("compressed size = %d", len(data))
	}
	if mimeType != "image/jpeg" {
		t.Fatalf("mime type = %q", mimeType)
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("compressed image is not decodable: %v", err)
	}
}

func TestPrepareToAPIUploadImagesUploadsPrivateCanvasImage(t *testing.T) {
	t.Setenv("CANVAS_ALLOW_PRIVATE_UPSTREAMS", "1")
	const uploaded = "https://files.toapis.example/uploads/reference.png"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/uploads/images" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer toapi-key" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		if err := r.ParseMultipartForm(12 << 20); err != nil {
			t.Fatal(err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if header.Filename != "reference.png" {
			t.Fatalf("filename = %q", header.Filename)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"url": uploaded, "mime_type": "image/png", "size": 8},
		})
	}))
	defer server.Close()

	input := Input{ReferenceImages: []Media{{
		Name: "reference.png", MimeType: "image/png",
		DataURL: "data:image/png;base64,iVBORw0KGgo=",
	}}}
	if err := PrepareToAPIUploadImages(context.Background(), Config{
		InterfaceType: "toapis-video", BaseURL: server.URL, APIKey: "toapi-key",
	}, &input); err != nil {
		t.Fatal(err)
	}
	if input.ReferenceImages[0].URL != uploaded || input.ReferenceImages[0].DataURL != "" {
		t.Fatalf("media = %#v", input.ReferenceImages[0])
	}
}

func TestPrepareToAPIUploadImagesRejectsOversizeAndUnsafeURL(t *testing.T) {
	oversize := Input{ReferenceImages: []Media{{
		DataURL: DataURL("image/png", make([]byte, toAPIImageUploadLimitBytes+1)),
	}}}
	if err := PrepareToAPIUploadImages(context.Background(), Config{InterfaceType: "toapis-image"}, &oversize); err == nil {
		t.Fatal("oversized image accepted")
	}
	for _, value := range []string{"http://files.example/a.png", "https://user:pass@files.example/a.png", "https:///missing-host"} {
		if err := validateToAPIUploadURL(value); err == nil {
			t.Fatalf("unsafe URL accepted: %s", value)
		}
	}
	if err := validateToAPIUploadURL("https://files.example/a.png?id=1"); err != nil {
		t.Fatal(fmt.Errorf("safe URL rejected: %w", err))
	}
}

type toAPIResourcePort struct{}

func (toAPIResourcePort) Lookup(_ string, resourceID string) (ResourceInfo, error) {
	return ResourceInfo{
		ID: resourceID, Status: "ready", Provider: "s3",
		MimeType: map[string]string{
			"voice": "audio/mpeg",
			"clip":  "video/mp4",
		}[resourceID],
		Size: 711271,
	}, nil
}

func (toAPIResourcePort) Open(_ string, resourceID string) (ResourceInfo, io.ReadCloser, error) {
	info, _ := toAPIResourcePort{}.Lookup("", resourceID)
	return info, io.NopCloser(bytes.NewReader([]byte("local-only"))), nil
}

func (toAPIResourcePort) PublicURL(info ResourceInfo, _ time.Time) (string, error) {
	return "https://cdn.example/" + info.ID, nil
}

func (toAPIResourcePort) HTTPSPublicURL(info ResourceInfo, _ time.Time) (string, error) {
	return "https://cdn.example/" + info.ID, nil
}

func (toAPIResourcePort) LocalMode() bool { return false }

func TestHydrateToAPINonImageReferencesUsesPublicURLs(t *testing.T) {
	ctx := WithRuntime(context.Background(), Runtime{Resources: toAPIResourcePort{}})
	input := Input{
		Mode:            "video",
		Config:          Config{InterfaceType: "toapis-video"},
		ReferenceVideos: []Media{{StorageKey: "resource:clip"}},
		ReferenceAudios: []Media{{StorageKey: "resource:voice"}},
	}
	if err := HydrateMedia(ctx, "user-1", &input, MediaHydrationPolicyFor(ctx, input)); err != nil {
		t.Fatal(err)
	}
	if input.ReferenceVideos[0].URL != "https://cdn.example/clip" || input.ReferenceVideos[0].DataURL != "" {
		t.Fatalf("video reference must use public URL, got %#v", input.ReferenceVideos[0])
	}
	if input.ReferenceAudios[0].URL != "https://cdn.example/voice" || input.ReferenceAudios[0].DataURL != "" {
		t.Fatalf("audio reference must use public URL, got %#v", input.ReferenceAudios[0])
	}
}
