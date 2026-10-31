package manju

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const officialMasterRulePath = "short-drama-director/SKILL.md"

const officialRuleSourceCommit = "6f16955c57053d77ea0ffbdbbf3a7985a3e0080a"

type OfficialRuleFile struct {
	Path    string
	SHA256  string
	Content string
	Exact   bool
}

type OfficialRuleBundle struct {
	Stage        string
	MasterPath   string
	MasterSHA256 string
	Files        []OfficialRuleFile
	Content      string
	ContentHash  string
}

type OfficialRuleManifestEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type OfficialRuleTreeManifest struct {
	Version      string                      `json:"version"`
	SourceCommit string                      `json:"sourceCommit"`
	MasterPath   string                      `json:"masterPath"`
	MasterSHA256 string                      `json:"masterSha256"`
	TreeSHA256   string                      `json:"treeSha256"`
	FileCount    int                         `json:"fileCount"`
	Files        []OfficialRuleManifestEntry `json:"files"`
}

func (m OfficialRuleTreeManifest) JSON() (string, error) {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (r *ManjuSkillRuntime) OfficialRuleTreeManifest() (*OfficialRuleTreeManifest, error) {
	root, err := r.ensureRoot()
	if err != nil {
		return nil, err
	}
	files, err := r.EmbeddedFiles()
	if err != nil {
		return nil, err
	}
	manifest := &OfficialRuleTreeManifest{
		Version:      manjuSkillVersion,
		SourceCommit: officialRuleSourceCommit,
		MasterPath:   officialMasterRulePath,
		TreeSHA256:   manjuSkillContentHash,
		FileCount:    len(files),
		Files:        make([]OfficialRuleManifestEntry, 0, len(files)),
	}
	for _, rel := range files {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("读取嵌入规则文件 %s 失败: %w", rel, err)
		}
		sum := fmt.Sprintf("%x", sha256.Sum256(raw))
		manifest.Files = append(manifest.Files, OfficialRuleManifestEntry{Path: rel, SHA256: sum})
		if rel == officialMasterRulePath {
			manifest.MasterSHA256 = sum
		}
	}
	if manifest.MasterSHA256 == "" {
		return nil, fmt.Errorf("官方总控文件 %s 不在嵌入规则清单中", officialMasterRulePath)
	}
	return manifest, nil
}

var officialRuleCodeSpanPattern = regexp.MustCompile("`([^`]+)`")

func (r *ManjuSkillRuntime) ReadOfficialRuleFile(rel string) (*OfficialRuleFile, error) {
	rel = strings.TrimPrefix(strings.TrimSpace(rel), "./")
	clean := path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	if clean == "." || clean != rel || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return nil, fmt.Errorf("官方规则路径无效: %s", rel)
	}
	files, err := r.EmbeddedFiles()
	if err != nil {
		return nil, err
	}
	allowed := false
	for _, candidate := range files {
		if candidate == clean {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("官方规则文件不在只读清单中: %s", rel)
	}
	if err := gateManjuSkillPath(clean); err != nil {
		return nil, err
	}
	root, err := r.ensureRoot()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(clean)))
	if err != nil {
		return nil, fmt.Errorf("读取官方规则文件 %s 失败: %w", clean, err)
	}
	return &OfficialRuleFile{
		Path:    clean,
		SHA256:  fmt.Sprintf("%x", sha256.Sum256(raw)),
		Content: string(raw),
		Exact:   true,
	}, nil
}

func (r *ManjuSkillRuntime) OfficialRuleBundle(stage string, options ...ManjuSkillStageOptions) (*OfficialRuleBundle, error) {
	stage = strings.ToUpper(strings.TrimSpace(stage))
	if stage == "" {
		return nil, fmt.Errorf("官方规则阶段不能为空")
	}
	root, err := r.ensureRoot()
	if err != nil {
		return nil, err
	}
	masterRaw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(officialMasterRulePath)))
	if err != nil {
		return nil, fmt.Errorf("读取官方总控原文失败: %w", err)
	}
	master := string(masterRaw)
	paths, err := officialStageRulePaths(r, master, stage)
	if err != nil {
		return nil, err
	}
	if len(options) > 0 {
		conditionalPaths, err := officialConditionalRulePaths(r, stage, options[0])
		if err != nil {
			return nil, err
		}
		paths = mergeOfficialRulePaths(paths, conditionalPaths)
	}
	files, err := r.officialRuleFiles(root, paths)
	if err != nil {
		return nil, err
	}
	var content strings.Builder
	content.WriteString("===== V7.4.26 官方总控原文 =====\n")
	content.WriteString(master)
	content.WriteString("\n\n===== 官方总控强制读取原文 =====\n")
	var digest strings.Builder
	for _, file := range files {
		fmt.Fprintf(&content, "\n--- FILE: %s | SHA256: %s ---\n", file.Path, file.SHA256)
		content.WriteString(file.Content)
		if !strings.HasSuffix(file.Content, "\n") {
			content.WriteString("\n")
		}
		fmt.Fprintf(&digest, "%s:%s\n", file.Path, file.SHA256)
	}
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(digest.String())))
	return &OfficialRuleBundle{
		Stage:        stage,
		MasterPath:   officialMasterRulePath,
		MasterSHA256: fmt.Sprintf("%x", sha256.Sum256(masterRaw)),
		Files:        files,
		Content:      content.String(),
		ContentHash:  contentHash,
	}, nil
}

