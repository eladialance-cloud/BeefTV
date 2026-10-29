package manju

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

//go:embed output_contracts/p3-storyboard-v1.json
var p3StoryboardContractJSON []byte

//go:embed output_contracts/p4-feed-v1.json
var p4FeedContractJSON []byte

//go:embed output_contracts/p0b-decision-v1.json
var p0bDecisionContractJSON []byte

var (
	p0bDecisionContractOnce sync.Once
	p0bDecisionContractData P0BDecisionContract
	p0bDecisionContractErr  error

	p3StoryboardContractOnce sync.Once
	p3StoryboardContractData P3StoryboardContract
	p3StoryboardContractErr  error

	p4FeedContractOnce sync.Once
	p4FeedContractData P4FeedContract
	p4FeedContractErr  error

	ErrP0BDecisionContract  = errors.New("P0B 决策锚点不符合共享输出契约")
	ErrP3StoryboardContract = errors.New("P3 分镜与故事板不符合共享输出契约")
	ErrP4FeedContract       = errors.New("P4 投喂稿不符合共享输出契约")
)

type P0BDecisionContract struct {
	SchemaVersion    int                 `json:"schemaVersion"`
	RulesVersion     string              `json:"rulesVersion"`
	ArtifactKind     string              `json:"artifactKind"`
	RequiredSections []string            `json:"requiredSections"`
	RequiredColumns  map[string][]string `json:"requiredColumns"`
}

