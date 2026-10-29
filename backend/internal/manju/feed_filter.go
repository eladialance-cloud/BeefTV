package manju

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var (
	manjuUnitHeadingPattern     = regexp.MustCompile(`(?m)^## 【投喂单元\s+([0-9]+)/([0-9]+)\s+·\s+([^】]+)】\s*$`)
	manjuSegmentNumberPattern   = regexp.MustCompile(`段\s*0?([0-9]+)`)
	manjuStateSegmentPattern    = regexp.MustCompile(`(?i)(?:段|seg(?:ment)?|s)\s*[_\-]?\s*0?([0-9]+)`)
	manjuStateRangePattern      = regexp.MustCompile(`(?i)(?:段|seg(?:ment)?|s)\s*[_\-]?\s*0?([0-9]+)\s*(?:-|~|～|—|→|至|到|起至)\s*(?:段|seg(?:ment)?|s)?\s*[_\-]?\s*0?([0-9]+)`)
	manjuFigureReferencePattern = regexp.MustCompile(`图\s*[0-9]+`)
	manjuAspectRatioSuffix      = regexp.MustCompile(`[，,]\s*aspect_ratio=[^）)\n]+|（\s*aspect_ratio=[^）)\n]+）`)
)

type manjuFeedFigure struct {
	Number         int
	Name           string
	Control        string
	DoesNotControl string
}

type manjuNamedCandidate struct {
	Name     string
	Value    map[string]any
	Position int
}

func filterCompiledManjuFeed(feed, stateJSON string) (string, error) {
	var state map[string]any
	if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
		return "", fmt.Errorf("解析 P4 编译状态失败: %w", err)
	}
	segments := anySlice(state["segments"])
	if len(segments) == 0 {
		return feed, nil
	}
	mode := strings.ToUpper(fallbackValue(anyString(anyMap(state["locks"])["asset_mode"]), AssetModeM1Value))
	characters := anyMap(state["characters"])
	scenes := anyMap(state["scenes"])
	props := anyMap(state["props"])

	mainFeed := feed
	wiring := ""
	if index := strings.Index(feed, "\n## 素材接线单"); index >= 0 {
		mainFeed = feed[:index]
		wiring = feed[index:]
	}
	matches := manjuUnitHeadingPattern.FindAllStringSubmatchIndex(mainFeed, -1)
	if len(matches) == 0 {
		return feed, nil
	}

	usedFigures := make(map[int]manjuFeedFigure)
	var out strings.Builder
	out.WriteString(mainFeed[:matches[0][0]])
	for index, match := range matches {
		segmentNumber := 0
		segmentID := mainFeed[match[6]:match[7]]
		if parsed := manjuSegmentNumberPattern.FindStringSubmatch(segmentID); len(parsed) > 1 {
			segmentNumber, _ = strconv.Atoi(parsed[1])
		}
		sectionStart := match[0]
		sectionEnd := len(mainFeed)
		if index+1 < len(matches) {
			sectionEnd = matches[index+1][0]
		}
		section := mainFeed[sectionStart:sectionEnd]
		codeStart := strings.Index(section, "```text")
		if codeStart < 0 {
			out.WriteString(section)
			continue
		}
		contentStart := codeStart + len("```text")
		if newline := strings.IndexByte(section[contentStart:], '\n'); newline >= 0 {
			contentStart += newline + 1
		}
		contentEnd := strings.Index(section[contentStart:], "```")
		if contentEnd < 0 {
			out.WriteString(section)
			continue
		}
		contentEnd += contentStart
		segment := map[string]any{}
		if segmentNumber > 0 && segmentNumber <= len(segments) {
			segment = anyMap(segments[segmentNumber-1])
		}
		unitContent := section[contentStart:contentEnd]
		segment = manjuSegmentForUnitContent(segment, unitContent)
		filtered, figures := filterManjuUnitContent(
			unitContent, segment, segmentNumber, mode, characters, scenes, props,
		)
		for number, figure := range figures {
			usedFigures[number] = figure
		}
		out.WriteString(section[:contentStart])
		out.WriteString(filtered)
		out.WriteString(section[contentEnd:])
	}
	if mode == AssetModeM1Value {
		wiring = renderManjuWiring(usedFigures)
	}
	if strings.TrimSpace(wiring) != "" {
		out.WriteString("\n")
		out.WriteString(strings.TrimSpace(wiring))
		out.WriteString("\n")
	}
	return out.String(), nil
}

