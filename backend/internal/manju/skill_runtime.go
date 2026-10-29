package manju

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"infinite-canvas/backend/internal/model"
)

//go:embed all:skilldata/v7.4.26
var embeddedManjuSkill embed.FS

const manjuSkillVersion = "V7.4.26"

const manjuSkillContentHash = "3f505ee09eabb4d414cb93f1724014ed2aa277951eaaa275a3f605a30c4a8c4f"

type ManjuSkillRuntime struct {
	once sync.Once
	root string
	err  error
}

type ManjuSkillStageOptions struct {
	Stage          string
	Track          string
	Source         string
	Combat         string
	Model          string
	Mode           string
	SeamMode       string
	ExternalScript bool
	Timeout        time.Duration
}

type ManjuSkillValidationOptions struct {
	Model                 string
	Track                 string
	Combat                string
	Mode                  string
	Aspect                string
	StyleSource           string
	StylePromptParameters string
	StyleVisualPrompt     string
	StyleMotionParameters string
	StyleForbiddenItems   []string
	StyleBans             string
	Assets                int
	StateJSON             string
	UpstreamScript        string
	Timeout               time.Duration
}

type ManjuSkillValidationResult struct {
	Passed           bool
	ExitCode         int
	Output           string
	Issues           []string
	DeferredIssues   []string
	Warnings         []string
	DialogueWarnings []string
}

var defaultManjuSkillRuntime = &ManjuSkillRuntime{}

func (r *ManjuSkillRuntime) ensureRoot() (string, error) {
	r.once.Do(func() {
		source, err := fs.Sub(embeddedManjuSkill, "skilldata/v7.4.26")
		if err != nil {
			r.err = err
			return
		}
		root, err := os.MkdirTemp("", "visideep-manju-skill-v7426-*")
		if err != nil {
			r.err = err
			return
		}
		err = fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == "." {
				return nil
			}
			target := filepath.Join(root, filepath.FromSlash(path))
			if entry.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			data, readErr := fs.ReadFile(source, path)
			if readErr != nil {
				return readErr
			}
			if mkdirErr := os.MkdirAll(filepath.Dir(target), 0o755); mkdirErr != nil {
				return mkdirErr
			}
			mode := fs.FileMode(0o644)
			if strings.HasSuffix(strings.ToLower(path), ".py") || strings.HasSuffix(strings.ToLower(path), ".sh") {
				mode = 0o755
			}
			return os.WriteFile(target, data, mode)
		})
		if err != nil {
			_ = os.RemoveAll(root)
			r.err = err
			return
		}
		r.root = root
	})
	if r.err != nil {
		return "", r.err
	}
	return r.root, nil
}

