package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/url"
	"path"
	"strings"

	"infinite-canvas/backend/internal/protocol"
)

const toAPIImageUploadLimitBytes = 10 * 1024 * 1024

// PrepareToAPIUploadImages turns private canvas images into ToAPI public URLs.
// ToAPI generation endpoints do not accept inline image bytes.
func PrepareToAPIUploadImages(ctx context.Context, config Config, input *Input) error {
	if input == nil || !IsToAPIMediaConfig(config) {
		return nil
	}
	for index := range input.ReferenceImages {
		media := &input.ReferenceImages[index]
		if IsPublicMediaURL(strings.TrimSpace(media.URL)) {
			continue
		}
		dataURL := strings.TrimSpace(media.DataURL)
		if dataURL == "" && strings.HasPrefix(strings.TrimSpace(media.URL), "data:") {
			dataURL = strings.TrimSpace(media.URL)
		}
		if dataURL == "" {
			continue
		}
		data, mimeType, err := MediaBytes(Media{DataURL: dataURL, Type: media.Type, MimeType: media.MimeType})
		if err != nil {
			return fmt.Errorf("读取 ToAPI 参考图片失败：%w", err)
		}
		if len(data) == 0 {
			return errors.New("ToAPI 参考图片内容为空")
		}
		if len(data) > toAPIImageUploadLimitBytes {
			data, mimeType, err = fitToAPIImageUpload(data, mimeType)
			if err != nil {
				return err
			}
			dataURL = DataURL(mimeType, data)
		}
		filename := strings.TrimSpace(media.Name)
		if filename == "" {
			filename = fmt.Sprintf("reference-%d%s", index+1, toAPIImageExtension(mimeType))
		} else if len(data) <= toAPIImageUploadLimitBytes && mimeType == "image/jpeg" && !strings.EqualFold(path.Ext(filename), ".jpg") && !strings.EqualFold(path.Ext(filename), ".jpeg") {
			filename = strings.TrimSuffix(filename, path.Ext(filename)) + ".jpg"
		}
		spec := protocol.RequestSpec{
			Method:      "POST",
			Path:        "/v1/uploads/images",
			ContentType: "multipart/form-data",
			Body:        map[string]any{"purpose": "generation"},
			Files: []protocol.RequestFilePart{{
				Name: "file", Filename: filename, MIMEType: mimeType,
				Reference: protocol.MediaReference{DataURL: dataURL, MIMEType: mimeType, Kind: "image"},
			}},
			Auth: protocol.ManifestAuth{Type: "bearer", Field: "apiKey"},
		}
		body, err := ExecuteProtocolRequest(WithRequestKind(ctx, "upload"), config, spec)
		if err != nil {
			return fmt.Errorf("上传 ToAPI 参考图片失败：%w", err)
		}
		var response struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
			Data    struct {
				URL string `json:"url"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return fmt.Errorf("解析 ToAPI 图片上传响应失败：%w", err)
		}
		if !response.Success || strings.TrimSpace(response.Data.URL) == "" {
			return fmt.Errorf("ToAPI 图片上传失败：%s", defaultString(strings.TrimSpace(response.Message), "响应缺少图片 URL"))
		}
		if err := validateToAPIUploadURL(response.Data.URL); err != nil {
			return err
		}
		media.URL = strings.TrimSpace(response.Data.URL)
		media.DataURL = ""
		media.StorageKey = ""
		media.MimeType = mimeType
		media.Bytes = int64(len(data))
	}
	return nil
}

func fitToAPIImageUpload(data []byte, mimeType string) ([]byte, string, error) {
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("ToAPI 参考图片超过 10MB 且无法压缩：%w", err)
	}
	bounds := source.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 || int64(bounds.Dx())*int64(bounds.Dy()) > 50_000_000 {
		return nil, "", errors.New("ToAPI 参考图片超过 10MB 且尺寸过大，请先压缩后重试")
	}
	flattened := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(flattened, flattened.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(flattened, flattened.Bounds(), source, bounds.Min, draw.Over)
	for scale := 1.0; scale >= 0.25; scale *= 0.82 {
		candidate := resizeRGBNearest(flattened, scale)
		for _, quality := range []int{88, 80, 72, 64, 56} {
			var encoded bytes.Buffer
			if err := jpeg.Encode(&encoded, candidate, &jpeg.Options{Quality: quality}); err != nil {
				return nil, "", fmt.Errorf("压缩 ToAPI 参考图片失败：%w", err)
			}
			if encoded.Len() <= toAPIImageUploadLimitBytes {
				return encoded.Bytes(), "image/jpeg", nil
			}
		}
	}
	return nil, "", fmt.Errorf("ToAPI 参考图片无法压缩到 10MB 以内（原格式 %s）", mimeType)
}

func resizeRGBNearest(source *image.RGBA, scale float64) *image.RGBA {
	if scale >= 1 {
		return source
	}
	width := max(1, int(float64(source.Bounds().Dx())*scale))
	height := max(1, int(float64(source.Bounds().Dy())*scale))
	resized := image.NewRGBA(image.Rect(0, 0, width, height))
	sourceWidth := source.Bounds().Dx()
	sourceHeight := source.Bounds().Dy()
	for y := 0; y < height; y++ {
		sourceY := min(sourceHeight-1, y*sourceHeight/height)
		for x := 0; x < width; x++ {
			sourceX := min(sourceWidth-1, x*sourceWidth/width)
			resized.SetRGBA(x, y, source.RGBAAt(sourceX, sourceY))
		}
	}
	return resized
}

func validateToAPIUploadURL(value string) error {
	address, err := url.Parse(strings.TrimSpace(value))
	if err != nil || address.Scheme != "https" || address.Hostname() == "" || address.User != nil {
		return errors.New("ToAPI 图片上传接口没有返回安全的 HTTPS 地址")
	}
	return nil
}

func toAPIImageExtension(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}