type contractSource struct {
	Role   string `json:"role"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type P3StoryboardContract struct {
	SchemaVersion           string                      `json:"schemaVersion"`
	RulesVersion            string                      `json:"rulesVersion"`
	Sources                 []contractSource            `json:"sources"`
	RequiredSections        []string                    `json:"requiredSections"`
	RequiredSectionMessages map[string]string           `json:"requiredSectionMessages"`
	Storyboard              P3StoryboardSectionContract `json:"storyboard"`
	Timeline                P3TimelineContract          `json:"timeline"`
	AssetNamespaces         map[string]string           `json:"assetNamespaces"`
	AssetRefs               P3AssetRefsContract         `json:"assetRefs"`
	Safety                  P3SafetyContract            `json:"safety"`
	C23f                    P3C23fContract              `json:"c23f"`
	StyleConstraint         P3StyleConstraintContract   `json:"styleConstraint"`
	Budget                  P3BudgetContract            `json:"budget"`
	Prompt                  P3PromptContract            `json:"prompt"`
	ProjectState            P3ProjectStateContract      `json:"projectState"`
}

type P3StoryboardSectionContract struct {
	HeadingTemplate         string            `json:"headingTemplate"`
	TableHeaders            []string          `json:"tableHeaders"`
	AssetReferenceRule      string            `json:"assetReferenceRule"`
	Phases                  []string          `json:"phases"`
	MinimumLayouts          map[string]string `json:"minimumLayouts"`
	PanelBoundaryIsNotCut   bool              `json:"panelBoundaryIsNotCut"`
	DeliveryChecklistFields []string          `json:"deliveryChecklistFields"`
}

type P3TimelineContract struct {
	ShotSpec               string   `json:"shotSpec"`
	RequiredShotFields     []string `json:"requiredShotFields"`
	IntegerSeconds         bool     `json:"integerSeconds"`
	Continuous             bool     `json:"continuous"`
	ResetEachSegmentToZero bool     `json:"resetEachSegmentToZero"`
	DialogueEmptyValue     string   `json:"dialogueEmptyValue"`
	DialogueRule           string   `json:"dialogueRule"`
}

type P3AssetRefsContract struct {
	Required     bool           `json:"required"`
	Namespaces   []string       `json:"namespaces"`
	LimitByModel map[string]int `json:"limitByModel"`
	Instruction  string         `json:"instruction"`
}

type P3SafetyContract struct {
	Source        string   `json:"source"`
	HighRiskWords []string `json:"highRiskWords"`
	Instruction   string   `json:"instruction"`
}

type P3C23fContract struct {
	FullBodyCharacterLimit int  `json:"fullBodyCharacterLimit"`
	ActionIntentLimit      int  `json:"actionIntentLimit"`
	SplitAtShotBoundary    bool `json:"splitAtShotBoundary"`
}

func sortedAssetRefKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func validateP3AssetRefsForP4(state map[string]any, videoModel string) error {
	contract := mustLoadP3StoryboardContract()
	characters := anyMap(state["characters"])
	scenes := anyMap(state["scenes"])
	props := anyMap(state["props"])
	segments := anySlice(state["segments"])
	limit := contract.AssetRefs.LimitByModel[normalizeManjuSkillModel(videoModel)]
	issues := make([]string, 0)
	for index, raw := range segments {
		segment := anyMap(raw)
		segmentLabel := fallbackValue(anyString(segment["seg_id"]), fmt.Sprintf("段%d", index+1))
		refs, ok := segment["asset_refs"].(map[string]any)
		if !ok {
			issues = append(issues, segmentLabel+" 缺少 asset_refs")
			continue
		}
		sceneRefs := stringList(refs["scenes"])
		characterRefs := uniqueStrings(stringList(refs["characters"]))
		propRefs := uniqueStrings(stringList(refs["props"]))
		if len(sceneRefs) != 1 {
			issues = append(issues, segmentLabel+" asset_refs.scenes 必须且只能包含当前 scene")
		} else if current := anyString(segment["scene"]); current != "" && sceneRefs[0] != current {
			issues = append(issues, fmt.Sprintf("%s asset_refs.scenes=%q 与当前 scene=%q 不一致", segmentLabel, sceneRefs[0], current))
		}
		for _, name := range sceneRefs {
			if len(scenes) > 0 && scenes[name] == nil {
				issues = append(issues, fmt.Sprintf("%s 引用未知场景 %q", segmentLabel, name))
			}
		}
		for _, name := range characterRefs {
			if len(characters) > 0 && characters[name] == nil {
				issues = append(issues, fmt.Sprintf("%s 引用未知人物 %q", segmentLabel, name))
			}
		}
		for _, name := range propRefs {
			if len(props) > 0 && props[name] == nil {
				issues = append(issues, fmt.Sprintf("%s 引用未知道具 %q", segmentLabel, name))
			}
		}
		if limit > 0 && 1+len(characterRefs)+len(propRefs) > limit {
			issues = append(issues, fmt.Sprintf(
				"%s 实际引用 %d 个资产，超过模型 %s 单段上限 %d",
				segmentLabel, 1+len(characterRefs)+len(propRefs), normalizeManjuSkillModel(videoModel), limit,
			))
		}
		states := anyMap(refs["character_states"])
		for character, rawStates := range states {
			characterState := anyMap(characters[character])
			if len(characterState) == 0 {
				issues = append(issues, fmt.Sprintf("%s character_states 引用未知人物 %q", segmentLabel, character))
				continue
			}
			available := anyMap(characterState["char_states"])
			for _, stateKey := range stringList(rawStates) {
				if available[stateKey] == nil {
					issues = append(issues, fmt.Sprintf("%s 人物 %q 未声明状态 %q", segmentLabel, character, stateKey))
				}
			}
		}
	}
	if len(issues) > 0 {
		return fmt.Errorf("%w: %s", ErrP3StoryboardContract, strings.Join(issues, "；"))
	}
	return nil
}

func stringList(value any) []string {
	items := anySlice(value)
	result := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(anyString(item))
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

type P3StyleConstraintContract struct {
	Source              string `json:"source"`
	LensField           string `json:"lensField"`
	RangeOwner          string `json:"rangeOwner"`
	InstructionTemplate string `json:"instructionTemplate"`
}

type P3BudgetContract struct {
	SoftByModel map[string]int `json:"softByModel"`
	HardByModel map[string]int `json:"hardByModel"`
	Instruction string         `json:"instruction"`
}

type P3PromptContract struct {
	Heading       string   `json:"heading"`
	RequiredItems []string `json:"requiredItems"`
}

type P3ProjectStateContract struct {
	StageKey    string         `json:"stageKey"`
	Instruction string         `json:"instruction"`
	Example     map[string]any `json:"example"`
}

type P4FeedContract struct {
	SchemaVersion          string                   `json:"schemaVersion"`
	RulesVersion           string                   `json:"rulesVersion"`
	Sources                []contractSource         `json:"sources"`
	Sections               []string                 `json:"sections"`
	Validator              P4ValidatorContract      `json:"validator"`
	Gate                   P4GateContract           `json:"gate"`
	ExecutionOrder         []string                 `json:"executionOrder"`
	ShotLine               P4ShotLineContract       `json:"shotLine"`
	StyleLock              P4StyleLockContract      `json:"styleLock"`
	DeferredContractIssues P4DeferredContractIssues `json:"deferredContractIssues"`
	RepairRoutes           map[string]string        `json:"repairRoutes"`
}

type P4ValidatorContract struct {
	Script       string   `json:"script"`
	RequiredArgs []string `json:"requiredArgs"`
	OutputMode   string   `json:"outputMode"`
	FailClosed   bool     `json:"failClosed"`
}

type P4GateContract struct {
	Script        string `json:"script"`
	Mode          string `json:"mode"`
	DefaultAssets int    `json:"defaultAssets"`
	FailClosed    bool   `json:"failClosed"`
}

type P4ShotLineContract struct {
	RequiredElements  []string `json:"requiredElements"`
	SoundField        string   `json:"soundField"`
	DialogueField     string   `json:"dialogueField"`
	EmptyDialogue     string   `json:"emptyDialogue"`
	ActionIntentLimit int      `json:"actionIntentLimit"`
}

type P4StyleLockContract struct {
	InjectPosition    string   `json:"injectPosition"`
	StyleMarker       string   `json:"styleMarker"`
	PerformanceMarker string   `json:"performanceMarker"`
	RequiredTokens    []string `json:"requiredTokens"`
}

type P4DeferredContractIssues struct {
	Owner  string   `json:"owner"`
	Codes  []string `json:"codes"`
	Reason string   `json:"reason"`
}

type p4ValidationOutput struct {
	Issues   []string `json:"issues"`
	Warnings []string `json:"warnings"`
}

type styleLensRange struct {
	Min    int
	Max    int
	Source string
}

var styleLensRangePattern = regexp.MustCompile(`\[\s*(\d{1,3})\s*mm\s*,\s*(\d{1,3})\s*mm\s*\]`)

func loadP0BDecisionContract() (P0BDecisionContract, error) {
	p0bDecisionContractOnce.Do(func() {
		p0bDecisionContractErr = json.Unmarshal(p0bDecisionContractJSON, &p0bDecisionContractData)
		if p0bDecisionContractErr != nil {
			return
		}
		if p0bDecisionContractData.SchemaVersion != 1 ||
			p0bDecisionContractData.RulesVersion != manjuSkillVersion ||
			p0bDecisionContractData.ArtifactKind != "EMOTION_ANCHOR_TABLE.md" ||
			len(p0bDecisionContractData.RequiredSections) != 2 ||
			len(p0bDecisionContractData.RequiredColumns) != 2 {
			p0bDecisionContractErr = errors.New("P0B 决策锚点共享输出契约不完整")
		}
	})
	return p0bDecisionContractData, p0bDecisionContractErr
}

func mustLoadP0BDecisionContract() P0BDecisionContract {
	contract, err := loadP0BDecisionContract()
	if err != nil {
		panic(err)
	}
	return contract
}

func p0bDecisionPrompt() string {
	contract := mustLoadP0BDecisionContract()
	var out strings.Builder
	out.WriteString("任务：输出 P0B《情绪锚点表》决策工件。\n")
	out.WriteString("只输出以下合同要求的 Markdown 正文，不得用说明、清单或“详见上游”代替表格。\n")
	for _, heading := range contract.RequiredSections {
		out.WriteString(heading)
		out.WriteString("\n")
		columns := contract.RequiredColumns[heading]
		out.WriteString("| ")
		out.WriteString(strings.Join(columns, " | "))
		out.WriteString(" |\n|")
		for range columns {
			out.WriteString("---|")
		}
		out.WriteString("\n每张表至少填写 1 行有效数据。\n")
	}
	out.WriteString("《构图雷区白名单》每行的“允许条件”或“替代方案”至少填写一项；两项都为空时不得锁定。\n")
	return out.String()
}

func validateP0BDecisionArtifact(content string) error {
	contract, err := loadP0BDecisionContract()
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	issues := make([]string, 0)
	for _, heading := range contract.RequiredSections {
		section, found := p0bMarkdownSection(lines, heading)
		if !found {
			issues = append(issues, fmt.Sprintf("缺少必需章节 %s", heading))
			continue
		}
		table, found := p0bMarkdownTable(section)
		if !found {
			issues = append(issues, fmt.Sprintf("%s 缺少 Markdown 表格", heading))
			continue
		}
		if len(table.Rows) == 0 {
			issues = append(issues, fmt.Sprintf("%s 不能为空表", heading))
		}
		columnIndexes := make(map[string]int, len(table.Headers))
		for index, header := range table.Headers {
			columnIndexes[header] = index
		}
		for _, column := range contract.RequiredColumns[heading] {
			if _, ok := columnIndexes[column]; !ok {
				issues = append(issues, fmt.Sprintf("%s 缺少必需列 %s", heading, column))
			}
		}
		switch heading {
		case "## 情绪锚点表":
			for rowIndex, row := range table.Rows {
				for _, column := range contract.RequiredColumns[heading] {
					columnIndex, ok := columnIndexes[column]
					if !ok {
						continue
					}
					if p0bTableCell(row, columnIndex) == "" {
						issues = append(issues, fmt.Sprintf("情绪锚点表第 %d 行 %s 不能为空", rowIndex+1, column))
					}
				}
			}
		case "## 构图雷区白名单":
			for rowIndex, row := range table.Rows {
				zoneIndex := columnIndexes["雷区"]
				constraintIndex := columnIndexes["下游约束"]
				allowedIndex, hasAllowed := columnIndexes["允许条件"]
				replacementIndex, hasReplacement := columnIndexes["替代方案"]
				if zoneIndex < len(row) && strings.TrimSpace(row[zoneIndex]) == "" {
					issues = append(issues, fmt.Sprintf("构图雷区白名单第 %d 行 雷区 不能为空", rowIndex+1))
				}
				if constraintIndex < len(row) && strings.TrimSpace(row[constraintIndex]) == "" {
					issues = append(issues, fmt.Sprintf("构图雷区白名单第 %d 行 下游约束 不能为空", rowIndex+1))
				}
				allowed := hasAllowed && p0bOptionalTableCell(row, allowedIndex)
				replacement := hasReplacement && p0bOptionalTableCell(row, replacementIndex)
				if !allowed && !replacement {
					issues = append(issues, fmt.Sprintf("构图雷区白名单第 %d 行必须填写允许条件或替代方案", rowIndex+1))
				}
			}
		}
	}
	if len(issues) > 0 {
		return fmt.Errorf("%w: %s", ErrP0BDecisionContract, strings.Join(issues, "；"))
	}
	return nil
}

type p0bMarkdownTableData struct {
	Headers []string
	Rows    [][]string
}

func p0bMarkdownSection(lines []string, heading string) ([]string, bool) {
	start := -1
	for index, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = index
			break
		}
	}
	if start < 0 {
		return nil, false
	}
	section := make([]string, 0)
	for index := start + 1; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if strings.HasPrefix(trimmed, "#") {
			break
		}
		section = append(section, lines[index])
	}
	return section, true
}

func p0bMarkdownTable(lines []string) (p0bMarkdownTableData, bool) {
	for index := 0; index+1 < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if !strings.Contains(line, "|") {
			continue
		}
		headers, ok := p0bMarkdownRow(line)
		if !ok || !p0bMarkdownSeparator(strings.TrimSpace(lines[index+1]), len(headers)) {
			continue
		}
		rows := make([][]string, 0)
		for rowIndex := index + 2; rowIndex < len(lines); rowIndex++ {
			rowLine := strings.TrimSpace(lines[rowIndex])
			if rowLine == "" || !strings.Contains(rowLine, "|") {
				break
			}
			row, ok := p0bMarkdownRow(rowLine)
			if !ok {
				break
			}
			rows = append(rows, row)
		}
		return p0bMarkdownTableData{Headers: headers, Rows: rows}, true
	}
	return p0bMarkdownTableData{}, false
}

func p0bMarkdownRow(line string) ([]string, bool) {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	if line == "" {
		return nil, false
	}
	rawCells := strings.Split(line, "|")
	cells := make([]string, 0, len(rawCells))
	for _, cell := range rawCells {
		cell = strings.TrimSpace(cell)
		cell = strings.Trim(cell, "*`")
		cells = append(cells, strings.TrimSpace(cell))
	}
	return cells, len(cells) > 0
}