func (r *ManjuSkillRuntime) EmbeddedFiles() ([]string, error) {
	source, err := fs.Sub(embeddedManjuSkill, "skilldata/v7.4.26")
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, 170)
	err = fs.WalkDir(source, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func (r *ManjuSkillRuntime) EmbeddedRuleHash() (string, error) {
	files, err := r.EmbeddedFiles()
	if err != nil {
		return "", err
	}
	source, err := fs.Sub(embeddedManjuSkill, "skilldata/v7.4.26")
	if err != nil {
		return "", err
	}
	var manifest strings.Builder
	for _, path := range files {
		data, err := fs.ReadFile(source, path)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(&manifest, "%s:%x\n", path, sum)
	}
	sum := sha256.Sum256([]byte(manifest.String()))
	return fmt.Sprintf("%x", sum), nil
}

func (r *ManjuSkillRuntime) StageContext(options ManjuSkillStageOptions) (string, error) {
	bundle, err := r.StageRuleDispatch(options)
	if err != nil {
		return "", err
	}
	return bundle.Content, nil
}

func (r *ManjuSkillRuntime) StageRuleDispatch(options ManjuSkillStageOptions) (*OfficialRuleBundle, error) {
	stage := strings.ToUpper(strings.TrimSpace(options.Stage))
	if stage == "" {
		return nil, errors.New("漫剧规则库阶段不能为空")
	}
	bundle, err := r.OfficialRuleBundle(stage, options)
	if err != nil {
		return nil, fmt.Errorf("加载 V7.4.26 %s 官方总控规则失败: %w", stage, err)
	}
	return bundle, nil
}

func (r *ManjuSkillRuntime) ValidateP4(content string, options ManjuSkillValidationOptions) (ManjuSkillValidationResult, error) {
	contract, err := loadP4FeedContract()
	if err != nil {
		return ManjuSkillValidationResult{}, err
	}
	tempFile, cleanup, err := writeManjuSkillTemp("p4-feed-*.md", content)
	if err != nil {
		return ManjuSkillValidationResult{}, err
	}
	defer cleanup()
	statePath := ""
	if strings.TrimSpace(options.StateJSON) != "" {
		var stateCleanup func()
		statePath, stateCleanup, err = writeManjuSkillTemp("project-state-*.json", options.StateJSON)
		if err != nil {
			return ManjuSkillValidationResult{}, err
		}
		defer stateCleanup()
	}

	model := normalizeManjuSkillModel(options.Model)
	mode := strings.ToUpper(fallbackValue(options.Mode, AssetModeM1Value))
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}

	validateArgs := p4ValidateArgs(tempFile, statePath, options)
	validateOutput, validateCode, validateErr := r.runPython(filepath.FromSlash(contract.Validator.Script), validateArgs, timeout)
	result := ManjuSkillValidationResult{Output: validateOutput, ExitCode: validateCode}
	parsed, parsedOK := decodeP4ValidationOutput(validateOutput)
	validatorOutputOK := strings.TrimSpace(validateOutput) != "" && validP4ValidatorOutput(validateOutput)
	if parsedOK {
		result.Issues, result.DeferredIssues = splitP4DeferredContractIssues(parsed.Issues)
		result.Warnings = parsed.Warnings
		if len(result.DeferredIssues) > 0 {
			result.Warnings = append(
				result.Warnings,
				"CT5-P45 已延后至 P4.5 独立附件校验，不阻断 P4 文本编译",
			)
		}
	}
	onlyDeferredIssues := validatorOutputOK &&
		len(parsed.Issues) > 0 &&
		len(result.Issues) == 0 &&
		len(result.DeferredIssues) > 0
	if !validatorOutputOK && validateCode == 0 {
		result.Issues = append(result.Issues, "validate_prompt.py exit 0 未返回有效 JSON 结果")
	}
	if (validateErr != nil || validateCode != 0 || !validatorOutputOK || len(result.Issues) > 0) && !onlyDeferredIssues {
		result.Output = strings.TrimSpace(validateOutput)
		if result.Output == "" {
			if validateErr != nil {
				result.Output = validateErr.Error()
			} else {
				result.Output = fmt.Sprintf("validate_prompt.py exit %d", validateCode)
			}
		}
		if len(result.Issues) == 0 {
			result.Issues = append(result.Issues, firstNonEmptyLine(result.Output, fmt.Sprintf("validate_prompt.py exit %d", validateCode)))
		}
		if route := p4RepairRouteInstruction(result.Issues...); route != "" {
			result.Output = strings.TrimSpace(result.Output + "\n" + route)
		}
		return result, nil
	}

	assets := options.Assets
	if assets <= 0 {
		assets = contract.Gate.DefaultAssets
	}
	gateArgs := []string{tempFile, "--model", model, "--mode", mode, "--assets", fmt.Sprint(assets)}
	gateOutput, gateCode, gateErr := r.runPython(filepath.FromSlash(contract.Gate.Script), gateArgs, timeout)
	result.Output = strings.TrimSpace(strings.Join([]string{strings.TrimSpace(validateOutput), strings.TrimSpace(gateOutput)}, "\n\n"))
	result.ExitCode = gateCode
	result.Output = strings.TrimSpace(gateOutput)
	gateOutputOK := validP4GateOutput(gateOutput)
	if gateErr != nil || gateCode != 0 || !gateOutputOK {
		if !gateOutputOK && gateCode == 0 {
			result.Issues = append(result.Issues, "gate_check.py exit 0 未返回含 FAIL=0 的标准汇总")
		}
		if gateOutput != "" {
			result.Issues = append(result.Issues, firstFailureLines(gateOutput)...)
		}
		if len(result.Issues) == 0 && gateErr != nil {
			result.Issues = append(result.Issues, gateErr.Error())
		}
		if result.Output == "" {
			result.Output = fmt.Sprintf("gate_check.py exit %d", gateCode)
		}
		if route := p4RepairRouteInstruction(result.Issues...); route != "" {
			result.Output = strings.TrimSpace(result.Output + "\n" + route)
		}
		return result, nil
	}
	result.Passed = true
	return result, nil
}

func validP4ValidatorOutput(output string) bool {
	if strings.TrimSpace(output) == "" {
		return false
	}
	_, ok := decodeP4ValidationOutput(output)
	return ok
}

