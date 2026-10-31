package manju

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

// ErrModuleDisabled marks failures caused by an engine that an administrator
// turned off through the manju admin surface (MANJU_MODULES_FILE).
var ErrModuleDisabled = errors.New("漫剧引擎模块已被后台停用")

// manjuModulesStatePath mirrors hostedapi's MANJU_MODULES_FILE convention: a
// JSON object {"<module-id>": true} where true means disabled.
func manjuModulesStatePath() string { return os.Getenv("MANJU_MODULES_FILE") }

// moduleGateCache caches the disabled-module map keyed on file identity so
// pipeline stages can gate every rule read without hammering the disk.
type moduleGateCache struct {
	mu      sync.Mutex
	path    string
	modTime time.Time
	size    int64
	state   map[string]bool
}

var gateCache moduleGateCache

// moduleDisabled reports whether the given engine module has been disabled by
// an administrator. Missing or unconfigured state files mean everything is on.
func moduleDisabled(moduleID string) bool {
	p := manjuModulesStatePath()
	if strings.TrimSpace(p) == "" {
		return false
	}
	gateCache.mu.Lock()
	defer gateCache.mu.Unlock()
	info, err := os.Stat(p)
	if err != nil {
		gateCache.state = nil
		return false
	}
	if gateCache.state == nil || gateCache.path != p || !info.ModTime().Equal(gateCache.modTime) || info.Size() != gateCache.size {
		state := map[string]bool{}
		if data, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(data, &state)
		}
		gateCache.path = p
		gateCache.modTime = info.ModTime()
		gateCache.size = info.Size()
		gateCache.state = state
	}
	return gateCache.state[moduleID]
}

// gateManjuSkillPath enforces the admin module toggles on skill-file access.
// Skill paths are "<module-id>/..." (for example "sd-feed-engine/SKILL.md");
// the first segment is the module id. The master controller file stays
// ungated: disabling it would brick the whole pipeline by design, and the
// admin surface describes it as such.
func gateManjuSkillPath(relPath string) error {
	moduleID := pathFirstSegment(relPath)
	if moduleID == "" || moduleID == "short-drama-director" {
		return nil
	}
	if moduleDisabled(moduleID) {
		return fmt.Errorf("%w: %s 已停用，无法读取其规则 %s；请在后台重新启用该模块", ErrModuleDisabled, moduleID, relPath)
	}
	return nil
}

// pathFirstSegment returns the first slash-separated segment of a cleaned
// skill-relative path, or "" when the path has none.
func pathFirstSegment(relPath string) string {
	clean := path.Clean(strings.ReplaceAll(strings.TrimSpace(relPath), "\\", "/"))
	if clean == "." || clean == "/" || strings.HasPrefix(clean, "../") {
		return ""
	}
	if i := strings.IndexByte(clean, '/'); i > 0 {
		return clean[:i]
	}
	return ""
}