func manjuSegmentForUnitContent(segment map[string]any, content string) map[string]any {
	shots := anySlice(segment["shots"])
	if len(shots) <= 1 {
		return segment
	}
	matched := make([]any, 0, len(shots))
	for _, raw := range shots {
		shot := anyMap(raw)
		if manjuShotAppearsInUnit(shot, content) {
			matched = append(matched, shot)
		}
	}
	if len(matched) == 0 || len(matched) == len(shots) {
		return segment
	}
	clone := make(map[string]any, len(segment))
	for key, value := range segment {
		clone[key] = value
	}
	clone["shots"] = matched
	// In a shot-only split unit, standing from the full segment would re-add every character.
	clone["standing"] = []any{}
	return clone
}

func manjuShotAppearsInUnit(shot map[string]any, content string) bool {
	action := strings.Trim(strings.TrimSpace(anyString(shot["action"])), "。；;，, ")
	runes := []rune(action)
	if len(runes) > 0 {
		if strings.Contains(content, action) {
			return true
		}
		headLength := len(runes)
		if headLength > 14 {
			headLength = 14
		}
		if headLength >= 6 && strings.Contains(content, string(runes[:headLength])) {
			return true
		}
	}
	dialogue := strings.Trim(strings.TrimSpace(anyString(shot["dialogue"])), "。；;，, ")
	return dialogue != "" && dialogue != "无" && strings.Contains(content, dialogue)
}

func filterManjuUnitContent(
	content string,
	segment map[string]any,
	segmentNumber int,
	mode string,
	characters, scenes, props map[string]any,
) (string, map[int]manjuFeedFigure) {
	context := manjuSegmentContext(segment)
	figures := make(map[int]manjuFeedFigure)

	selectedScenes := matchManjuScenes(scenes, context, anyString(segment["scene"]))
	if len(selectedScenes) > 2 {
		selectedScenes = selectedScenes[:2]
	}
	sceneParts := make([]string, 0, len(selectedScenes))
	for _, candidate := range selectedScenes {
		part, figure := manjuSceneReference(candidate.Name, candidate.Value, mode)
		sceneParts = append(sceneParts, part)
		if figure != nil && len(figures) < 9 {
			figures[figure.Number] = *figure
		}
	}

	selectedCharacters := matchManjuCharacters(characters, context, segmentNumber)
	characterParts := make([]string, 0, len(selectedCharacters))
	for _, candidate := range selectedCharacters {
		part, figure := manjuCharacterReference(candidate.Name, candidate.Value, mode, segmentNumber)
		if figure != nil && len(figures) >= 9 && !manjuFigureExists(figures, figure.Number) {
			continue
		}
		characterParts = append(characterParts, part)
		if figure != nil {
			figures[figure.Number] = *figure
		}
	}

	selectedProps := matchManjuNamedCandidates(props, context)
	for _, candidate := range selectedProps {
		part, figure := manjuPropReference(candidate.Name, candidate.Value, mode)
		if figure != nil && len(figures) >= 9 && !manjuFigureExists(figures, figure.Number) {
			continue
		}
		sceneParts = append(sceneParts, part)
		if figure != nil {
			figures[figure.Number] = *figure
		}
	}

	cleaned := stripManjuAspectRatioFromStyleLine(content)
	if len(sceneParts) > 0 {
		cleaned = replaceManjuFeedLine(cleaned, "【场景资产】", "【场景资产】"+strings.Join(sceneParts, "；"))
	}
	characterLine := "无（本单元无人入画）"
	if len(characterParts) > 0 {
		characterLine = strings.Join(characterParts, "；")
	}
	cleaned = replaceManjuFeedLine(cleaned, "【核心人物】", "【核心人物】"+characterLine)
	return cleaned, figures
}

func stripManjuAspectRatioFromStyleLine(content string) string {
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "【画幅风格】") {
			lines[index] = manjuAspectRatioSuffix.ReplaceAllString(line, "")
		}
	}
	return strings.Join(lines, "\n")
}

func manjuSegmentContext(segment map[string]any) string {
	parts := []string{
		anyString(segment["seg_id"]),
		anyString(segment["scene"]),
		anyString(segment["segment_type"]),
		anyString(segment["sfx_summary"]),
	}
	for _, value := range anySlice(segment["standing"]) {
		parts = append(parts, anyString(value))
	}
	for _, value := range anySlice(segment["storyboard_panels"]) {
		parts = append(parts, anyString(value))
	}
	for _, raw := range anySlice(segment["shots"]) {
		shot := anyMap(raw)
		for _, key := range []string{"spec", "action", "sound", "dialogue", "optics_and_camera_lock"} {
			parts = append(parts, anyString(shot[key]))
		}
	}
	return strings.Join(parts, "\n")
}