func validP4GateOutput(output string) bool {
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		const prefix = "汇总：FAIL="
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, prefix))
		return len(fields) > 0 && fields[0] == "0"
	}
	return false
}

func (r *ManjuSkillRuntime) ValidateAssetBlocks(content string) (ManjuSkillValidationResult, error) {
	contract, err := loadAssetBlockContract()
	if err != nil {
		return ManjuSkillValidationResult{}, err
	}
	if err := contract.validateHeadingStructure(content); err != nil {
		message := err.Error()
		return ManjuSkillValidationResult{
			ExitCode: 1,
			Output:   message,
			Issues:   []string{message},
		}, nil
	}
	tempFile, cleanup, err := writeManjuSkillTemp("p2-assets-*.md", content)
	if err != nil {
		return ManjuSkillValidationResult{}, err
	}
	defer cleanup()
	output, code, runErr := r.runPython(
		filepath.Join("short-drama-director", "scripts", "check_asset_blocks.py"),
		[]string{tempFile},
		90*time.Second,
	)
	result := ManjuSkillValidationResult{Output: strings.TrimSpace(output), ExitCode: code}
	if runErr != nil || code != 0 {
		result.Issues = firstFailureLines(output)
		if len(result.Issues) == 0 {
			if runErr != nil {
				result.Issues = append(result.Issues, runErr.Error())
			} else {
				result.Issues = append(result.Issues, fmt.Sprintf("check_asset_blocks.py exit %d", code))
			}
		}
		return result, nil
	}
	result.Passed = true
	return result, nil
}

func (r *ManjuSkillRuntime) ValidateP45(content string, segments int) (ManjuSkillValidationResult, error) {
	dir, err := os.MkdirTemp("", "visideep-p45-*")
	if err != nil {
		return ManjuSkillValidationResult{}, err
	}
	defer os.RemoveAll(dir)
	if segments <= 0 {
		return ManjuSkillValidationResult{}, errors.New("P4.5 校验段数必须大于 0")
	}
	tempFile := filepath.Join(dir, "项目-预演故事板.md")
	if err := os.WriteFile(tempFile, []byte(content), 0o600); err != nil {
		return ManjuSkillValidationResult{}, err
	}
	output, code, runErr := r.runPython(
		filepath.Join("sd-storyboard-engine", "scripts", "check_p45.py"),
		[]string{tempFile, "--segments", fmt.Sprint(segments)},
		90*time.Second,
	)
	result := ManjuSkillValidationResult{Output: strings.TrimSpace(output), ExitCode: code}
	if runErr != nil || code != 0 {
		result.Issues = firstFailureLines(output)
		if len(result.Issues) == 0 {
			if runErr != nil {
				result.Issues = append(result.Issues, runErr.Error())
			} else {
				result.Issues = append(result.Issues, fmt.Sprintf("check_p45.py exit %d", code))
			}
		}
		return result, nil
	}
	result.Passed = true
	return result, nil
}

func (r *ManjuSkillRuntime) ValidateDialogueIntegrity(
	upstreamScript, feed string,
	timeout time.Duration,
) (ManjuSkillValidationResult, error) {
	if strings.TrimSpace(upstreamScript) == "" {
		return ManjuSkillValidationResult{}, ErrProjectStateUnavailable
	}
	if strings.TrimSpace(feed) == "" {
		return ManjuSkillValidationResult{
			ExitCode: -1,
			Output:   "投喂稿为空，无法执行 script_feed_diff.py",
			Issues:   []string{"投喂稿为空，无法执行 script_feed_diff.py"},
		}, nil
	}
	scriptPath, scriptCleanup, err := writeManjuSkillTemp("p3-upstream-*.md", upstreamScript)
	if err != nil {
		return ManjuSkillValidationResult{}, err
	}
	defer scriptCleanup()
	feedPath, feedCleanup, err := writeManjuSkillTemp("p4-feed-*.md", feed)
	if err != nil {
		return ManjuSkillValidationResult{}, err
	}
	defer feedCleanup()
	return r.runDialogueDiff(scriptPath, feedPath, upstreamScript, timeout)
}

