package storageconfig

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// stubS3 是最小的进程内 S3 兼容端点：只实现 Put/Get/Head/Delete，
// 不校验签名。它让读写后端能在没有真实云存储的情况下端到端验证。
type stubS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	lastKey string
}

func newStubS3(t *testing.T) (*httptest.Server, *stubS3) {
	t.Helper()
	stub := &stubS3{objects: map[string][]byte{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		key := parts[1]
		stub.mu.Lock()
		defer stub.mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "read body", http.StatusBadRequest)
				return
			}
			stub.objects[key] = body
			stub.lastKey = key
			w.Header().Set("ETag", `"stub"`)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			body, ok := stub.objects[key]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", itoa(len(body)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		case http.MethodHead:
			body, ok := stub.objects[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", itoa(len(body)))
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			delete(stub.objects, key)
			stub.lastKey = key
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unsupported", http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	return server, stub
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 12)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

func TestS3BlobStoreRoundTrip(t *testing.T) {
	server, stub := newStubS3(t)
	store, err := NewS3BlobStore(Target{
		Provider: "s3", Endpoint: server.URL, Region: "us-east-1", Bucket: "beeftv",
		AccessKeyID: "ak", SecretAccessKey: "sk", Prefix: "uploads", PathStyle: true,
	}, t.TempDir())
	if err != nil {
		t.Fatalf("构造读写后端失败: %v", err)
	}

	if err := store.Write("a/b.txt", strings.NewReader("hello-object")); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	stub.mu.Lock()
	gotKey := stub.lastKey
	stub.mu.Unlock()
	if gotKey != "uploads/a/b.txt" {
		t.Fatalf("路径前缀未生效，得到 %q", gotKey)
	}

	if err := store.Exists("a/b.txt"); err != nil {
		t.Fatalf("对象应存在: %v", err)
	}
	file, err := store.Open("a/b.txt")
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	defer file.Close()
	// Open 必须返回可 Seek 且可 Stat 的句柄，资产服务的 Range 逻辑依赖这一点。
	if _, err := file.Seek(6, io.SeekStart); err != nil {
		t.Fatalf("返回句柄必须可 Seek: %v", err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatalf("返回句柄必须可 Stat: %v", err)
	}
	if info.Size() != int64(len("hello-object")) {
		t.Fatalf("对象大小不符，得到 %d", info.Size())
	}
	rest, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("读取内容失败: %v", err)
	}
	if string(rest) != "object" {
		t.Fatalf("Seek 后内容不符，得到 %q", string(rest))
	}

	if err := store.Delete("a/b.txt"); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if err := store.Exists("a/b.txt"); err == nil {
		t.Fatal("删除后对象不应存在")
	}
}

func TestNewS3BlobStoreRejectsIncompleteTarget(t *testing.T) {
	if _, err := NewS3BlobStore(Target{Endpoint: "https://oss.example.com"}, t.TempDir()); err == nil {
		t.Fatal("缺少 Bucket 必须报错")
	}
	if _, err := NewS3BlobStore(Target{Bucket: "beeftv"}, t.TempDir()); err == nil {
		t.Fatal("缺少接入点与 Region 必须报错")
	}
}