func productionSegmentVisualAssetCodes(state, segment map[string]any, limit int) []string {
	if limit <= 0 {
		limit = 9
	}
	segmentNumber := 0
	if match := manjuSegmentNumberPattern.FindStringSubmatch(anyString(segment["seg_id"])); len(match) > 1 {
		segmentNumber, _ = strconv.Atoi(match[1])
	}
	context := manjuSegmentContext(segment)
	result := make([]string, 0, limit)
	seen := make(map[string]bool)
	add := func(code string) bool {
		code = strings.TrimSpace(code)
		normalized := visualCanvasCodes(code)
		if code == "" || len(normalized) == 0 || seen[normalized[0]] {
			return len(result) < limit
		}
		code = normalized[0]
		seen[code] = true
		result = append(result, code)
		return len(result) < limit
	}
	for _, candidate := range matchManjuCharacters(anyMap(state["characters"]), context, segmentNumber) {
		if !add(anyString(candidate.Value["id"])) {
			return result
		}
	}
	selectedScenes := matchManjuNamedCandidates(anyMap(state["scenes"]), context+"\n"+anyString(segment["scene"]))
	if len(selectedScenes) > 2 {
		selectedScenes = selectedScenes[:2]
	}
	for _, candidate := range selectedScenes {
		if !add(anyString(candidate.Value["id"])) {
			return result
		}
	}
	for _, candidate := range matchManjuNamedCandidates(anyMap(state["props"]), context) {
		if !add(anyString(candidate.Value["id"])) {
			return result
		}
	}
	return result
}

func matchManjuCharacters(characters map[string]any, context string, segmentNumber int) []manjuNamedCandidate {
	candidates := make([]manjuNamedCandidate, 0)
	for name, raw := range characters {
		character := anyMap(raw)
		position := manjuCharacterAliasPosition(context, name)
		if position < 0 {
			if id := strings.TrimSpace(anyString(character["id"])); len([]rune(id)) >= 2 {
				position = strings.Index(context, id)
			}
		}
		if position < 0 {
			continue
		}
		candidates = append(candidates, manjuNamedCandidate{Name: name, Value: character, Position: position})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Position == candidates[j].Position {
			return candidates[i].Name < candidates[j].Name
		}
		return candidates[i].Position < candidates[j].Position
	})
	return candidates
}

func manjuCharacterAliasPosition(context, name string) int {
	if position := manjuAliasPosition(context, name); position >= 0 {
		return position
	}
	runes := []rune(strings.TrimSpace(name))
	if len(runes) < 3 {
		return -1
	}
	// P3 台账通常保存完整姓名，而镜行会使用去掉姓氏的常用称呼。
	return strings.Index(context, string(runes[1:]))
}

func matchManjuNamedCandidates(items map[string]any, context string) []manjuNamedCandidate {
	candidates := make([]manjuNamedCandidate, 0)
	for name, raw := range items {
		position := manjuAliasPosition(context, name)
		if position < 0 {
			continue
		}
		candidates = append(candidates, manjuNamedCandidate{Name: name, Value: anyMap(raw), Position: position})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Position == candidates[j].Position {
			return candidates[i].Name < candidates[j].Name
		}
		return candidates[i].Position < candidates[j].Position
	})
	return candidates
}

func matchManjuScenes(items map[string]any, context, sceneName string) []manjuNamedCandidate {
	if exact := matchManjuNamedCandidates(items, context+"\n"+sceneName); len(exact) > 0 {
		return exact
	}
	sceneName = strings.TrimSpace(sceneName)
	if sceneName == "" {
		return nil
	}
	bestScore := 0
	candidates := make([]manjuNamedCandidate, 0)
	for name, raw := range items {
		score := manjuLongestCommonSubstring([]rune(sceneName), []rune(name))
		if score < 2 || score < bestScore {
			continue
		}
		if score > bestScore {
			bestScore = score
			candidates = candidates[:0]
		}
		candidates = append(candidates, manjuNamedCandidate{Name: name, Value: anyMap(raw), Position: score})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Position == candidates[j].Position {
			return candidates[i].Name < candidates[j].Name
		}
		return candidates[i].Position > candidates[j].Position
	})
	if len(candidates) > 1 {
		candidates = candidates[:1]
	}
	return candidates
}