func (r *ManjuSkillRuntime) runDialogueDiff(scriptPath, feedPath, upstreamScript string, timeout time.Duration) (ManjuSkillValidationResult, error) {
	args := []string{"--script", scriptPath, "--feed", feedPath}
	if names := dialogueNamesFromScript(upstreamScript); len(names) > 0 {
		args = append(args, "--names", strings.Join(names, ","))
	}
	output, code, runErr := r.runPython(
		filepath.Join("sd-feed-engine", "scripts", "script_feed_diff.py"),
		args,
		timeout,
	)
	result := ManjuSkillValidationResult{Output: strings.TrimSpace(output), ExitCode: code}
	if !validDialogueDiffOutput(result.Output) {
		message := "script_feed_diff.py 未返回有效台词完整性报告"
		if runErr != nil {
			message += ": " + runErr.Error()
		}
		if result.Output != "" {
			message += ": " + firstNonEmptyLine(result.Output, result.Output)
		}
		result.Issues = append(result.Issues, message)
		return result, nil
	}
	switch code {
	case 0:
		result.Passed = runErr == nil
		if !result.Passed {
			result.Issues = append(result.Issues, "script_feed_diff.py exit 0 但执行未成功: "+runErr.Error())
		}
	case 1:
		result.Passed = true
		result.DialogueWarnings = dialogueDiffWarnings(result.Output)
		if len(result.DialogueWarnings) == 0 {
			result.DialogueWarnings = append(result.DialogueWarnings, "script_feed_diff.py 报告台词部分覆盖，需逐句核对")
		}
	case 2:
		result.Issues = dialogueDiffIssues(result.Output)
		if len(result.Issues) == 0 {
			result.Issues = append(result.Issues, "script_feed_diff.py exit 2：投喂稿存在缺失台词")
		}
	default:
		message := fmt.Sprintf("script_feed_diff.py exit %d", code)
		if runErr != nil {
			message += ": " + runErr.Error()
		}
		result.Issues = append(result.Issues, message)
	}
	return result, nil
}

func dialogueNamesFromScript(script string) []string {
	seen := map[string]bool{}
	names := make([]string, 0, 4)
	for _, rawLine := range strings.Split(script, "\n") {
		line := strings.TrimSpace(rawLine)
		hadDialoguePrefix := strings.HasPrefix(line, "台词：")
		line = strings.TrimPrefix(line, "台词：")
		for {
			index := strings.IndexAny(line, "：:")
			if index <= 0 {
				break
			}
			name := strings.TrimSpace(line[:index])
			if pause := strings.IndexAny(name, "（("); pause >= 0 {
				name = strings.TrimSpace(name[:pause])
			}
			if hadDialoguePrefix && name == "无" {
				if strings.HasPrefix(line[index:], "：") {
					line = strings.TrimSpace(line[index+len("："):])
				} else {
					line = strings.TrimSpace(line[index+1:])
				}
				continue
			}
			if name != "" && name != "无" && !dialogueScriptFieldLabel(name) {
				if !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
			break
		}
	}
	sort.Strings(names)
	return names
}

func dialogueScriptFieldLabel(name string) bool {
	switch name {
	case "场景", "画面内容", "状态", "光影", "色调", "台词", "重音", "人声", "音效",
		"文字平面图", "导演稿", "角色表演档案", "分镜总表", "故事板表", "交付清单",
		"台词精简建议清单", "P3 交付清单":
		return true
	default:
		return false
	}
}

func validDialogueDiffOutput(output string) bool {
	return strings.Contains(output, "=== 汇总 ===") &&
		strings.Contains(output, "❌ 缺失") &&
		strings.Contains(output, "⚠ 弱匹配")
}

func dialogueDiffWarnings(output string) []string {
	warnings := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "⚠ 部分覆盖") || strings.HasPrefix(line, "⚠ 弱匹配") {
			warnings = append(warnings, line)
		}
	}
	return warnings
}

func dialogueDiffIssues(output string) []string {
	issues := []string{"台词完整性校验失败：投喂稿存在缺失台词"}
	captureMissing := false
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "★ 缺失明细") {
			captureMissing = true
			continue
		}
		if captureMissing && trimmed != "" {
			issues = append(issues, trimmed)
			continue
		}
		if strings.Contains(trimmed, "❌ 缺失") {
			issues = append(issues, trimmed)
		}
	}
	return issues
}

