package storageconfig

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

// COSBlobStore uses Tencent Cloud's native COS protocol. The S3-compatible
// endpoint is not used here because COS and the AWS SDK disagree on request
// signing/checksum behavior for some uploads.
type COSBlobStore struct {
	client     *cos.Client
	prefix     string
	cdnBaseURL string
	tempDir    string
}

func newCOSClient(target Target, timeout time.Duration) (*cos.Client, error) {
	region := strings.TrimSpace(target.Region)
	if region == "" {
		region = cosRegionFromEndpoint(target.Endpoint)
	}
	if region == "" {
		return nil, errors.New("腾讯 COS 缺少 Region")
	}
	bucket := strings.TrimSpace(target.Bucket)
	if bucket == "" {
		return nil, errors.New("腾讯 COS 缺少 Bucket")
	}
	secure := true
	if parsed, err := url.Parse(strings.TrimSpace(target.Endpoint)); err == nil && strings.EqualFold(parsed.Scheme, "http") {
		secure = false
	}
	bucketURL, err := cos.NewBucketURL(bucket, region, secure)
	if err != nil {
		return nil, fmt.Errorf("腾讯 COS Bucket 或 Region 格式错误: %w", err)
	}
	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &cos.AuthorizationTransport{
			SecretID:  strings.TrimSpace(target.AccessKeyID),
			SecretKey: target.SecretAccessKey,
		},
	}
	return cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, httpClient), nil
}

func cosRegionFromEndpoint(endpoint string) string {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.ToLower(parsed.Hostname()), ".")
	for index, part := range parts {
		if part == "cos" && index+1 < len(parts) {
			return parts[index+1]
		}
	}
	return ""
}

func NewCOSBlobStore(target Target, tempDir string) (*COSBlobStore, error) {
	client, err := newCOSClient(target, 0)
	if err != nil {
		return nil, err
	}
	tempDir = strings.TrimSpace(tempDir)
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		return nil, err
	}
	return &COSBlobStore{
		client: client, prefix: strings.Trim(strings.TrimSpace(target.Prefix), "/"),
		cdnBaseURL: strings.TrimSpace(target.CDNBaseURL), tempDir: tempDir,
	}, nil
}

func (s *COSBlobStore) key(objectKey string) string {
	key := strings.Trim(strings.TrimSpace(objectKey), "/")
	if s.prefix == "" {
		return key
	}
	return s.prefix + "/" + key
}

func (s *COSBlobStore) Write(objectKey string, body io.Reader) error {
	if body == nil {
		return errors.New("对象内容为空")
	}
	if _, err := s.client.Object.Put(context.Background(), s.key(objectKey), body, nil); err != nil {
		return fmt.Errorf("写入腾讯 COS 失败: %w", err)
	}
	return nil
}

func (s *COSBlobStore) Open(objectKey string) (*os.File, error) {
	response, err := s.client.Object.Get(context.Background(), s.key(objectKey), nil)
	if err != nil {
		return nil, fmt.Errorf("读取腾讯 COS 失败: %w", err)
	}
	defer response.Body.Close()
	s.PruneTempFiles(30 * time.Minute)

	temp, err := os.CreateTemp(s.tempDir, "beeftv-cos-*")
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(temp, response.Body); err != nil {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
		return nil, fmt.Errorf("写入腾讯 COS 暂存文件失败: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
		return nil, err
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
		return nil, err
	}
	_ = os.Remove(temp.Name())
	return temp, nil
}

func (s *COSBlobStore) Delete(objectKey string) error {
	if _, err := s.client.Object.Delete(context.Background(), s.key(objectKey), nil); err != nil {
		return fmt.Errorf("删除腾讯 COS 对象失败: %w", err)
	}
	return nil
}

func (s *COSBlobStore) Exists(objectKey string) error {
	if _, err := s.client.Object.Head(context.Background(), s.key(objectKey), nil); err != nil {
		return fmt.Errorf("腾讯 COS 对象不可用: %w", err)
	}
	return nil
}

func (s *COSBlobStore) DirectURL(objectKey string, expires time.Time) (string, bool, error) {
	if base := strings.TrimSpace(s.cdnBaseURL); base != "" {
		escaped := (&url.URL{Path: s.key(objectKey)}).EscapedPath()
		return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(escaped, "/"), true, nil
	}
	ttl := time.Until(expires)
	if ttl <= 0 {
		return "", false, errors.New("腾讯 COS 直连地址有效期必须为正")
	}
	signed, err := s.client.Object.GetPresignedURL2(context.Background(), http.MethodGet, s.key(objectKey), ttl, nil, true)
	if err != nil {
		return "", false, fmt.Errorf("签发腾讯 COS 直连地址失败: %w", err)
	}
	return signed.String(), false, nil
}

func (s *COSBlobStore) PruneTempFiles(olderThan time.Duration) {
	if s == nil || s.tempDir == "" {
		return
	}
	cutoff := time.Now().Add(-olderThan)
	entries, err := os.ReadDir(s.tempDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "beeftv-cos-") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(s.tempDir, entry.Name()))
	}
}

var _ interface {
	Write(string, io.Reader) error
	Open(string) (*os.File, error)
	Delete(string) error
	Exists(string) error
} = (*COSBlobStore)(nil)