func manjuLongestCommonSubstring(left, right []rune) int {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	previous := make([]int, len(right)+1)
	current := make([]int, len(right)+1)
	best := 0
	for _, leftRune := range left {
		for index, rightRune := range right {
			if leftRune != rightRune {
				current[index+1] = 0
				continue
			}
			current[index+1] = previous[index] + 1
			if current[index+1] > best {
				best = current[index+1]
			}
		}
		previous, current = current, previous
		for index := range current {
			current[index] = 0
		}
	}
	return best
}

func manjuAliasPosition(context, name string) int {
	best := -1
	for _, alias := range manjuNameAliases(name) {
		if index := strings.Index(context, alias); index >= 0 && (best < 0 || index < best) {
			best = index
		}
	}
	return best
}

func manjuNameAliases(name string) []string {
	aliases := []string{strings.TrimSpace(name)}
	for _, group := range regexp.MustCompile(`[（(]([^）)]+)[）)]`).FindAllStringSubmatch(name, -1) {
		aliases = append(aliases, group[1])
	}
	if index := strings.IndexAny(name, "（("); index >= 0 {
		aliases = append(aliases, name[:index])
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(aliases))
	for _, value := range aliases {
		for _, token := range splitManjuAliases(value) {
			token = strings.TrimSpace(token)
			if len([]rune(token)) < 2 || seen[token] {
				continue
			}
			seen[token] = true
			result = append(result, token)
		}
	}
	return result
}

func splitManjuAliases(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case '/', '、', '，', ',', '；', ';', ' ', '与', '及', '和', '·':
			return true
		}
		return unicode.IsSpace(r)
	})
}

func manjuCharacterReference(name string, character map[string]any, mode string, segmentNumber int) (string, *manjuFeedFigure) {
	figureNumber := int(anyNumber(character["fig_no"]))
	voice := fallbackValue(anyString(character["voice_lock"]), "未提供，未伪造")
	devParts := make([]string, 0)
	for stateKey, raw := range anyMap(character["char_states"]) {
		state := anyMap(raw)
		if !manjuCharacterStateApplies(stateKey, state, segmentNumber) {
			continue
		}
		item := strings.TrimSpace(manjuFigureReferencePattern.ReplaceAllString(anyString(state["item"]), ""))
		if item == "" {
			continue
		}
		action := strings.TrimSpace(manjuFigureReferencePattern.ReplaceAllString(anyString(state["action"]), ""))
		consequence := strings.TrimSpace(manjuFigureReferencePattern.ReplaceAllString(anyString(state["consequence"]), ""))
		devParts = append(devParts, fmt.Sprintf(
			"⚠️资产状态偏离声明：本段%s%s（%s），关于「%s」的状态以本段提示词为准，覆盖角色参考图",
			name, action, consequence, item,
		))
	}
	dev := ""
	if len(devParts) > 0 {
		dev = "；" + strings.Join(devParts, "；")
	}
	if mode != AssetModeM0Value && figureNumber > 0 {
		return fmt.Sprintf("@//chr/ - %s（图%d，声线锁：%s%s）", name, figureNumber, voice, dev), &manjuFeedFigure{
			Number: figureNumber, Name: "角色·" + name,
			Control:        "锁定" + name + "面部五官、头身比与基础服装形制",
			DoesNotControl: "不控制本段动作姿态与已声明偏离的配饰状态",
		}
	}
	return fmt.Sprintf(
		"@//chr/ - %s：%s，%s，%s（同一张脸跨镜锁定），声线锁：%s%s",
		name,
		fallbackValue(anyString(character["age_anchor"]), "年龄段未提供"),
		fallbackValue(anyString(character["body_anchor"]), "体态未提供"),
		fallbackValue(anyString(character["look_3piece"]), "身份外观未提供"),
		voice, dev,
	), nil
}

func manjuSceneReference(name string, scene map[string]any, mode string) (string, *manjuFeedFigure) {
	figureNumber := int(anyNumber(scene["fig_no"]))
	lighting := strings.TrimSpace(anyString(scene["lighting"]))
	base := ""
	if mode != AssetModeM0Value && figureNumber > 0 {
		base = fmt.Sprintf("@//scn/ - %s（图%d）", name, figureNumber)
	} else {
		base = fmt.Sprintf("@//scn/ - %s（%s；%s；%s）", name,
			anyString(scene["coords"]), anyString(anyMap(scene["set_states"])["default"]), anyString(scene["media_layer"]))
	}
	if lighting != "" {
		base += "；时段光影：" + lighting
	}
	if figureNumber <= 0 {
		return base, nil
	}
	return base, &manjuFeedFigure{
		Number: figureNumber, Name: "场景·" + name,
		Control: "锁定空间几何、建筑材质与主光源方向", DoesNotControl: "不控制人物站位与动作",
	}
}