func (r *ManjuSkillRuntime) PrevisContext() (string, error) {
	root, err := r.ensureRoot()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(root, "sd-storyboard-engine", "references", "p45-previs-storyboard.md"))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *ManjuSkillRuntime) DefaultProjectStateTemplate() (string, error) {
	root, err := r.ensureRoot()
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp("", "project-state-template-*.json")
	if err != nil {
		return "", err
	}
	path := file.Name()
	_ = file.Close()
	defer os.Remove(path)
	output, code, runErr := r.runPython(
		filepath.Join("short-drama-director", "scripts", "sd_core.py"),
		[]string{"--init-state", path}, 60*time.Second,
	)
	if runErr != nil || code != 0 {
		return "", fmt.Errorf("sd_core.py --init-state 失败: %w: %s", runErr, strings.TrimSpace(output))
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err == nil {
		return string(data), nil
	}
	// sd_core.py receives an absolute output path; read that path directly.
	data, err = os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *ManjuSkillRuntime) CompileFeed(stateJSON string, options ManjuSkillValidationOptions) (string, ManjuSkillValidationResult, error) {
	if strings.TrimSpace(stateJSON) == "" {
		return "", ManjuSkillValidationResult{}, ErrProjectStateUnavailable
	}
	preparedStateJSON, err := r.prepareOfficialCompileState(stateJSON, options)
	if err != nil {
		return "", ManjuSkillValidationResult{}, err
	}
	stateJSON = preparedStateJSON
	statePath, cleanup, err := writeManjuSkillTemp("project-state-*.json", stateJSON)
	if err != nil {
		return "", ManjuSkillValidationResult{}, err
	}
	defer cleanup()

	output, code, runErr := r.runPython(
		filepath.Join("short-drama-director", "scripts", "compile_feed.py"),
		[]string{"--state", statePath},
		options.Timeout,
	)
	feed := strings.TrimSpace(output)
	if runErr != nil || code != 0 || feed == "" {
		message := strings.TrimSpace(output)
		if message == "" && runErr != nil {
			message = runErr.Error()
		}
		return feed, ManjuSkillValidationResult{Output: message, ExitCode: code}, fmt.Errorf("官方 compile_feed.py 编译失败: %s", message)
	}
	options.StateJSON = stateJSON
	validationOptions := options
	if strings.TrimSpace(validationOptions.StyleSource) != "" {
		if _, found, styleErr := r.officialStyleLockBlocks(validationOptions.StyleSource); styleErr == nil && !found {
			// 自定义画风使用通用段8/段9，不是导演卡 11 槽位，不触发 C23d2 导演转译对账。
			validationOptions.StyleSource = ""
		}
	}
	validation, validationErr := r.ValidateP4(feed, validationOptions)
	if validationErr != nil || !validation.Passed {
		return feed, validation, validationErr
	}
	dialogueValidation, dialogueErr := r.ValidateDialogueIntegrity(
		options.UpstreamScript,
		feed,
		options.Timeout,
	)
	combined := validation
	combined.Output = strings.TrimSpace(strings.Join([]string{
		strings.TrimSpace(validation.Output),
		strings.TrimSpace(dialogueValidation.Output),
	}, "\n\n"))
	combined.ExitCode = dialogueValidation.ExitCode
	combined.Passed = dialogueValidation.Passed
	combined.Issues = append(combined.Issues, dialogueValidation.Issues...)
	combined.DialogueWarnings = append(combined.DialogueWarnings, dialogueValidation.DialogueWarnings...)
	if dialogueErr != nil {
		combined.Passed = false
		message := "台词完整性基线不可用: " + dialogueErr.Error()
		combined.Issues = append(combined.Issues, message)
		if combined.Output != "" {
			combined.Output += "\n\n"
		}
		combined.Output += message
	}
	return feed, combined, nil
}

func (r *ManjuSkillRuntime) prepareOfficialCompileState(stateJSON string, options ManjuSkillValidationOptions) (string, error) {
	var state map[string]any
	if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
		return "", fmt.Errorf("解析 P4 编译状态失败: %w", err)
	}
	if strings.TrimSpace(options.StyleSource) != "" {
		blocks, found, err := r.officialStyleLockBlocks(options.StyleSource)
		if err != nil {
			return "", err
		}
		styleBlock, performanceBlock := splitStyleLockBlocks(blocks)
		if !found {
			lensMin, lensMax := genericStyleLensRange(stateJSON)
			styleBlock, performanceBlock = genericStylePromptBlocksWithLens(
				options.StylePromptParameters,
				options.StyleMotionParameters,
				lensMin,
				lensMax,
			)
		}
		locks := anyMap(state["locks"])
		if locks == nil {
			locks = map[string]any{}
		}
		locks["style_block"] = styleBlock
		locks["performance_block"] = performanceBlock
		state["locks"] = locks
	}
	prepared, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化 P4 编译状态失败: %w", err)
	}
	return string(prepared), nil
}