func p0bMarkdownSeparator(line string, columns int) bool {
	cells, ok := p0bMarkdownRow(line)
	if !ok || len(cells) != columns {
		return false
	}
	for _, cell := range cells {
		cell = strings.Trim(cell, ":")
		cell = strings.TrimSpace(cell)
		if len(cell) < 3 || strings.Trim(cell, "-") != "" {
			return false
		}
	}
	return true
}

func p0bTableCell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func p0bOptionalTableCell(row []string, index int) bool {
	value := strings.TrimSpace(p0bTableCell(row, index))
	switch value {
	case "", "-", "—", "/", "无", "N/A", "n/a":
		return false
	default:
		return true
	}
}

func loadP3StoryboardContract() (P3StoryboardContract, error) {
	p3StoryboardContractOnce.Do(func() {
		p3StoryboardContractErr = json.Unmarshal(p3StoryboardContractJSON, &p3StoryboardContractData)
		if p3StoryboardContractErr != nil {
			return
		}
		if p3StoryboardContractData.SchemaVersion == "" ||
			p3StoryboardContractData.RulesVersion != manjuSkillVersion ||
			len(p3StoryboardContractData.Sources) == 0 ||
			len(p3StoryboardContractData.RequiredSections) == 0 ||
			len(p3StoryboardContractData.Prompt.RequiredItems) == 0 ||
			p3StoryboardContractData.StyleConstraint.LensField == "" ||
			!p3StoryboardContractData.AssetRefs.Required ||
			len(p3StoryboardContractData.AssetRefs.Namespaces) != 3 ||
			p3StoryboardContractData.AssetRefs.LimitByModel["2.0"] <= 0 ||
			p3StoryboardContractData.AssetRefs.LimitByModel["2.5"] <= 0 ||
			p3StoryboardContractData.AssetRefs.LimitByModel["h3"] <= 0 ||
			len(p3StoryboardContractData.Safety.HighRiskWords) == 0 ||
			p3StoryboardContractData.Safety.Instruction == "" ||
			p3StoryboardContractData.ProjectState.StageKey == "" {
			p3StoryboardContractErr = errors.New("P3 分镜与故事板共享输出契约不完整")
		}
	})
	return p3StoryboardContractData, p3StoryboardContractErr
}