func manjuPropReference(name string, prop map[string]any, mode string) (string, *manjuFeedFigure) {
	figureNumber := int(anyNumber(prop["fig_no"]))
	if mode != AssetModeM0Value && figureNumber > 0 {
		return fmt.Sprintf("@//prp/ - %s（图%d）", name, figureNumber), &manjuFeedFigure{
			Number: figureNumber, Name: "道具·" + name,
			Control: "锁定" + name + "外观形制与材质", DoesNotControl: "不控制持握人物与运动轨迹",
		}
	}
	return fmt.Sprintf("@//prp/ - %s（%s）", name, anyString(prop["material_desc"])), nil
}

func manjuCharacterStateApplies(stateKey string, state map[string]any, segmentNumber int) bool {
	if segmentNumber <= 0 {
		return false
	}
	startSegment := int(anyNumber(state["startSegment"]))
	endSegment := int(anyNumber(state["endSegment"]))
	if startSegment <= 0 {
		startSegment = int(anyNumber(state["fromSegment"]))
	}
	if endSegment <= 0 {
		endSegment = int(anyNumber(state["toSegment"]))
	}
	if startSegment > 0 {
		return segmentNumber >= startSegment && (endSegment <= 0 || segmentNumber <= endSegment)
	}

	scope := strings.Join([]string{
		stateKey,
		anyString(state["scope"]),
		anyString(state["range"]),
		anyString(state["effectiveRange"]),
		anyString(state["item"]),
		anyString(state["action"]),
		anyString(state["consequence"]),
	}, "\n")
	for _, match := range manjuStateRangePattern.FindAllStringSubmatch(scope, -1) {
		start, startErr := strconv.Atoi(match[1])
		end, endErr := strconv.Atoi(match[2])
		if startErr == nil && endErr == nil {
			if start > end {
				start, end = end, start
			}
			return segmentNumber >= start && segmentNumber <= end
		}
	}
	numbers := make([]int, 0, 4)
	for _, match := range manjuStateSegmentPattern.FindAllStringSubmatch(scope, -1) {
		value, err := strconv.Atoi(match[1])
		if err == nil && value > 0 {
			numbers = append(numbers, value)
		}
	}
	if len(numbers) == 0 {
		return false
	}
	persistentFrom := strings.Contains(scope, "起") || strings.Contains(scope, "持续")
	if persistentFrom {
		minimum := numbers[0]
		for _, value := range numbers[1:] {
			if value < minimum {
				minimum = value
			}
		}
		return segmentNumber >= minimum
	}
	for _, value := range numbers {
		if value == segmentNumber {
			return true
		}
	}
	return false
}

func manjuFigureExists(figures map[int]manjuFeedFigure, number int) bool {
	_, ok := figures[number]
	return ok
}

func replaceManjuFeedLine(content, marker, replacement string) string {
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), marker) {
			lines[index] = replacement
		}
	}
	return strings.Join(lines, "\n")
}

func renderManjuWiring(figures map[int]manjuFeedFigure) string {
	if len(figures) == 0 {
		return ""
	}
	items := make([]manjuFeedFigure, 0, len(figures))
	for _, figure := range figures {
		items = append(items, figure)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Number < items[j].Number })
	var out strings.Builder
	out.WriteString("## 素材接线单（M1 编号占位必备 · 平台挂图指南）\n\n")
	out.WriteString("| 图号 | 对应资产 | 你该挂哪张 | 只控制什么 | 不控制什么 |\n")
	out.WriteString("|---|---|---|---|---|\n")
	for _, figure := range items {
		fmt.Fprintf(&out, "| 图%d | %s | 挂载 P2 产出的「%s」设定图 | %s | %s |\n",
			figure.Number, figure.Name, figure.Name, figure.Control, figure.DoesNotControl)
	}
	return out.String()
}

func anyNumber(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case json.Number:
		result, _ := typed.Float64()
		return result
	case string:
		result, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return result
	default:
		return 0
	}
}
