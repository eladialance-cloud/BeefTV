package manju

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var (
	p3OfficialSegmentHeading = regexp.MustCompile(`(?m)^###\s+段([1-9][0-9]*)(?:\s*[｜|]\s*(.*?))?\s*$`)
	p3OfficialShotHeading    = regexp.MustCompile(`(?m)^#{3,6}\s+镜头\s*([0-9]+)\s*\|\s*(.+?)\s*$`)
	p3OfficialSegmentSeconds = regexp.MustCompile(`(?:^|[｜|])\s*([1-9][0-9]*)\s*s\s*$`)
	p3OfficialShotField      = regexp.MustCompile(`^([^：:\n]{1,16})[：:]\s*(.*)$`)
	p3OfficialCleanGridLine  = regexp.MustCompile(`^\s*[0-9]+\.\s+\*\*\s*段([1-9][0-9]*)\s+CLEAN_GRID\s*[｜|]\s*([0-9]+)\s*格[^*]*\*\*\s*[：:]\s*(.+)$`)
	p3OfficialPanelMarker    = regexp.MustCompile(`面板\s*([0-9]+)\s*（([^）]+)）`)
)

var p3OfficialShotFields = []string{
	"场景",
	"画面内容",
	"状态",
	"光影",
	"色调",
	"台词",
	"重音",
	"人声",
	"音效",
}

type p3OfficialSegmentBlock struct {
	Code        string
	Heading     string
	SegmentType string
	Seconds     int
	Shots       []map[string]any
	RawShots    string
}

func compileP3OfficialStoryboardState(content string, baseState map[string]any) (map[string]any, error) {
	blocks, err := parseP3OfficialStoryboard(content, baseState)
	if err != nil {
		return nil, err
	}
	segments := make([]any, 0, len(blocks))
	for _, block := range blocks {
		shotValues := make([]any, 0, len(block.Shots))
		sceneRefs := make([]string, 0, 2)
		characterRefs := make([]string, 0, len(block.Shots))
		propRefs := make([]string, 0, len(block.Shots))
		for _, shot := range block.Shots {
			refs := anyMap(shot["_asset_refs"])
			sceneRefs = append(sceneRefs, stringList(refs["scenes"])...)
			characterRefs = append(characterRefs, stringList(refs["characters"])...)
			propRefs = append(propRefs, stringList(refs["props"])...)
			copyShot := make(map[string]any, len(shot))
			for key, value := range shot {
				if key != "_asset_refs" {
					copyShot[key] = value
				}
			}
			shotValues = append(shotValues, copyShot)
		}
		segments = append(segments, map[string]any{
			"seg_id":       block.Code,
			"scene":        block.Shots[0]["scene"],
			"segment_type": block.SegmentType,
			"asset_refs": map[string]any{
				"scenes":           uniqueAnyStrings(sceneRefs),
				"characters":       uniqueAnyStrings(characterRefs),
				"props":            uniqueAnyStrings(propRefs),
				"character_states": map[string]any{},
			},
			"shots": shotValues,
		})
	}
	return map[string]any{"segments": segments}, nil
}