func loadP4FeedContract() (P4FeedContract, error) {
	p4FeedContractOnce.Do(func() {
		p4FeedContractErr = json.Unmarshal(p4FeedContractJSON, &p4FeedContractData)
		if p4FeedContractErr != nil {
			return
		}
		if p4FeedContractData.SchemaVersion == "" ||
			p4FeedContractData.RulesVersion != manjuSkillVersion ||
			len(p4FeedContractData.Sources) == 0 ||
			len(p4FeedContractData.Sections) == 0 ||
			len(p4FeedContractData.Validator.RequiredArgs) == 0 ||
			p4FeedContractData.StyleLock.InjectPosition == "" ||
			p4FeedContractData.StyleLock.StyleMarker == "" ||
			p4FeedContractData.StyleLock.PerformanceMarker == "" ||
			len(p4FeedContractData.DeferredContractIssues.Codes) == 0 ||
			len(p4FeedContractData.RepairRoutes) == 0 {
			p4FeedContractErr = errors.New("P4 投喂共享输出契约不完整")
		}
	})
	return p4FeedContractData, p4FeedContractErr
}

func mustLoadP3StoryboardContract() P3StoryboardContract {
	contract, err := loadP3StoryboardContract()
	if err != nil {
		panic(err)
	}
	return contract
}