func officialConditionalRulePaths(runtime *ManjuSkillRuntime, stage string, options ManjuSkillStageOptions) ([]string, error) {
	candidates := make([]string, 0, 32)
	track := strings.TrimSpace(options.Track)
	source := strings.TrimSpace(options.Source)
	combat := strings.ToUpper(strings.TrimSpace(options.Combat))
	model := strings.ToLower(strings.TrimSpace(options.Model))

	candidates = append(candidates,
		"short-drama-director/references/project-bundle-contract.md",
		"short-drama-director/references/aspect-ratio-adaptation.md",
		"short-drama-director/references/style-bible/style-lock-protocol.md",
		"short-drama-director/references/style-bible/director-to-prompt-protocol.md",
		"short-drama-director/references/style-bible/director-param-library.md",
		"short-drama-director/references/style-bible/style-prefix-blocks.md",
		"short-drama-director/references/style-bible/director-style-catalog-74.md",
		"short-drama-director/references/style-bible/style-fit-and-combat-matrix.md",
		"short-drama-director/references/style-bible/style-archetype-taxonomy.md",
		"short-drama-director/references/style-bible/style-bible-spec.md",
	)
	if (source == "3" || source == "4") && (stage == "P0" || stage == "P1") {
		candidates = append(candidates,
			"sd-recon-engine/SKILL.md",
			"sd-recon-engine/references/source-ladder.md",
			"sd-recon-engine/references/recon-evidence.md",
			"sd-recon-engine/references/intel-card.md",
		)
	}
	if track == "2" {
		candidates = append(candidates,
			"sd-overseas-engine/SKILL.md",
			"sd-overseas-engine/references/language-contract.md",
			"sd-overseas-engine/references/north-american-english.md",
			"sd-overseas-engine/references/evidence-and-authority.md",
			"sd-overseas-engine/references/overseas-platform-and-audience.md",
		)
	}
	if track == "3" && stage != "P2" {
		candidates = append(candidates,
			"sd-jieshuo-engine/SKILL.md",
			"sd-jieshuo-engine/references/narration-vo-script.md",
			"sd-jieshuo-engine/references/history-visual-style.md",
			"sd-jieshuo-engine/references/explainer-formulas.md",
		)
	}
	if stage == "P1" && options.ExternalScript {
		candidates = append(candidates, "sd-script-engine/references/external-script-polish.md")
	}
	if (stage == "P3" || stage == "P4") && strings.TrimSpace(options.SeamMode) == SeamCreative {
		candidates = append(candidates, "short-drama-director/references/creative-transition-library.md")
	}
	switch stage {
	case "P1":
		candidates = append(candidates,
			"short-drama-director/references/acting-direction.md",
			"short-drama-director/references/screenplay-gate-engine.md",
			"sd-script-engine/references/source-registration.md",
			"sd-script-engine/references/episode-planning.md",
		)
	case "P2":
		candidates = append(candidates,
			"short-drama-director/references/production-ledger-handbook.md",
			"short-drama-director/references/character-lineage-and-sheets.md",
			"short-drama-director/references/lighting-and-antifake.md",
			"short-drama-director/references/comfyui-canvas-automation.md",
			"sd-asset-engine/references/three-list-extraction.md",
			"sd-asset-engine/references/continuity-and-crowd.md",
		)
	case "P3":
		candidates = append(candidates,
			"short-drama-director/references/camera-transitions-6types.md",
			"short-drama-director/references/camera-specs-15rules.md",
			"short-drama-director/references/spatial-topview-camera.md",
			"short-drama-director/references/cinematic-dramaturgy-rules.md",
			"short-drama-director/references/facs-micro-expression.md",
			"short-drama-director/references/facs-au-dictionary.md",
			"short-drama-director/references/facs-emotion-recipes.md",
			"short-drama-director/references/wenxi-micro-expression.md",
			"short-drama-director/references/vo-os-weaving.md",
			"short-drama-director/references/platform-safety-compliance-guide.md",
			"short-drama-director/references/comfyui-canvas-automation.md",
			"short-drama-director/references/spatial-reference-system-V3.md",
			"short-drama-director/references/lighting-and-antifake.md",
		)
	case "P4", "P5":
		candidates = append(candidates, "short-drama-director/references/delivery-pitfalls.md")
	}
	if stage == "P4" {
		candidates = append(candidates,
			"short-drama-director/references/dialogue-speed-check.md",
			"short-drama-director/references/seedance-render-engine.md",
			"short-drama-director/references/audiovisual-aesthetic-presets.md",
			"short-drama-director/references/dialect-voice-control-dictionary.md",
			"short-drama-director/references/vo-os-weaving.md",
			"short-drama-director/references/platform-safety-compliance-guide.md",
			"short-drama-director/references/comfyui-canvas-automation.md",
			"sd-feed-engine/references/engineering-language-cleanup.md",
			"sd-feed-engine/references/segment-continuity-strategy.md",
		)
	}
	if stage == "P5" {
		candidates = append(candidates,
			"short-drama-director/references/platform-safety-compliance-guide.md",
			"sd-qc-engine/references/editing-transition-gate.md",
			"sd-qc-engine/references/sound-design-template.md",
			"sd-qc-engine/references/post-music-sound-design.md",
			"sd-qc-engine/references/rule-taxonomy-and-verdict.md",
			"sd-qc-engine/references/stage-acceptance-table.md",
		)
	}
	if stage == "P3" || stage == "P4" {
		switch combat {
		case "R1":
			candidates = append(candidates,
				"sd-real-engine/SKILL.md",
				"sd-real-engine/references/r1-combat-core.md",
				"sd-real-engine/references/kinetic-chain-11.md",
				"sd-real-engine/references/defense-evasion-rules.md",
				"sd-real-engine/references/impact-physics.md",
				"sd-real-engine/references/camera-action-specs.md",
				"sd-real-engine/references/render-adapters.md",
				"sd-real-engine/references/multi-opponent-rules.md",
				"sd-real-engine/references/real-fight-techniques.md",
				"sd-real-engine/references/technique-combo-library.md",
				"sd-real-engine/references/martial-styles-extended.md",
				"sd-real-engine/references/storyboard-example.md",
			)
		case "R2":
			candidates = append(candidates,
				"sd-combat-engine/SKILL.md",
				"sd-combat-engine/references/combat-iron-laws.md",
				"sd-combat-engine/references/combat-construction.md",
				"sd-combat-engine/references/eight-sentence-masters.md",
				"sd-combat-engine/references/combat-intensity-guardrails.md",
				"sd-combat-engine/references/combat-rhythm-and-load-guardrails.md",
				"sd-combat-engine/references/tier-and-finisher-system.md",
				"sd-combat-engine/references/vfx-rendering-mandate.md",
				"sd-combat-engine/references/impact-frame-and-banlist.md",
				"sd-combat-engine/references/moves-library.md",
				"sd-combat-engine/references/spell-system.md",
				"sd-combat-engine/references/ultimate-array-library.md",
				"sd-combat-engine/references/presets-library.md",
				"sd-combat-engine/references/syntax-corpus.md",
				"sd-combat-engine/references/seedance-dual-engine.md",
				"sd-combat-engine/references/combat-cheatsheet.md",
			)
		case "R3":
			candidates = append(candidates,
				"short-drama-director/references/anime-ultimate-vfx-paradigm.md",
				"sd-xuanhuan-engine/SKILL.md",
				"sd-xuanhuan-engine/references/core-combat-laws.md",
				"sd-xuanhuan-engine/references/camera-specs-15modes.md",
				"sd-xuanhuan-engine/references/close-quarters-engine.md",
				"sd-xuanhuan-engine/references/xuanhuan-elements-vfx.md",
				"sd-xuanhuan-engine/references/grand-scale-domains.md",
				"sd-xuanhuan-engine/references/arsenal-moves-library.md",
				"sd-xuanhuan-engine/references/arsenal-parameter-library.md",
				"sd-xuanhuan-engine/references/character-scene-presets.md",
				"sd-xuanhuan-engine/references/epic-battlefield-protocols.md",
			)
		}
	}
	if model == "h3" && (stage == "P4" || stage == "P5") {
		candidates = append(candidates, "short-drama-director/references/h3-adapter.md")
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	index, err := newOfficialRulePathIndex(runtime)
	if err != nil {
		return nil, err
	}
	resolved := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		rulePath, ok := index.resolve(candidate)
		if !ok {
			return nil, fmt.Errorf("官方触发台账规则文件不存在: %s", candidate)
		}
		resolved = append(resolved, rulePath)
	}
	return mergeOfficialRulePaths(nil, resolved), nil
}