func syncManjuCompileStyleFingerprint(state map[string]any, visual string) {
	if state == nil || strings.TrimSpace(visual) == "" {
		return
	}
	receipt := anyMap(state["receipt"])
	if receipt == nil {
		receipt = map[string]any{}
	}
	fingerprints := anySlice(receipt["fingerprints"])
	updated := make([]any, 0, len(fingerprints)+1)
	found := false
	for _, raw := range fingerprints {
		text := strings.TrimSpace(anyString(raw))
		if strings.HasPrefix(text, "【P0 可投喂视觉参数：") {
			updated = append(updated, fmt.Sprintf("【P0 可投喂视觉参数：%s】", strings.TrimSpace(visual)))
			found = true
			continue
		}
		if text != "" {
			updated = append(updated, text)
		}
	}
	if !found {
		updated = append(updated, fmt.Sprintf("【P0 可投喂视觉参数：%s】", strings.TrimSpace(visual)))
	}
	receipt["fingerprints"] = updated
	state["receipt"] = receipt
}

func syncManjuCompileLocks(state map[string]any, options ManjuSkillValidationOptions) {
	if state == nil {
		return
	}
	locks := anyMap(state["locks"])
	if locks == nil {
		locks = map[string]any{}
	}
	if value := strings.TrimSpace(options.Model); value != "" {
		locks["model"] = normalizeManjuSkillModel(value)
	}
	if value := strings.TrimSpace(options.Track); value != "" {
		locks["track"] = value
	}
	if value := strings.TrimSpace(options.Aspect); value != "" {
		locks["aspect"] = value
	}
	if value := strings.ToUpper(strings.TrimSpace(options.Mode)); value != "" {
		locks["asset_mode"] = value
	}
	if value := strings.TrimSpace(options.StylePromptParameters); value != "" {
		locks["style_card"] = value
	} else if value := strings.TrimSpace(options.StyleSource); value != "" {
		locks["style_card"] = value
	}
	if value := strings.TrimSpace(options.StyleMotionParameters); value != "" {
		locks["style_motion"] = value
	}
	if len(options.StyleForbiddenItems) > 0 {
		locks["style_forbidden"] = options.StyleForbiddenItems
		worldAnchor := anyMap(state["world_anchor"])
		if worldAnchor == nil {
			worldAnchor = map[string]any{}
		}
		merged := make([]any, 0, len(anySlice(worldAnchor["forbidden_elements"]))+len(options.StyleForbiddenItems))
		seen := map[string]bool{}
		for _, raw := range anySlice(worldAnchor["forbidden_elements"]) {
			item := strings.TrimSpace(anyString(raw))
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			merged = append(merged, item)
		}
		for _, item := range uniqueCustomStyleItems(options.StyleForbiddenItems...) {
			if seen[item] {
				continue
			}
			seen[item] = true
			merged = append(merged, item)
		}
		worldAnchor["forbidden_elements"] = merged
		state["world_anchor"] = worldAnchor
	}
	state["locks"] = locks
}

func (r *ManjuSkillRuntime) AuditContract(content string, director string) (ManjuSkillValidationResult, error) {
	tempFile, cleanup, err := writeManjuSkillTemp("full-contract-*.md", content)
	if err != nil {
		return ManjuSkillValidationResult{}, err
	}
	defer cleanup()
	args := []string{tempFile, "--mode", "full", "--delivery", "full-project"}
	if strings.TrimSpace(director) != "" {
		args = append(args, "--director", strings.TrimSpace(director))
	}
	output, code, runErr := r.runPython(filepath.Join("short-drama-director", "scripts", "audit_contract.py"), args, 90*time.Second)
	result := ManjuSkillValidationResult{Output: output, ExitCode: code, Passed: code == 0 && runErr == nil}
	if !result.Passed {
		result.Issues = firstFailureLines(output)
		if len(result.Issues) == 0 && runErr != nil {
			result.Issues = append(result.Issues, runErr.Error())
		}
	}
	return result, nil
}