func mustLoadP4FeedContract() P4FeedContract {
	contract, err := loadP4FeedContract()
	if err != nil {
		panic(err)
	}
	return contract
}

func p3StoryboardPrompt() string {
	return p3StoryboardPromptWithLensRange(nil)
}

func p3StoryboardPromptWithLensRange(lensRange *styleLensRange) string {
	contract := mustLoadP3StoryboardContract()
	var out strings.Builder
	out.WriteString(contract.Prompt.Heading)
	out.WriteString("\n")
	for _, item := range contract.Prompt.RequiredItems {
		out.WriteString("- ")
		out.WriteString(item)
		out.WriteString("\n")
	}
	out.WriteString("\n【C9 平台高危词硬门】\n")
	out.WriteString(contract.Safety.Instruction)
	out.WriteString("\n高危词清单：")
	out.WriteString(strings.Join(contract.Safety.HighRiskWords, "、"))
	out.WriteString("\n")
	out.WriteString(contract.Budget.Instruction)
	if lensRange != nil {
		fmt.Fprintf(
			&out,
			"\n【P0 画风焦段硬锁】允许区间 `[%dmm, %dmm]`。%s\n",
			lensRange.Min,
			lensRange.Max,
			contract.StyleConstraint.InstructionTemplate,
		)
	}
	return out.String()
}

