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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// newS3Client 统一探针与读写后端的目标解析：接入点、区域、静态密钥与请求风格。
func newS3Client(target Target, timeout time.Duration) *s3.Client {
	region := strings.TrimSpace(target.Region)
	if region == "" {
		region = "us-east-1"
	}
	options := s3.Options{
		Region:       region,
		Credentials:  credentials.NewStaticCredentialsProvider(target.AccessKeyID, target.SecretAccessKey, ""),
		UsePathStyle: target.PathStyle,
	}
	if endpoint := strings.TrimSpace(target.Endpoint); endpoint != "" {
		options.BaseEndpoint = aws.String(endpoint)
	}
	if timeout > 0 {
		options.HTTPClient = &http.Client{Timeout: timeout}
	}
	return s3.New(options)
}

// S3BlobStore 把 asset.BlobStore 的字节读写落到 S3 兼容对象存储。
//
// 它满足现有端口：Write/Delete/Exists 直接映射 PutObject/DeleteObject/HeadObject；
// Open 需要返回可 Seek 且可 Stat 的 *os.File，因此把对象取回本地临时文件后再交给
// 调用方。这是当前的既定契约，换成流式 Range 读取需要先改宽端口。
type S3BlobStore struct {
	client *s3.Client
	bucket string
	prefix string
	// cdnBaseURL 非空时浏览器直连 CDN，不再签发临时地址。
	cdnBaseURL string
	tempDir    string
}

// DirectURL 返回浏览器可用的下载地址。配置了 CDN 时优先用它：该地址稳定，
// 跳转响应可以缓存；否则签发短时 S3 GET 地址，跳转响应绝不能缓存。
func (s *S3BlobStore) DirectURL(objectKey string, expires time.Time) (string, bool, error) {
	if s == nil {
		return "", false, errors.New("对象存储未初始化")
	}
	if base := strings.TrimSpace(s.cdnBaseURL); base != "" {
		escaped := (&url.URL{Path: s.key(objectKey)}).EscapedPath()
		return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(escaped, "/"), true, nil
	}
	ttl := time.Until(expires)
	if ttl <= 0 {
		return "", false, errors.New("直连地址有效期必须为正")
	}
	request, err := s3.NewPresignClient(s.client).PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(s.key(objectKey)),
	}, func(options *s3.PresignOptions) {
		options.Expires = ttl
	})
	if err != nil {
		return "", false, fmt.Errorf("签发对象存储直连地址失败: %w", err)
	}
	return request.URL, false, nil
}

// NewS3BlobStore 用一条存储配置构造读写后端。tempDir 用于 Open 的暂存文件，
// 留空时使用系统临时目录。
func NewS3BlobStore(target Target, tempDir string) (*S3BlobStore, error) {
	if strings.TrimSpace(target.Bucket) == "" {
		return nil, fmt.Errorf("缺少 Bucket")
	}
	if strings.TrimSpace(target.Endpoint) == "" && strings.TrimSpace(target.Region) == "" {
		return nil, fmt.Errorf("缺少接入点或 Region")
	}
	tempDir = strings.TrimSpace(tempDir)
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		return nil, err
	}
	return &S3BlobStore{
		client: newS3Client(target, 0), bucket: strings.TrimSpace(target.Bucket),
		prefix: strings.Trim(strings.TrimSpace(target.Prefix), "/"), tempDir: tempDir,
		cdnBaseURL: strings.TrimSpace(target.CDNBaseURL),
	}, nil
}

func (s *S3BlobStore) key(objectKey string) string {
	key := strings.Trim(strings.TrimSpace(objectKey), "/")
	if s.prefix == "" {
		return key
	}
	return s.prefix + "/" + key
}

func (s *S3BlobStore) Write(objectKey string, body io.Reader) error {
	if body == nil {
		return fmt.Errorf("对象内容为空")
	}
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(s.key(objectKey)), Body: body,
	})
	if err != nil {
		return fmt.Errorf("写入对象存储失败: %w", err)
	}
	return nil
}

// Open 把对象下载到临时文件并返回句柄。Unix 上创建后立刻 unlink，关闭即回收；
// Windows 不允许删除仍打开的文件，因此由 PruneTempFiles 兜底清理。
func (s *S3BlobStore) Open(objectKey string) (*os.File, error) {
	object, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(s.key(objectKey)),
	})
	if err != nil {
		return nil, fmt.Errorf("读取对象存储失败: %w", err)
	}
	defer object.Body.Close()
	s.PruneTempFiles(30 * time.Minute)

	temp, err := os.CreateTemp(s.tempDir, "beeftv-s3-*")
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(temp, object.Body); err != nil {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
		return nil, fmt.Errorf("写入暂存文件失败: %w", err)
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

func (s *S3BlobStore) Delete(objectKey string) error {
	_, err := s.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(s.key(objectKey)),
	})
	if err != nil {
		return fmt.Errorf("删除对象存储对象失败: %w", err)
	}
	return nil
}

func (s *S3BlobStore) Exists(objectKey string) error {
	_, err := s.client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(s.key(objectKey)),
	})
	if err != nil {
		return fmt.Errorf("对象存储对象不可用: %w", err)
	}
	return nil
}

// PruneTempFiles 清理异常退出遗留的暂存文件（Windows 无法 unlink-on-open）。
func (s *S3BlobStore) PruneTempFiles(olderThan time.Duration) {
	if s == nil || s.tempDir == "" {
		return
	}
	cutoff := time.Now().Add(-olderThan)
	entries, err := os.ReadDir(s.tempDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "beeftv-s3-") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(s.tempDir, entry.Name()))
	}
}

// 编译期断言：S3BlobStore 与 asset.BlobStore 的端口形状一致。
// 这里写成匿名接口，让本文件不依赖资源域；接入处（router.go）另有真实断言。
var _ interface {
	Write(string, io.Reader) error
	Open(string) (*os.File, error)
	Delete(string) error
	Exists(string) error
} = (*S3BlobStore)(nil)
