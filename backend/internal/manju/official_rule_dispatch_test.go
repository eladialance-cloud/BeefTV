package manju

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestOfficialRuleBundleDispatchesFromMasterControlVerbatim(t *testing.T) {
	bundle, err := defaultManjuSkillRuntime.OfficialRuleBundle("P4")
	if err != nil {
		t.Fatal(err)
	}
	root, err := defaultManjuSkillRuntime.ensureRoot()
	if err != nil {
		t.Fatal(err)
	}
	masterPath := filepath.Join(root, "short-drama-director", "SKILL.md")
	master, err := os.ReadFile(masterPath)
	if err != nil {
		t.Fatal(err)
	}
	masterHash := fmt.Sprintf("%x", sha256.Sum256(master))
	if bundle.MasterSHA256 != masterHash {
		t.Fatalf("master hash = %s, want %s", bundle.MasterSHA256, masterHash)
	}
	masterTail := lastNonEmptyTextLine(string(master))
	if masterTail == "" || !strings.Contains(bundle.Content, masterTail) {
		t.Fatalf("official bundle omitted the master control tail %q", masterTail)
	}
	if strings.Contains(bundle.Content, "阶段完整章节原文分页") {
		t.Fatal("official bundle still depends on slice_context pagination")
	}
	if strings.Contains(bundle.Content, "[已截断]") {
		t.Fatal("official bundle must never contain a truncation marker")
	}
	for _, required := range []string{
		"sd-feed-engine/SKILL.md",
		"sd-feed-engine/references/feed-mode-and-gates.md",
		"sd-feed-engine/references/asset-mapping-and-rules.md",
		"short-drama-director/references/model-adapters.md",
		"short-drama-director/references/segment-splicing.md",
		"short-drama-director/references/prompt-feeding-checklist.md",
	} {
		var matched *OfficialRuleFile
		for index := range bundle.Files {
			if bundle.Files[index].Path == required {
				matched = &bundle.Files[index]
				break
			}
		}
		if matched == nil {
			t.Fatalf("official P4 dispatch omitted %s", required)
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(required)))
		if err != nil {
			t.Fatal(err)
		}
		wantHash := fmt.Sprintf("%x", sha256.Sum256(raw))
		if matched.SHA256 != wantHash {
			t.Fatalf("%s hash = %s, want %s", required, matched.SHA256, wantHash)
		}
		if tail := lastNonEmptyTextLine(string(raw)); tail == "" || !strings.Contains(bundle.Content, tail) {
			t.Fatalf("%s content was not included verbatim", required)
		}
	}
}

func TestStageContextUsesOfficialMasterDispatch(t *testing.T) {
	options := ManjuSkillStageOptions{
		Stage: "P4", Track: "1", Source: "1", Combat: "none", Model: "2.5", Mode: "M1",
	}
	bundle, err := defaultManjuSkillRuntime.OfficialRuleBundle("P4", options)
	if err != nil {
		t.Fatal(err)
	}
	contextText, err := defaultManjuSkillRuntime.StageContext(options)
	if err != nil {
		t.Fatal(err)
	}
	if contextText != bundle.Content {
		t.Fatal("StageContext did not return the official master dispatch verbatim")
	}
}