func p3ProjectStateInstruction() string {
	contract := mustLoadP3StoryboardContract()
	example, err := json.MarshalIndent(contract.ProjectState.Example, "", "  ")
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf(`

【BeefTV 项目状态传输块】
阶段键：%s

<!-- MANJU_PROJECT_STATE_PATCH_JSON
%s
-->
不得写入 locks 或 receipt。`,
		contract.ProjectState.StageKey,
		example,
	)
}

func styleLensRangeForSource(styleSource string) (styleLensRange, bool, error) {
	styleSource = strings.TrimSpace(styleSource)
	if styleSource == "" {
		return styleLensRange{}, false, nil
	}
	blocks, found, err := defaultManjuSkillRuntime.officialStyleLockBlocks(styleSource)
	if err != nil || !found {
		return styleLensRange{}, false, err
	}
	match := styleLensRangePattern.FindStringSubmatch(blocks)
	if len(match) != 3 {
		return styleLensRange{}, false, nil
	}
	minLens, err := strconv.Atoi(match[1])
	if err != nil {
		return styleLensRange{}, false, err
	}
	maxLens, err := strconv.Atoi(match[2])
	if err != nil {
		return styleLensRange{}, false, err
	}
	if minLens > maxLens {
		minLens, maxLens = maxLens, minLens
	}
	return styleLensRange{Min: minLens, Max: maxLens, Source: styleSource}, true, nil
}

func p4FeedPrompt(assetMode string) string {
	contract := mustLoadP4FeedContract()
	assetMode = strings.ToUpper(strings.TrimSpace(assetMode))
	var out strings.Builder
	out.WriteString("任务：输出 P4 投喂封装。")
	if assetMode == AssetModeM0Value {
		out.WriteString("第一行必须声明 `> 素材形态：M0（纯提示词 · 不使用参考图）`。")
	} else {
		out.WriteString("第一行必须声明 `> 素材形态：M1（编号占位 · 图号见接线单）` 或 `> 素材形态：M2（已入库 · 可图像级核验）`。")
	}
	out.WriteString("\n逐段投喂块必须使用契约七段式：")
	out.WriteString(strings.Join(contract.Sections, " → "))
	out.WriteString("。\n")
	out.WriteString("镜行必须同时写出")
	out.WriteString(strings.Join(contract.ShotLine.RequiredElements, " · "))
	out.WriteString("，并包含行内 `")
	out.WriteString(contract.ShotLine.SoundField)
	out.WriteString("` 与 `")
	out.WriteString(contract.ShotLine.DialogueField)
	out.WriteString("`；无台词固定写 `")
	out.WriteString(contract.ShotLine.EmptyDialogue)
	out.WriteString("`。\n")
	fmt.Fprintf(
		&out,
		"画风块位置固定为 `%s`，每个投喂单元都必须包含 `%s` 与 `%s`，二者必须有可解析的光比与镜头区间。\n",
		contract.StyleLock.InjectPosition,
		contract.StyleLock.StyleMarker,
		contract.StyleLock.PerformanceMarker,
	)
	if assetMode == AssetModeM0Value {
		out.WriteString("M0 禁止出现任何 `图N`、参考图槽位或资源身份。\n")
	} else {
		out.WriteString("M1 必须附《素材接线单》，图号与正文双向一致；若上方存在《P3 已锁定故事板图资源映射》，必须按段引用其中的真实 resourceId，不得另造图号或资源身份。\n")
	}
	out.WriteString("必须包含 `# P4 投喂封装`、`## 素材接线单`（M0 除外）、`## 台词完整性`、`## 六道闸门`；不得新增、删除或改写 P3 台词。\n")
	fmt.Fprintf(
		&out,
		"BeefTV 预检与官方门禁顺序：`%s` → `%s` → `%s`；校验参数固定包含 %s，任何 FAIL 均关闭通过。",
		contract.ExecutionOrder[0],
		contract.ExecutionOrder[1],
		contract.ExecutionOrder[2],
		strings.Join(contract.Validator.RequiredArgs, " "),
	)
	return out.String()
}