func uniqueAnyStrings(values []string) []any {
	seen := make(map[string]bool, len(values))
	result := make([]any, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func parseP3OfficialStoryboard(content string, baseState map[string]any) ([]p3OfficialSegmentBlock, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	segmentMatches := p3OfficialSegmentHeading.FindAllStringSubmatchIndex(normalized, -1)
	if len(segmentMatches) == 0 {
		return nil, fmt.Errorf("%w: 未找到 V7.4.26 官方 ### 段N 分镜小节", ErrP3StoryboardContract)
	}
	characters := anyMap(baseState["characters"])
	scenes := anyMap(baseState["scenes"])
	props := anyMap(baseState["props"])
	blocks := make([]p3OfficialSegmentBlock, 0, len(segmentMatches))
	seen := make(map[string]bool, len(segmentMatches))
	for index, match := range segmentMatches {
		code := "段" + normalized[match[2]:match[3]]
		if seen[code] {
			return nil, fmt.Errorf("%w: 官方分镜段号重复 %s", ErrP3StoryboardContract, code)
		}
		seen[code] = true
		end := len(normalized)
		if index+1 < len(segmentMatches) {
			end = segmentMatches[index+1][0]
		}
		heading := strings.TrimSpace(normalized[match[0]:match[1]])
		blockText := normalized[match[1]:end]
		seconds, err := p3OfficialStoryboardDuration(heading)
		if err != nil {
			return nil, fmt.Errorf("%w: %s 标题时长无效: %v", ErrP3StoryboardContract, code, err)
		}
		shots, rawShots, err := parseP3OfficialStoryboardShots(blockText, characters, scenes, props)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrP3StoryboardContract, code, err)
		}
		if len(shots) == 0 {
			return nil, fmt.Errorf("%w: %s 缺少 ###/#### 镜头 K", ErrP3StoryboardContract, code)
		}
		if seconds > 0 && seconds < len(shots) {
			return nil, fmt.Errorf(
				"%w: %s 的 %d 秒无法拆成 %d 个整秒镜头",
				ErrP3StoryboardContract,
				code,
				seconds,
				len(shots),
			)
		}
		if err := assignP3OfficialShotTimeline(shots, seconds); err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrP3StoryboardContract, code, err)
		}
		blocks = append(blocks, p3OfficialSegmentBlock{
			Code:        code,
			Heading:     heading,
			SegmentType: p3OfficialSegmentType(heading, shots, characters),
			Seconds:     seconds,
			Shots:       shots,
			RawShots:    rawShots,
		})
	}
	return blocks, nil
}

func p3OfficialSegmentType(
	heading string,
	shots []map[string]any,
	characters map[string]any,
) string {
	// V7.4.26 C27 checks explicit non-dialogue intent before dialogue intent.
	nonDialogueTypes := []struct {
		label   string
		markers []string
	}{
		{label: "战斗动作段", markers: []string{"战斗动作段", "战斗", "打斗", "武戏"}},
		{label: "动作段", markers: []string{"动作段"}},
		{label: "空镜段", markers: []string{"空镜"}},
		{label: "转场段", markers: []string{"转场"}},
		{label: "蒙太奇", markers: []string{"蒙太奇"}},
		{label: "独白段", markers: []string{"独白"}},
		{label: "静场段", markers: []string{"静场"}},
		{label: "宣判段", markers: []string{"宣判"}},
		{label: "留白段", markers: []string{"留白"}},
		{label: "氛围段", markers: []string{"氛围段"}},
		{label: "长镜段", markers: []string{"长镜段"}},
		{label: "情绪积累段", markers: []string{"情绪积累"}},
	}
	for _, candidate := range nonDialogueTypes {
		if containsAnyText(heading, candidate.markers) {
			return candidate.label
		}
	}
	if containsAnyText(heading, []string{"对话", "对白", "文戏", "谈判", "争执", "争吵", "商议", "聊"}) {
		return "对话戏"
	}

	hasDialogue := false
	hasVisualCharacter := false
	for _, shot := range shots {
		if dialogue := strings.TrimSpace(anyString(shot["dialogue"])); dialogue != "" &&
			dialogue != "无" && dialogue != "—" {
			hasDialogue = true
		}
		visualContent := anyString(shot["action"]) + "\n" + anyString(shot["state"])
		if len(p3ExactCatalogMatches(visualContent, characters)) > 0 {
			hasVisualCharacter = true
		}
	}
	if hasDialogue {
		return "对话戏"
	}
	if len(characters) == 0 || hasVisualCharacter {
		return ""
	}
	return "空镜段"
}

func p3OfficialStoryboardDuration(heading string) (int, error) {
	match := p3OfficialSegmentSeconds.FindStringSubmatch(strings.TrimSpace(heading))
	if len(match) != 2 {
		return 0, nil
	}
	seconds, err := strconv.Atoi(match[1])
	if err != nil || seconds <= 0 {
		return 0, fmt.Errorf("invalid duration %q", match[1])
	}
	return seconds, nil
}

