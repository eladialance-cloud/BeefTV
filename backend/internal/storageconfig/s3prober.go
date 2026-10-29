package storageconfig

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// S3Prober verifies an S3-compatible endpoint by writing, reading back and
// deleting one throwaway object. AWS S3, Aliyun OSS, Tencent COS, Qiniu and
// MinIO all speak this protocol; only the endpoint and path-style convention
// differ, and both come from the stored configuration.
type S3Prober struct {
	Timeout time.Duration
}

func NewS3Prober() *S3Prober {
	return &S3Prober{Timeout: 20 * time.Second}
}

var connectivityPayload = []byte("beeftv-connectivity-probe")

func (p *S3Prober) Probe(ctx context.Context, target Target) error {
	if strings.TrimSpace(target.Bucket) == "" {
		return fmt.Errorf("缺少 Bucket")
	}
	if normalizedProvider(target.Provider) == "cos" {
		return p.probeCOS(ctx, target)
	}
	client := newS3Client(target, p.Timeout)

	prefix := strings.Trim(strings.TrimSpace(target.Prefix), "/")
	objectKey := ".beeftv-connectivity-" + uuid.NewString()
	if prefix != "" {
		objectKey = prefix + "/" + objectKey
	}

	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(target.Bucket),
		Key:           aws.String(objectKey),
		Body:          bytes.NewReader(connectivityPayload),
		ContentLength: aws.Int64(int64(len(connectivityPayload))),
		ContentType:   aws.String("application/octet-stream"),
	}); err != nil {
		return fmt.Errorf("写入测试对象失败: %w", err)
	}
	// 无论后续步骤是否成功，都尽力清理测试对象，避免污染用户的 Bucket。
	defer func() {
		_, _ = client.DeleteObject(context.WithoutCancel(ctx), &s3.DeleteObjectInput{
			Bucket: aws.String(target.Bucket),
			Key:    aws.String(objectKey),
		})
	}()

	object, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(target.Bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("读取测试对象失败: %w", err)
	}
	defer object.Body.Close()
	body, err := io.ReadAll(io.LimitReader(object.Body, int64(len(connectivityPayload))+1))
	if err != nil {
		return fmt.Errorf("读取测试对象内容失败: %w", err)
	}
	if !bytes.Equal(body, connectivityPayload) {
		return fmt.Errorf("测试对象内容不一致，读取到 %d 字节", len(body))
	}
	return nil
}