func p4ValidateArgs(contentPath, statePath string, options ManjuSkillValidationOptions) []string {
	contract := mustLoadP4FeedContract()
	model := normalizeManjuSkillModel(options.Model)
	track := fallbackValue(options.Track, "1")
	combat := normalizeP4Combat(options.Combat)
	args := []string{
		contentPath,
		"--model", model,
		"--track", track,
		"--combat", combat,
		"--mode", contract.Validator.OutputMode,
	}
	if strings.TrimSpace(options.Aspect) != "" {
		args = append(args, "--aspect", options.Aspect)
	}
	if strings.TrimSpace(options.StyleSource) != "" {
		args = append(args, "--style-source", options.StyleSource)
	}
	if statePath != "" {
		args = append(args, "--state", statePath)
	}
	args = append(args, contract.Validator.RequiredArgs...)
	return args
}

func normalizeP4Combat(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "R1", "R2", "R3", "ON":
		return "on"
	case "NONE", "OFF", "":
		return "auto"
	default:
		return value
	}
}

func p4StyleLockMarkers() (styleMarker, performanceMarker, position string, err error) {
	contract, err := loadP4FeedContract()
	if err != nil {
		return "", "", "", err
	}
	return contract.StyleLock.StyleMarker,
		contract.StyleLock.PerformanceMarker,
		contract.StyleLock.InjectPosition,
		nil
}

func p4RepairStageForIssue(issue string) string {
	contract := mustLoadP4FeedContract()
	codes := make([]string, 0, len(contract.RepairRoutes))
	for code := range contract.RepairRoutes {
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool {
		if len(codes[i]) == len(codes[j]) {
			return codes[i] < codes[j]
		}
		return len(codes[i]) > len(codes[j])
	})
	for _, code := range codes {
		if strings.Contains(issue, code) {
			return contract.RepairRoutes[code]
		}
	}
	return ""
}

func splitP4DeferredContractIssues(issues []string) (active, deferred []string) {
	contract := mustLoadP4FeedContract()
	for _, issue := range issues {
		isDeferred := false
		for _, code := range contract.DeferredContractIssues.Codes {
			if strings.Contains(issue, code) {
				deferred = append(deferred, issue)
				isDeferred = true
				break
			}
		}
		if !isDeferred {
			active = append(active, issue)
		}
	}
	return active, deferred
}

func decodeP4ValidationOutput(output string) (p4ValidationOutput, bool) {
	var result p4ValidationOutput
	found := false
	searchFrom := 0
	for searchFrom < len(output) {
		offset := strings.IndexByte(output[searchFrom:], '{')
		if offset < 0 {
			break
		}
		start := searchFrom + offset
		var candidate p4ValidationOutput
		decoder := json.NewDecoder(strings.NewReader(output[start:]))
		if err := decoder.Decode(&candidate); err == nil &&
			(candidate.Issues != nil || candidate.Warnings != nil) {
			result = candidate
			found = true
		}
		searchFrom = start + 1
	}
	return result, found
}

func p4RepairRouteInstruction(issues ...string) string {
	routes := make([]string, 0, len(issues))
	seen := make(map[string]bool)
	for _, issue := range issues {
		stage := p4RepairStageForIssue(issue)
		if stage == "" || seen[stage] {
			continue
		}
		seen[stage] = true
		routes = append(routes, stage)
	}
	if len(routes) == 0 {
		return ""
	}
	return "修复路由：" + strings.Join(routes, " / ")
}