func parseP3OfficialStoryboardShots(
	block string,
	characters, scenes, props map[string]any,
) ([]map[string]any, string, error) {
	matches := p3OfficialShotHeading.FindAllStringSubmatchIndex(block, -1)
	if len(matches) == 0 {
		return nil, "", nil
	}
	shots := make([]map[string]any, 0, len(matches))
	seen := make(map[int]bool, len(matches))
	var raw strings.Builder
	for index, match := range matches {
		number, _ := strconv.Atoi(block[match[2]:match[3]])
		if number <= 0 || seen[number] {
			return nil, "", fmt.Errorf("镜头号无效或重复 %d", number)
		}
		seen[number] = true
		end := len(block)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		headingTail := strings.TrimSpace(block[match[4]:match[5]])
		spec, err := p3OfficialShotSpec(headingTail)
		if err != nil {
			return nil, "", fmt.Errorf("镜头 %d 标题无效: %v", number, err)
		}
		fields, err := p3OfficialShotFieldValues(block[match[1]:end])
		if err != nil {
			return nil, "", fmt.Errorf("镜头 %d: %v", number, err)
		}
		sceneKeys, err := p3OfficialShotSceneKeys(fields["场景"], scenes)
		if err != nil {
			return nil, "", fmt.Errorf("镜头 %d: %v", number, err)
		}
		if len(sceneKeys) == 0 {
			return nil, "", fmt.Errorf("镜头 %d 缺场景", number)
		}
		action := strings.TrimSpace(fields["画面内容"])
		for _, label := range []string{"光影", "色调"} {
			if value := strings.TrimSpace(fields[label]); value != "" && value != "无" && value != "—" {
				action += "；" + label + "：" + value
			}
		}
		sound := fields["音效"]
		if voice := strings.TrimSpace(fields["人声"]); voice != "" && voice != "无" && voice != "—" {
			if sound == "" || sound == "无" {
				sound = voice
			} else {
				sound += "；" + voice
			}
		}
		searchText := strings.Join([]string{
			headingTail,
			fields["画面内容"],
			fields["状态"],
			fields["光影"],
			fields["色调"],
			fields["台词"],
			fields["重音"],
			fields["人声"],
			fields["音效"],
		}, "\n")
		duration, err := p3OfficialShotDuration(
			fields["画面内容"],
			fields["台词"],
			number-1,
			len(matches),
		)
		if err != nil {
			return nil, "", fmt.Errorf("镜头 %d: %v", number, err)
		}
		shot := map[string]any{
			"no":          number,
			"spec":        spec,
			"action":      action,
			"sound":       sound,
			"dialogue":    p3OfficialDialogueForFeed(fields["台词"]),
			"scene":       sceneKeys[0],
			"scenes":      uniqueAnyStrings(sceneKeys),
			"state":       fields["状态"],
			"lighting":    fields["光影"],
			"color":       fields["色调"],
			"emphasis":    fields["重音"],
			"voice":       fields["人声"],
			"_asset_refs": p3OfficialShotAssetRefs(sceneKeys, searchText, characters, props),
			"_duration":   duration,
		}
		shots = append(shots, shot)
		raw.WriteString(block[match[0]:end])
		raw.WriteString("\n")
	}
	return shots, raw.String(), nil
}

func assignP3OfficialShotTimeline(shots []map[string]any, targetSeconds int) error {
	cursor := 0
	total := 0
	for index, shot := range shots {
		duration := int(anyNumber(shot["_duration"]))
		if duration <= 0 {
			return fmt.Errorf("shot %d has no valid rule-derived duration", index+1)
		}
		shot["start"] = cursor
		shot["end"] = cursor + duration
		cursor += duration
		total += duration
		delete(shot, "_duration")
	}
	if targetSeconds > 0 && total > targetSeconds {
		return fmt.Errorf(
			"content-derived timeline is %ds, exceeding declared segment maximum %ds; split dialogue/action at shot boundaries",
			total,
			targetSeconds,
		)
	}
	return nil
}