func (r *ManjuSkillRuntime) BuildBoard(markdown string) (string, error) {
	if strings.TrimSpace(markdown) == "" {
		return "", errors.New("官方 build_board_lite.py 拒绝空 Markdown 源稿")
	}
	dir, err := os.MkdirTemp("", "manju-board-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	markdownPath := filepath.Join(dir, "canonical.md")
	boardPath := filepath.Join(dir, "board.html")
	if err := os.WriteFile(markdownPath, []byte(markdown), 0o600); err != nil {
		return "", err
	}
	output, code, runErr := r.runPython(
		filepath.Join("short-drama-director", "scripts", "build_board_lite.py"),
		[]string{markdownPath, "--out", boardPath},
		90*time.Second,
	)
	if runErr != nil {
		return "", fmt.Errorf("官方 build_board_lite.py 执行失败: %w: %s", runErr, strings.TrimSpace(output))
	}
	if code != 0 {
		return "", fmt.Errorf("官方 build_board_lite.py 退出码 %d: %s", code, strings.TrimSpace(output))
	}
	board, err := os.ReadFile(boardPath)
	if err != nil {
		return "", fmt.Errorf("读取官方 build_board_lite.py 输出失败: %w", err)
	}
	if strings.TrimSpace(string(board)) == "" {
		return "", errors.New("官方 build_board_lite.py 输出为空")
	}
	return string(board), nil
}

func (r *ManjuSkillRuntime) SelfTest() error {
	checks := [][]string{
		{filepath.Join("short-drama-director", "scripts", "sd_core.py"), "--self-test"},
		{filepath.Join("short-drama-director", "scripts", "audit_contract.py"), "--self-test"},
		{filepath.Join("short-drama-director", "scripts", "validate_prompt.py"), "--self-test"},
		{filepath.Join("short-drama-director", "scripts", "compile_feed.py"), "--self-test"},
		{filepath.Join("short-drama-director", "scripts", "build_board_lite.py"), "--self-test"},
		{filepath.Join("short-drama-director", "scripts", "style-lock-ladder.py"), "--self-test"},
		{filepath.Join("short-drama-director", "scripts", "style-lock-recommend.py"), "--self-test"},
		{filepath.Join("sd-feed-engine", "scripts", "gate_check.py"), "--self-test"},
		{filepath.Join("sd-storyboard-engine", "scripts", "check_p45.py"), "--self-test"},
		{filepath.Join("short-drama-director", "scripts", "test_project_bundle.py")},
		{filepath.Join("short-drama-director", "scripts", "test_slice_stability.py")},
		{filepath.Join("short-drama-director", "scripts", "test_validator_stability.py")},
	}
	for _, check := range checks {
		output, code, err := r.runPython(check[0], check[1:], 180*time.Second)
		if err != nil || code != 0 {
			return fmt.Errorf("%s 自检失败: %w: %s", check[0], err, strings.TrimSpace(output))
		}
	}
	return nil
}

func (r *ManjuSkillRuntime) runPython(relativeScript string, args []string, timeout time.Duration) (string, int, error) {
	root, err := r.ensureRoot()
	if err != nil {
		return "", -1, err
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	python, err := manjuPythonExecutable()
	if err != nil {
		return "", -1, err
	}
	commandArgs := append([]string{filepath.Join(root, relativeScript)}, args...)
	cmd := exec.CommandContext(ctx, python, commandArgs...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			return output.String(), -1, ctx.Err()
		} else {
			return output.String(), -1, err
		}
	}
	return output.String(), code, err
}

func manjuPythonExecutable() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("MANJU_PYTHON")); configured != "" {
		return configured, nil
	}
	candidates := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		candidates = []string{"python", "python3"}
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", errors.New("服务端未安装 Python，V7.4.26 规则脚本无法运行")
}

func writeManjuSkillTemp(pattern, content string) (string, func(), error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	path := file.Name()
	cleanup := func() { _ = os.Remove(path) }
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func normalizeManjuSkillModel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "seedance-2.5", "2.5", "seedance 2.5":
		return "2.5"
	case "seedance-2.0", "2.0", "seedance 2.0":
		return "2.0"
	case "minimax-h3", "h3", "minimax h3":
		return "h3"
	default:
		return "2.5"
	}
}

func firstNonEmptyLine(output, fallback string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return fallback
}

func truncateManjuSkillText(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "\n[已截断]"
}

func firstFailureLines(output string) []string {
	lines := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "[FAIL]") || strings.HasPrefix(line, "[-]") || strings.Contains(line, "FAIL:") || strings.HasPrefix(line, "✗") {
			lines = append(lines, line)
		}
	}
	return lines
}

func manjuSkillVersionLabel() string {
	return manjuSkillVersion
}

