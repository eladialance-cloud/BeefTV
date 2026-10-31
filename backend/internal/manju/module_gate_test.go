package manju

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestModuleGateDisabledFromFile(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "modules.json")
	if err := os.WriteFile(statePath, []byte(`{"sd-feed-engine": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MANJU_MODULES_FILE", statePath)
	gateCache.mu.Lock()
	gateCache.state = nil
	gateCache.mu.Unlock()
	t.Cleanup(func() {
		gateCache.mu.Lock()
		gateCache.state = nil
		gateCache.mu.Unlock()
	})

	if moduleDisabled("sd-script") {
		t.Fatal("sd-script 未被停用，不应报告已禁用")
	}
	if !moduleDisabled("sd-feed-engine") {
		t.Fatal("sd-feed-engine 已在状态文件中停用，应报告禁用")
	}

	if err := gateManjuSkillPath("sd-feed-engine/SKILL.md"); !errors.Is(err, ErrModuleDisabled) {
		t.Fatalf("停用模块的规则读取应返回 ErrModuleDisabled，got %v", err)
	}
	if err := gateManjuSkillPath("sd-script/SKILL.md"); err != nil {
		t.Fatalf("未停用模块的规则读取不应被拦截，got %v", err)
	}
	if err := gateManjuSkillPath("short-drama-director/SKILL.md"); err != nil {
		t.Fatalf("总控不应被闸门拦截，got %v", err)
	}
}

func TestModuleGateNoConfigMeansAllEnabled(t *testing.T) {
	t.Setenv("MANJU_MODULES_FILE", "")
	gateCache.mu.Lock()
	gateCache.state = nil
	gateCache.mu.Unlock()
	t.Cleanup(func() {
		gateCache.mu.Lock()
		gateCache.state = nil
		gateCache.mu.Unlock()
	})

	if moduleDisabled("sd-feed-engine") {
		t.Fatal("未配置 MANJU_MODULES_FILE 时所有模块应默认启用")
	}
	if err := gateManjuSkillPath("sd-feed-engine/SKILL.md"); err != nil {
		t.Fatalf("未配置状态文件时不应拦截，got %v", err)
	}
}

func TestPathFirstSegment(t *testing.T) {
	cases := map[string]string{
		"sd-feed-engine/SKILL.md": "sd-feed-engine",
		"sd-qc/scripts/check.py":  "sd-qc",
		"README.md":     "", // 顶层散文件不属于任何模块，不拦截
		"../outside.md": "",
		"":              "",
	}
	for input, want := range cases {
		if got := pathFirstSegment(input); got != want {
			t.Errorf("pathFirstSegment(%q) = %q, want %q", input, got, want)
		}
	}
}