func p3OfficialShotDuration(action, dialogue string, shotIndex, shotCount int) (int, error) {
	actionSeconds := p3OfficialActionSeconds(action, shotIndex, shotCount)
	dialogueSeconds, err := p3OfficialDialogueSeconds(dialogue)
	if err != nil {
		return 0, err
	}
	if dialogueSeconds == 0 {
		return actionSeconds, nil
	}
	return maxInt(actionSeconds, dialogueSeconds), nil
}

func p3OfficialActionSeconds(action string, shotIndex, shotCount int) int {
	action = strings.TrimSpace(action)
	if action == "" {
		return 3
	}
	seconds := 3
	if shotIndex == 0 && containsAnyText(action, []string{"前景", "中景", "背景", "远景"}) {
		seconds = 4
	}
	impactCount := 0
	for _, marker := range []string{
		"撞", "砸", "冲", "崩", "断", "劈", "刺", "踢", "推", "拉", "拖", "坠",
		"扑", "抓", "夺", "挥", "砸下", "碾", "爆", "翻上", "后仰", "栽倒",
	} {
		impactCount += strings.Count(action, marker)
	}
	switch {
	case impactCount >= 2:
		seconds = 5
	case impactCount == 1:
		seconds = maxInt(seconds, 4)
	}
	if shotIndex > 0 && shotIndex < shotCount-1 {
		seconds = maxInt(seconds, 4)
	}
	if shotIndex == shotCount-1 {
		if containsAnyText(action, []string{"停住", "望", "看", "保持", "静止", "垂落", "站在原地"}) && impactCount == 0 {
			seconds = 3
		} else {
			seconds = maxInt(seconds, 4)
		}
	}
	return minInt(5, seconds)
}

func p3OfficialDialogueSeconds(dialogue string) (int, error) {
	dialogue = strings.TrimSpace(dialogue)
	if dialogue == "" || dialogue == "无" || dialogue == "—" {
		return 0, nil
	}
	rate := 4.0
	slowMarkers := []string{"低语", "低声", "气音", "气声", "一字一顿", "断续", "拖长", "慢"}
	fastMarkers := []string{"急促", "连珠", "抢话", "快速", "快"}
	if containsAnyText(dialogue, slowMarkers) {
		rate = 3.5
	} else if containsAnyText(dialogue, fastMarkers) {
		rate = 6
	}
	text := p3OfficialSpokenDialogueText(dialogue)
	chars := 0
	commas := 0
	sentences := 0
	for _, char := range text {
		switch {
		case unicode.IsLetter(char), unicode.IsNumber(char):
			chars++
		case char == '，' || char == '、' || char == ',':
			commas++
		case char == '。' || char == '！' || char == '？' || char == '!' || char == '?':
			sentences++
		}
	}
	seconds := int(math.Ceil(float64(chars)/rate + float64(commas)*0.2 + float64(sentences)*0.5))
	if seconds < 3 {
		seconds = 3
	}
	if seconds > 6 {
		return 0, fmt.Errorf(
			"台词按 V7.4.26 语速与停顿预算需要 %ds，超过单镜 6s；必须在镜头边界拆镜，不得压缩台词或拉伸镜头",
			seconds,
		)
	}
	return seconds, nil
}

func p3OfficialSpokenDialogueText(dialogue string) string {
	var out strings.Builder
	for _, part := range strings.Split(dialogue, " / ") {
		part = strings.TrimSpace(part)
		if part == "" || part == "无" {
			continue
		}
		if start := strings.IndexAny(part, "“\""); start >= 0 {
			part = part[start:]
		} else if index := strings.IndexAny(part, "：:"); index >= 0 {
			part = part[index+p3ColonByteLen(part[index:]):]
		}
		out.WriteString(part)
		out.WriteString("。")
	}
	return out.String()
}