func appendManjuSkillContext(out *strings.Builder, skillContext string) {
	out.WriteString("\n\n===== V7.4.26 严格规则执行边界 =====\n")
	out.WriteString(manjuStrictRuleBoundaryText())
	out.WriteString("\n\n===== V7.4.26 官方总控强制分发原文 =====\n")
	out.WriteString(skillContext)
}

func manjuStrictRuleBoundaryText() string {
	return strings.Join([]string{
		"规则原文是唯一输出依据。",
		"不得摘要、改写、重排、压缩或合并官方模板。",
		"不得摘要。",
		"不得改写。",
		"不得重排。",
		"不得压缩。",
		"不得合并官方模板。",
		"不得使用 BeefTV 自创的替代格式替换官方标题、章节、字段或顺序。",
		"不得把规则文档中的操作说明、开关状态、流程备注或检查说明复制进最终产物。",
		"固定字段名必须逐字保留，不得改写、合并或省略。",
		"上游已锁定工件必须原样承接；需要修改时只能回到对应上游阶段。",
		"不合格时只允许回本阶段重新生成，不得由 BeefTV 自动改写内容。",
	}, "\n")
}

func assetModeFromReceipt(receipt *model.ManjuLockReceipt) string {
	if receipt == nil {
		return AssetModeM1Value
	}
	view, err := receiptView(*receipt)
	if err != nil {
		return AssetModeM1Value
	}
	return fallbackValue(view.Snapshot.AssetMode, AssetModeM1Value)
}

func manjuSkillTrackCode(track string) string {
	switch strings.TrimSpace(track) {
	case "overseas":
		return "2"
	case "explainer":
		return "3"
	case "other":
		return "4"
	default:
		return "1"
	}
}

func manjuSkillSourceCode(source string) string {
	switch strings.TrimSpace(source) {
	case SourceAdaptation:
		return "2"
	case SourceBenchmark:
		return "3"
	case SourceTrend:
		return "4"
	case "polish":
		// Historical receipts may still use the retired fifth source.
		return "2"
	default:
		return "1"
	}
}

func detectManjuCombat(texts ...string) string {
	text := strings.Join(texts, "\n")
	switch {
	case containsAnyManjuText(text, "法术", "灵力", "御剑", "法阵", "元神", "玄幻", "仙侠", "神魔", "妖兽"):
		return "R3"
	case containsAnyManjuText(text, "枪战", "狙击", "追车", "暗杀", "搏击", "近身格斗", "防弹", "军警", "警匪"):
		return "R1"
	case containsAnyManjuText(text, "武侠", "兵器", "刀剑", "拳脚", "招式", "轻功", "打戏", "武打"):
		return "R2"
	default:
		return "none"
	}
}

func productionRuleCombat(
	scriptArtifacts []model.ManjuScriptArtifact,
	assetArtifacts []model.ManjuAssetArtifact,
	productionArtifacts []model.ManjuProductionArtifact,
	instruction string,
) string {
	var facts strings.Builder
	for _, artifact := range scriptArtifacts {
		facts.WriteString(artifact.Content)
		facts.WriteByte('\n')
	}
	for _, artifact := range assetArtifacts {
		facts.WriteString(artifact.Content)
		facts.WriteByte('\n')
	}
	for _, artifact := range productionArtifacts {
		facts.WriteString(artifact.Content)
		facts.WriteByte('\n')
	}
	facts.WriteString(instruction)
	return detectManjuCombat(facts.String())
}

func containsAnyManjuText(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func manjuSkillStageOptions(stage string, receipt *model.ManjuLockReceipt, mode, combat string) ManjuSkillStageOptions {
	options := ManjuSkillStageOptions{Stage: stage, Track: "1", Combat: fallbackValue(combat, "none"), Model: "2.5", Mode: fallbackValue(mode, AssetModeM1Value)}
	if receipt == nil {
		return options
	}
	view, err := receiptView(*receipt)
	if err != nil {
		return options
	}
	switch view.Snapshot.Track {
	case "overseas":
		options.Track = "2"
	case "explainer":
		options.Track = "3"
	case "other":
		options.Track = "4"
	}
	options.Model = normalizeManjuSkillModel(view.Snapshot.VideoModel)
	options.Source = manjuSkillSourceCode(view.Snapshot.Source)
	options.SeamMode = view.Snapshot.SeamMode
	if strings.TrimSpace(mode) == "" {
		options.Mode = fallbackValue(view.Snapshot.AssetMode, AssetModeM1Value)
	}
	return options
}