func TestStageRuleDispatchLoadsOfficialConditionalRules(t *testing.T) {
	cases := []struct {
		name     string
		options  ManjuSkillStageOptions
		required []string
	}{
		{
			name: "overseas P1",
			options: ManjuSkillStageOptions{
				Stage: "P1", Track: "2", Source: "1", Combat: "none", Model: "2.5", Mode: "M1",
			},
			required: []string{
				"sd-overseas-engine/SKILL.md",
				"sd-overseas-engine/references/language-contract.md",
				"sd-overseas-engine/references/north-american-english.md",
			},
		},
		{
			name: "explainer P4",
			options: ManjuSkillStageOptions{
				Stage: "P4", Track: "3", Source: "1", Combat: "none", Model: "2.5", Mode: "M1",
			},
			required: []string{
				"sd-jieshuo-engine/SKILL.md",
				"sd-jieshuo-engine/references/narration-vo-script.md",
				"sd-jieshuo-engine/references/history-visual-style.md",
				"sd-jieshuo-engine/references/explainer-formulas.md",
			},
		},
		{
			name: "benchmark P1",
			options: ManjuSkillStageOptions{
				Stage: "P1", Track: "1", Source: "3", Combat: "none", Model: "2.5", Mode: "M1",
			},
			required: []string{
				"sd-recon-engine/SKILL.md",
				"sd-recon-engine/references/source-ladder.md",
				"sd-recon-engine/references/recon-evidence.md",
				"sd-recon-engine/references/intel-card.md",
			},
		},
		{
			name: "R2 P3",
			options: ManjuSkillStageOptions{
				Stage: "P3", Track: "1", Source: "1", Combat: "R2", Model: "2.5", Mode: "M1",
			},
			required: []string{
				"sd-combat-engine/SKILL.md",
				"sd-combat-engine/references/combat-iron-laws.md",
				"sd-combat-engine/references/combat-construction.md",
				"sd-combat-engine/references/eight-sentence-masters.md",
				"sd-combat-engine/references/combat-intensity-guardrails.md",
				"sd-combat-engine/references/combat-rhythm-and-load-guardrails.md",
				"sd-combat-engine/references/tier-and-finisher-system.md",
			},
		},
		{
			name: "H3 P4",
			options: ManjuSkillStageOptions{
				Stage: "P4", Track: "1", Source: "1", Combat: "none", Model: "h3", Mode: "M1",
			},
			required: []string{
				"short-drama-director/references/h3-adapter.md",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle, err := defaultManjuSkillRuntime.StageRuleDispatch(tc.options)
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range tc.required {
				if !officialRuleBundleHasFile(bundle, required) {
					t.Fatalf("official conditional dispatch omitted %s", required)
				}
			}
		})
	}
}