func p3OfficialDialogueForFeed(dialogue string) string {
	dialogue = strings.TrimSpace(dialogue)
	if dialogue == "" || dialogue == "无" || dialogue == "—" {
		return "无"
	}
	parts := strings.Split(dialogue, " / ")
	formatted := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "无" {
			continue
		}
		index := strings.IndexAny(part, "：:")
		if index <= 0 {
			formatted = append(formatted, "【"+part+"】")
			continue
		}
		speaker := strings.TrimSpace(part[:index])
		text := strings.TrimSpace(part[index+p3ColonByteLen(part[index:]):])
		hint := ""
		if open := strings.IndexAny(speaker, "（("); open >= 0 {
			close := strings.LastIndexAny(speaker, "）)")
			if close > open {
				hint = strings.TrimSpace(speaker[open+1 : close])
			}
			speaker = strings.TrimSpace(speaker[:open])
		}
		rate := "标准"
		if containsAnyText(hint, []string{"低语", "低声", "气音", "气声", "一字一顿", "断续", "拖长", "慢"}) {
			rate = "慢速"
		} else if containsAnyText(hint, []string{"急促", "连珠", "抢话", "快速", "快"}) {
			rate = "快速"
		}
		function := "信息交代"
		switch {
		case strings.ContainsAny(text, "？?"):
			function = "追问"
		case strings.ContainsAny(text, "！!") || strings.Contains(text, "——"):
			function = "喊话"
		}
		tone := []string{}
		if hint != "" {
			tone = append(tone, hint)
		}
		tone = append(tone, rate, function)
		formatted = append(formatted, fmt.Sprintf("【%s（%s）：%s】", speaker, strings.Join(tone, "·"), text))
	}
	if len(formatted) == 0 {
		return "无"
	}
	return strings.Join(formatted, " / ")
}

func p3ColonByteLen(value string) int {
	if strings.HasPrefix(value, "：") {
		return len("：")
	}
	return 1
}

func containsAnyText(value string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func p3OfficialShotSpec(headingTail string) (string, error) {
	parts := strings.Split(headingTail, "|")
	if len(parts) < 6 {
		return "", fmt.Errorf("expected space type, shot size, lens, angle, movement and subject label")
	}
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	size, lens, angle, movement, subjectLabel := parts[1], parts[2], parts[3], parts[4], parts[5]
	if size == "" || lens == "" || angle == "" || movement == "" {
		return "", fmt.Errorf("shot size, lens, angle or movement is empty")
	}
	spec := fmt.Sprintf("%s · %s · %s · %s", size, lens, angle, movement)
	if subjectLabel != "" {
		spec += "（" + subjectLabel + "）"
	}
	return spec, nil
}

func p3OfficialShotFieldValues(block string) (map[string]string, error) {
	values := make(map[string]string, len(p3OfficialShotFields))
	for _, rawLine := range strings.Split(block, "\n") {
		match := p3OfficialShotField.FindStringSubmatch(strings.TrimSpace(rawLine))
		if len(match) != 3 {
			continue
		}
		label := strings.TrimSpace(match[1])
		if !p3OfficialShotFieldKnown(label) {
			continue
		}
		values[label] = strings.TrimSpace(match[2])
	}
	missing := make([]string, 0)
	for _, label := range p3OfficialShotFields {
		if strings.TrimSpace(values[label]) == "" {
			missing = append(missing, label)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("九字段缺失：%s", strings.Join(missing, "、"))
	}
	return values, nil
}

func p3OfficialShotFieldKnown(label string) bool {
	for _, field := range p3OfficialShotFields {
		if field == label {
			return true
		}
	}
	return false
}

func p3OfficialShotAssetRefs(
	sceneKeys []string,
	searchText string,
	characters, props map[string]any,
) map[string]any {
	return map[string]any{
		"scenes":           uniqueAnyStrings(sceneKeys),
		"characters":       p3ExactCatalogMatches(searchText, characters),
		"props":            p3ExactCatalogMatches(searchText, props),
		"character_states": map[string]any{},
	}
}

func p3OfficialShotSceneKeys(value string, catalog map[string]any) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if len(catalog) == 0 || catalog[value] != nil {
		return []string{value}, nil
	}
	parts := regexp.MustCompile(`\s*[/｜|、]\s*`).Split(value, -1)
	keys := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("场景字段包含空场景键 %q", value)
		}
		if catalog[part] == nil {
			return nil, fmt.Errorf("引用 P2 未声明场景 %q", part)
		}
		keys = append(keys, part)
	}
	return keys, nil
}

