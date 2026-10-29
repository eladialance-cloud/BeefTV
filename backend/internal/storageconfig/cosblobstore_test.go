package storageconfig

import "testing"

func TestCOSRegionFromEndpoint(t *testing.T) {
	cases := map[string]string{
		"https://cos.ap-shanghai.myqcloud.com":                         "ap-shanghai",
		"https://shentongapi-1433476827.cos.ap-guangzhou.myqcloud.com": "ap-guangzhou",
		"https://cos.ap-beijing.tencentcos.cn":                         "ap-beijing",
		"https://example.com":                                          "",
	}
	for endpoint, want := range cases {
		if got := cosRegionFromEndpoint(endpoint); got != want {
			t.Fatalf("cosRegionFromEndpoint(%q) = %q, want %q", endpoint, got, want)
		}
	}
}

func TestNewCOSBlobStoreRequiresBucketAndRegion(t *testing.T) {
	if _, err := NewCOSBlobStore(Target{Provider: "cos", Bucket: "demo-1250000000"}, t.TempDir()); err == nil {
		t.Fatal("missing region must fail")
	}
	if _, err := NewCOSBlobStore(Target{Provider: "cos", Region: "ap-shanghai"}, t.TempDir()); err == nil {
		t.Fatal("missing bucket must fail")
	}
}