func mergeOfficialRulePaths(existing, additions []string) []string {
	seen := make(map[string]bool, len(existing)+len(additions))
	merged := make([]string, 0, len(existing)+len(additions))
	for _, group := range [][]string{existing, additions} {
		for _, rulePath := range group {
			if rulePath == "" || seen[rulePath] {
				continue
			}
			seen[rulePath] = true
			merged = append(merged, rulePath)
		}
	}
	return merged
}

func (r *ManjuSkillRuntime) officialRuleFiles(root string, paths []string) ([]OfficialRuleFile, error) {
	files := make([]OfficialRuleFile, 0, len(paths))
	for _, rel := range paths {
		if err := gateManjuSkillPath(rel); err != nil {
			return nil, err
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("读取官方总控指定规则 %s 失败: %w", rel, err)
		}
		files = append(files, OfficialRuleFile{
			Path:    rel,
			SHA256:  fmt.Sprintf("%x", sha256.Sum256(raw)),
			Content: string(raw),
			Exact:   true,
		})
	}
	return files, nil
}

func officialStageRulePaths(runtime *ManjuSkillRuntime, master, stage string) ([]string, error) {
	rows := officialStageTableRows(master, stage)
	if len(rows) == 0 {
		return nil, fmt.Errorf("官方总控未找到 %s 强制读取行", stage)
	}
	index, err := newOfficialRulePathIndex(runtime)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	paths := make([]string, 0)
	for _, row := range rows {
		cells := strings.Split(row, "|")
		if len(cells) < 4 {
			return nil, fmt.Errorf("官方总控 %s 强制读取行格式无效", stage)
		}
		for _, match := range officialRuleCodeSpanPattern.FindAllStringSubmatch(cells[3], -1) {
			candidate := strings.TrimSpace(match[1])
			if !looksLikeOfficialRulePath(candidate) {
				continue
			}
			resolved, ok := index.resolve(candidate)
			if !ok {
				return nil, fmt.Errorf("官方总控 %s 引用的规则文件不存在或路径歧义: %s", stage, candidate)
			}
			if !seen[resolved] {
				seen[resolved] = true
				paths = append(paths, resolved)
			}
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("官方总控 %s 强制读取行没有可解析规则文件", stage)
	}
	return paths, nil
}

func officialStageTableRows(master, stage string) []string {
	marker := "**" + stage + "**"
	var rows []string
	for _, line := range strings.Split(master, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		if stage == "P0" {
			cells := strings.Split(trimmed, "|")
			if len(cells) > 1 && strings.HasPrefix(strings.TrimSpace(cells[1]), "**P0") {
				rows = append(rows, trimmed)
			}
			continue
		}
		if !strings.Contains(trimmed, marker) {
			continue
		}
		cells := strings.Split(trimmed, "|")
		if len(cells) > 1 && strings.Contains(cells[1], marker) {
			rows = append(rows, trimmed)
		}
	}
	return rows
}

type officialRulePathIndex struct {
	files    map[string]bool
	basename map[string][]string
}

func newOfficialRulePathIndex(runtime *ManjuSkillRuntime) (*officialRulePathIndex, error) {
	files, err := runtime.EmbeddedFiles()
	if err != nil {
		return nil, err
	}
	index := &officialRulePathIndex{
		files:    make(map[string]bool, len(files)),
		basename: make(map[string][]string),
	}
	for _, rel := range files {
		index.files[rel] = true
		base := path.Base(rel)
		index.basename[base] = append(index.basename[base], rel)
	}
	for base := range index.basename {
		sort.Strings(index.basename[base])
	}
	return index, nil
}

func (i *officialRulePathIndex) resolve(candidate string) (string, bool) {
	candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "./")
	for _, rel := range []string{
		candidate,
		"short-drama-director/" + candidate,
		"short-drama-director/references/" + candidate,
	} {
		if i.files[rel] {
			return rel, true
		}
	}
	if strings.Contains(candidate, "/") {
		return "", false
	}
	matches := i.basename[path.Base(candidate)]
	if len(matches) != 1 {
		return "", false
	}
	return matches[0], true
}

func looksLikeOfficialRulePath(candidate string) bool {
	if candidate == "" || strings.ContainsAny(candidate, " \t\r\n") || strings.HasPrefix(candidate, "-") {
		return false
	}
	return strings.HasSuffix(candidate, ".md") ||
		strings.HasSuffix(candidate, ".py") ||
		strings.Contains(candidate, "/")
}
