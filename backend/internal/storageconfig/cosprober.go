package storageconfig

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
)

func (p *S3Prober) probeCOS(ctx context.Context, target Target) error {
	client, err := newCOSClient(target, p.Timeout)
	if err != nil {
		return err
	}
	prefix := strings.Trim(strings.TrimSpace(target.Prefix), "/")
	objectKey := ".beeftv-connectivity-" + uuid.NewString()
	if prefix != "" {
		objectKey = prefix + "/" + objectKey
	}
	if _, err := client.Object.Put(ctx, objectKey, bytes.NewReader(connectivityPayload), nil); err != nil {
		return fmt.Errorf("写入腾讯 COS 测试对象失败: %w", err)
	}
	defer func() {
		_, _ = client.Object.Delete(context.WithoutCancel(ctx), objectKey, nil)
	}()
	response, err := client.Object.Get(ctx, objectKey, nil)
	if err != nil {
		return fmt.Errorf("读取腾讯 COS 测试对象失败: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, int64(len(connectivityPayload))+1))
	if err != nil {
		return fmt.Errorf("读取腾讯 COS 测试对象内容失败: %w", err)
	}
	if !bytes.Equal(body, connectivityPayload) {
		return fmt.Errorf("腾讯 COS 测试对象内容不一致，读取到 %d 字节", len(body))
	}
	return nil
}