func p3ExactCatalogMatches(content string, catalog map[string]any) []any {
	if len(catalog) == 0 {
		return []any{}
	}
	names := make([]string, 0, len(catalog))
	for name := range catalog {
		if strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	matches := make([]any, 0)
	for _, name := range names {
		if strings.Contains(content, name) {
			matches = append(matches, name)
		}
	}
	return matches
}

func parseOfficialCLEANGridSegments(content string) ([]storyboardImageSegment, error) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	inSection := false
	segments := make([]storyboardImageSegment, 0)
	seen := make(map[string]bool)
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if p3MarkdownHeadingLevel(line) > 0 {
			if p3MarkdownHeadingLevel(line) == 2 {
				inSection = strings.Contains(line, "故事板图") && strings.Contains(line, "CLEAN_GRID")
			}
			continue
		}
		if !inSection {
			continue
		}
		match := p3OfficialCleanGridLine.FindStringSubmatch(line)
		if len(match) != 4 {
			continue
		}
		code := "段" + match[1]
		if seen[code] {
			return nil, fmt.Errorf("%w: CLEAN_GRID 段号重复 %s", ErrStoryboardImageInvalid, code)
		}
		seen[code] = true
		panelCount, err := strconv.Atoi(match[2])
		if err != nil || panelCount <= 0 {
			return nil, fmt.Errorf("%w: %s CLEAN_GRID 格数无效 %q", ErrStoryboardImageInvalid, code, match[2])
		}
		prompt := strings.TrimSpace(match[3])
		segment := storyboardImageSegment{
			Code: code, Title: code, Prompt: prompt,
			Phases: p3OfficialCLEANGridPhases(prompt, panelCount),
			Shots:  make([]string, 0, panelCount),
		}
		for index := 1; index <= panelCount; index++ {
			segment.Shots = append(segment.Shots, fmt.Sprintf("%s-P%d", code, index))
		}
		segments = append(segments, segment)
	}
	return segments, nil
}

func p3OfficialCLEANGridPhases(prompt string, count int) []string {
	phases := make([]string, 0, count)
	markers := p3OfficialPanelMarker.FindAllStringSubmatch(prompt, -1)
	for _, match := range markers {
		label := strings.TrimSpace(match[2])
		if separator := strings.IndexAny(label, "·｜|"); separator >= 0 {
			label = strings.TrimSpace(label[:separator])
		}
		if label != "" {
			phases = append(phases, label)
		}
	}
	if len(phases) >= count {
		return phases[:count]
	}
	defaults := map[int][]string{
		1: {"起始"},
		2: {"起始", "出口"},
		3: {"起始", "过程", "出口"},
		4: {"起始", "过程", "反应", "出口"},
		5: {"起始", "过程", "接触", "结果", "出口"},
		6: {"起始", "过程", "接触", "顶点", "结果", "出口"},
		7: {"起始", "过程", "接触", "顶点", "结果", "反应", "出口"},
		8: {"起始", "过程", "接触", "顶点", "结果", "反应", "出口", "补位"},
		9: {"起始", "过程", "接触", "顶点", "结果", "反应", "出口", "补位", "收束"},
	}
	if values, ok := defaults[count]; ok {
		return append([]string(nil), values...)
	}
	for len(phases) < count {
		phases = append(phases, fmt.Sprintf("面板%d", len(phases)+1))
	}
	return phases
}