func TestStageRuleDispatchLoadsAllOfficialP0Rows(t *testing.T) {
	bundle, err := defaultManjuSkillRuntime.StageRuleDispatch(ManjuSkillStageOptions{
		Stage: "P0", Track: "1", Source: "1", Combat: "none", Model: "2.5", Mode: "M1",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"short-drama-director/references/track-routing.md",
		"short-drama-director/references/asset-first-pipeline.md",
		"short-drama-director/references/asset-delivery-modes.md",
		"sd-decision-engine/SKILL.md",
		"sd-decision-engine/references/context-mapping.md",
		"sd-decision-engine/references/emotion-anchor-table.md",
		"sd-decision-engine/references/forbidden-zone-whitelist.md",
	} {
		if !officialRuleBundleHasFile(bundle, required) {
			t.Fatalf("official P0 dispatch omitted %s", required)
		}
	}
}

func TestStageRuleDispatchLoadsStageConditionalLedgerRules(t *testing.T) {
	cases := []struct {
		stage    string
		required []string
	}{
		{
			stage: "P1",
			required: []string{
				"short-drama-director/references/acting-direction.md",
				"short-drama-director/references/screenplay-gate-engine.md",
			},
		},
		{
			stage: "P2",
			required: []string{
				"short-drama-director/references/production-ledger-handbook.md",
				"short-drama-director/references/character-lineage-and-sheets.md",
			},
		},
		{
			stage: "P3",
			required: []string{
				"short-drama-director/references/camera-transitions-6types.md",
				"short-drama-director/references/camera-specs-15rules.md",
				"short-drama-director/references/spatial-topview-camera.md",
				"short-drama-director/references/cinematic-dramaturgy-rules.md",
				"short-drama-director/references/facs-micro-expression.md",
				"short-drama-director/references/facs-au-dictionary.md",
				"short-drama-director/references/facs-emotion-recipes.md",
				"short-drama-director/references/wenxi-micro-expression.md",
			},
		},
		{
			stage: "P4",
			required: []string{
				"short-drama-director/references/dialogue-speed-check.md",
				"short-drama-director/references/seedance-render-engine.md",
				"short-drama-director/references/delivery-pitfalls.md",
				"sd-feed-engine/references/engineering-language-cleanup.md",
				"sd-feed-engine/references/segment-continuity-strategy.md",
			},
		},
		{
			stage: "P5",
			required: []string{
				"short-drama-director/references/delivery-pitfalls.md",
				"sd-qc-engine/references/editing-transition-gate.md",
				"sd-qc-engine/references/sound-design-template.md",
				"sd-qc-engine/references/post-music-sound-design.md",
				"sd-qc-engine/references/rule-taxonomy-and-verdict.md",
				"sd-qc-engine/references/stage-acceptance-table.md",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.stage, func(t *testing.T) {
			bundle, err := defaultManjuSkillRuntime.StageRuleDispatch(ManjuSkillStageOptions{
				Stage: tc.stage, Track: "1", Source: "1", Combat: "none", Model: "2.5", Mode: "M1",
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range tc.required {
				if !officialRuleBundleHasFile(bundle, required) {
					t.Fatalf("official %s conditional ledger omitted %s", tc.stage, required)
				}
			}
		})
	}
}

func TestStageRuleDispatchLoadsProjectLockAndCrossStageRules(t *testing.T) {
	cases := []struct {
		stage    string
		required []string
	}{
		{
			stage: "P0",
			required: []string{
				"short-drama-director/references/aspect-ratio-adaptation.md",
				"short-drama-director/references/project-bundle-contract.md",
				"short-drama-director/references/style-bible/style-lock-protocol.md",
				"short-drama-director/references/style-bible/director-to-prompt-protocol.md",
				"short-drama-director/references/style-bible/director-param-library.md",
				"short-drama-director/references/style-bible/style-prefix-blocks.md",
				"short-drama-director/references/style-bible/director-style-catalog-74.md",
				"short-drama-director/references/style-bible/style-fit-and-combat-matrix.md",
				"short-drama-director/references/style-bible/style-archetype-taxonomy.md",
				"short-drama-director/references/style-bible/style-bible-spec.md",
			},
		},
		{
			stage: "P3",
			required: []string{
				"short-drama-director/references/vo-os-weaving.md",
				"short-drama-director/references/platform-safety-compliance-guide.md",
				"short-drama-director/references/comfyui-canvas-automation.md",
				"short-drama-director/references/spatial-reference-system-V3.md",
			},
		},
		{
			stage: "P4",
			required: []string{
				"short-drama-director/references/vo-os-weaving.md",
				"short-drama-director/references/platform-safety-compliance-guide.md",
				"short-drama-director/references/comfyui-canvas-automation.md",
			},
		},
		{
			stage: "P5",
			required: []string{
				"short-drama-director/references/platform-safety-compliance-guide.md",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.stage, func(t *testing.T) {
			bundle, err := defaultManjuSkillRuntime.StageRuleDispatch(ManjuSkillStageOptions{
				Stage: tc.stage, Track: "1", Source: "1", Combat: "none", Model: "2.5", Mode: "M1",
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range tc.required {
				if !officialRuleBundleHasFile(bundle, required) {
					t.Fatalf("official %s lock/cross-stage dispatch omitted %s", tc.stage, required)
				}
			}
		})
	}
}

func TestOfficialStageRulePathsRejectsAnyUnresolvedMasterReference(t *testing.T) {
	master := strings.Join([]string{
		"| 阶段 | 触发时机 | **必读文件** | 硬产出 |",
		"|---|---|---|---|",
		"| **P3** | test | `short-drama-director/references/acting-direction.md` / `missing-official-rule.md` | test |",
	}, "\n")
	if _, err := officialStageRulePaths(defaultManjuSkillRuntime, master, "P3"); err == nil {
		t.Fatal("stage dispatch silently accepted an unresolved official master reference")
	}
}

func TestStageRuleDispatchLoadsSelectedLockRules(t *testing.T) {
	cases := []struct {
		name     string
		options  ManjuSkillStageOptions
		required []string
	}{
		{
			name: "creative transition P3",
			options: ManjuSkillStageOptions{
				Stage: "P3", Track: "1", Source: "1", Combat: "none", Model: "2.5", Mode: "M1",
				SeamMode: SeamCreative,
			},
			required: []string{
				"short-drama-director/references/creative-transition-library.md",
			},
		},
		{
			name: "external script polish P1",
			options: ManjuSkillStageOptions{
				Stage: "P1", Track: "1", Source: "2", Combat: "none", Model: "2.5", Mode: "M1",
				ExternalScript: true,
			},
			required: []string{
				"sd-script-engine/references/external-script-polish.md",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle, err := defaultManjuSkillRuntime.StageRuleDispatch(tc.options)
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range tc.required {
				if !officialRuleBundleHasFile(bundle, required) {
					t.Fatalf("official selected-lock dispatch omitted %s", required)
				}
			}
		})
	}
}

func TestOfficialConditionalLedgerEveryRuleIsReachable(t *testing.T) {
	root, err := defaultManjuSkillRuntime.ensureRoot()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "short-drama-director", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	master := string(raw)
	start := strings.Index(master, "### ★ 按需规则文件 · 触发条件台账")
	if start < 0 {
		t.Fatal("official conditional rule ledger heading not found")
	}
	end := strings.Index(master[start:], "### 子技能路由表")
	if end < 0 {
		t.Fatal("official conditional rule ledger end marker not found")
	}
	ledger := master[start : start+end]
	index, err := newOfficialRulePathIndex(defaultManjuSkillRuntime)
	if err != nil {
		t.Fatal(err)
	}
	required := make([]string, 0, 64)
	seenRequired := make(map[string]bool)
	for _, line := range strings.Split(ledger, "\n") {
		for _, match := range officialRuleCodeSpanPattern.FindAllStringSubmatch(line, -1) {
			candidate := strings.TrimSpace(match[1])
			if strings.Contains(candidate, "{") ||
				!looksLikeOfficialRulePath(candidate) ||
				!strings.HasSuffix(candidate, ".md") {
				continue
			}
			resolved, ok := index.resolve(candidate)
			if !ok {
				t.Fatalf("conditional ledger contains unresolved rule path %s", candidate)
			}
			if strings.Contains(resolved, "/examples/") || seenRequired[resolved] {
				continue
			}
			seenRequired[resolved] = true
			required = append(required, resolved)
		}
	}
	if len(required) == 0 {
		t.Fatal("conditional ledger rule inventory is empty")
	}

	scenarios := []ManjuSkillStageOptions{
		{Stage: "P0", Track: "1", Source: "1", Combat: "none", Model: "2.5", Mode: "M1"},
		{Stage: "P1", Track: "1", Source: "1", Combat: "none", Model: "2.5", Mode: "M1"},
		{Stage: "P1", Track: "2", Source: "1", Combat: "none", Model: "2.5", Mode: "M1"},
		{Stage: "P1", Track: "1", Source: "3", Combat: "none", Model: "2.5", Mode: "M1"},
		{Stage: "P1", Track: "1", Source: "2", Combat: "none", Model: "2.5", Mode: "M1", ExternalScript: true},
		{Stage: "P2", Track: "1", Source: "1", Combat: "none", Model: "2.5", Mode: "M1"},
		{Stage: "P3", Track: "1", Source: "1", Combat: "R1", Model: "2.5", Mode: "M1"},
		{Stage: "P3", Track: "1", Source: "1", Combat: "R2", Model: "2.5", Mode: "M1"},
		{Stage: "P3", Track: "1", Source: "1", Combat: "R3", Model: "2.5", Mode: "M1"},
		{Stage: "P3", Track: "1", Source: "1", Combat: "none", Model: "2.5", Mode: "M1", SeamMode: SeamCreative},
		{Stage: "P4", Track: "1", Source: "1", Combat: "R1", Model: "2.5", Mode: "M1"},
		{Stage: "P4", Track: "1", Source: "1", Combat: "R2", Model: "2.5", Mode: "M1"},
		{Stage: "P4", Track: "1", Source: "1", Combat: "R3", Model: "2.5", Mode: "M1"},
		{Stage: "P4", Track: "3", Source: "1", Combat: "none", Model: "h3", Mode: "M1"},
		{Stage: "P5", Track: "3", Source: "1", Combat: "none", Model: "h3", Mode: "M1"},
	}
	covered := make(map[string]bool)
	for _, options := range scenarios {
		stagePaths, err := officialStageRulePaths(defaultManjuSkillRuntime, master, options.Stage)
		if err != nil {
			t.Fatal(err)
		}
		conditionalPaths, err := officialConditionalRulePaths(defaultManjuSkillRuntime, options.Stage, options)
		if err != nil {
			t.Fatal(err)
		}
		for _, rulePath := range mergeOfficialRulePaths(stagePaths, conditionalPaths) {
			covered[rulePath] = true
		}
	}
	covered["sd-storyboard-engine/references/p45-previs-storyboard.md"] = true

	missing := make([]string, 0)
	for _, rulePath := range required {
		if !covered[rulePath] {
			missing = append(missing, rulePath)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("official conditional ledger has unreachable rules: %s", strings.Join(missing, ", "))
	}
}

func TestProductionRuleCombatUsesLockedProjectArtifacts(t *testing.T) {
	scriptArtifacts := []model.ManjuScriptArtifact{{
		Kind: "SCRIPT.md", Content: "场景动作：主角御剑破阵，法相降临。",
	}}
	if got := productionRuleCombat(scriptArtifacts, nil, nil, ""); got != "R3" {
		t.Fatalf("locked script combat tier = %s, want R3", got)
	}
	assetArtifacts := []model.ManjuAssetArtifact{{
		Kind: "ASSET_COMPILE.md", Content: "动作：近身格斗，警匪追车。",
	}}
	if got := productionRuleCombat(nil, assetArtifacts, nil, ""); got != "R1" {
		t.Fatalf("locked asset combat tier = %s, want R1", got)
	}
	productionArtifacts := []model.ManjuProductionArtifact{{
		Kind: "DIRECTOR_STORYBOARD.md", Content: "镜行：刀剑交锋，轻功腾挪。",
	}}
	if got := productionRuleCombat(nil, nil, productionArtifacts, ""); got != "R2" {
		t.Fatalf("locked P3 combat tier = %s, want R2", got)
	}
	if got := productionRuleCombat(nil, nil, nil, "本镜使用法术对决"); got != "R3" {
		t.Fatalf("instruction combat tier = %s, want R3", got)
	}
}

func officialRuleBundleHasFile(bundle *OfficialRuleBundle, rel string) bool {
	for _, file := range bundle.Files {
		if file.Path == rel && file.Exact {
			return true
		}
	}
	return false
}

func TestOfficialRuleTreeManifestCoversEveryEmbeddedFileAndHash(t *testing.T) {
	manifest, err := defaultManjuSkillRuntime.OfficialRuleTreeManifest()
	if err != nil {
		t.Fatal(err)
	}
	files, err := defaultManjuSkillRuntime.EmbeddedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.FileCount != len(files) {
		t.Fatalf("manifest file count = %d, want %d", manifest.FileCount, len(files))
	}
	if len(manifest.Files) != len(files) {
		t.Fatalf("manifest entries = %d, want %d", len(manifest.Files), len(files))
	}
	root, err := defaultManjuSkillRuntime.ensureRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range manifest.Files {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(entry.Path)))
		if err != nil {
			t.Fatal(err)
		}
		want := fmt.Sprintf("%x", sha256.Sum256(raw))
		if entry.SHA256 != want {
			t.Fatalf("%s hash = %s, want %s", entry.Path, entry.SHA256, want)
		}
	}
	if manifest.MasterSHA256 == "" || manifest.TreeSHA256 == "" || manifest.SourceCommit == "" {
		t.Fatal("manifest must include master, tree and source commit identities")
	}
}

func TestBuildManjuRuleAuditRecordsPinsExactSources(t *testing.T) {
	manifest, err := defaultManjuSkillRuntime.OfficialRuleTreeManifest()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := buildManjuRuleSnapshot("project-1", 7, "user-1", manifest)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.TreeSHA256 != manifest.TreeSHA256 ||
		snapshot.MasterSHA256 != manifest.MasterSHA256 ||
		snapshot.ManifestJSON == "" {
		t.Fatalf("snapshot did not pin the complete rule identity: %+v", snapshot)
	}

	bundle, err := defaultManjuSkillRuntime.OfficialRuleBundle("P4")
	if err != nil {
		t.Fatal(err)
	}
	receipts := buildManjuRuleReadReceipts(snapshot, "run-1", "stage-1", "P4", "request-1", bundle.Files)
	if len(receipts) != len(bundle.Files) {
		t.Fatalf("receipts = %d, want %d", len(receipts), len(bundle.Files))
	}
	for _, receipt := range receipts {
		if receipt.SnapshotID != snapshot.ID || receipt.RulePath == "" ||
			receipt.ContentSHA256 == "" || receipt.EndByte <= receipt.StartByte {
			t.Fatalf("invalid rule read receipt: %+v", receipt)
		}
	}

	exchange, err := buildManjuModelExchange(
		"project-1", "run-1", "stage-1", "P4", "request-1",
		`{"prompt":"完整原文"}`, `{"contentHash":"request-hash"}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if exchange.Status != model.ManjuModelExchangeStatusQueued ||
		exchange.RequestJSON != `{"prompt":"完整原文"}` ||
		exchange.RequestSHA256 == "" {
		t.Fatalf("invalid model exchange: %+v", exchange)
	}
}

func TestOfficialRuleDispatchCoversEveryGenerationStage(t *testing.T) {
	for _, stage := range []string{"P0", "P1", "P2", "P3", "P4", "P5"} {
		t.Run(stage, func(t *testing.T) {
			bundle, err := defaultManjuSkillRuntime.OfficialRuleBundle(stage)
			if err != nil {
				t.Fatal(err)
			}
			if len(bundle.Files) == 0 {
				t.Fatalf("%s has no official referenced rule files", stage)
			}
			if !strings.Contains(bundle.Content, "===== V7.4.26 官方总控原文 =====") {
				t.Fatalf("%s omitted the master control", stage)
			}
			if strings.Contains(bundle.Content, "阶段完整章节原文分页") ||
				strings.Contains(bundle.Content, "[已截断]") {
				t.Fatalf("%s still contains sliced or truncated rule content", stage)
			}
		})
	}
}

func TestReadOfficialRuleFileReturnsExactEmbeddedBytesAndRejectsTraversal(t *testing.T) {
	file, err := defaultManjuSkillRuntime.ReadOfficialRuleFile("sd-feed-engine/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	root, err := defaultManjuSkillRuntime.ensureRoot()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "sd-feed-engine", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if file.Content != string(raw) ||
		file.SHA256 != fmt.Sprintf("%x", sha256.Sum256(raw)) ||
		!file.Exact {
		t.Fatal("official rule reader did not return the exact embedded file")
	}
	if _, err := defaultManjuSkillRuntime.ReadOfficialRuleFile("../outside.md"); err == nil {
		t.Fatal("official rule reader accepted a traversal path")
	}
}

func TestManjuArtifactVisibilityExposesOnlyUserDeliverables(t *testing.T) {
	cases := []struct {
		phase string
		stage string
		want  string
	}{
		{"P1", ScriptStageScriptDraft, model.ManjuArtifactVisibilityInternal},
		{"P2", AssetStageExtraction, model.ManjuArtifactVisibilityInternal},
		{"P2", AssetStageManifest, model.ManjuArtifactVisibilityInternal},
		{"P2", AssetStageCompile, model.ManjuArtifactVisibilityUser},
		{"P3", ProductionStageStoryboard, model.ManjuArtifactVisibilityUser},
		{"P4", ProductionStageFeed, model.ManjuArtifactVisibilityUser},
		{"P4.5", ProductionStageFeed, model.ManjuArtifactVisibilityInternal},
		{"P5", ProductionStageQC, model.ManjuArtifactVisibilityInternal},
	}
	for _, tc := range cases {
		if got := manjuArtifactVisibility(tc.phase, tc.stage); got != tc.want {
			t.Fatalf("visibility(%s,%s) = %s, want %s", tc.phase, tc.stage, got, tc.want)
		}
	}
}
