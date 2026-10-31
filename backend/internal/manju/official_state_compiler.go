package manju

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var officialP1CharacterHeading = regexp.MustCompile(
	`^#{2,6}\s+(?:[0-9]+(?:\.[0-9]+)*\s+)?(CHR-[0-9]+)\s+([^（(\n]+?)(?:\s*[（(]([^）)\n]+)[）)])?\s*$`,
)

type officialAnchorCharacter struct {
	Code  string
	Name  string
	Level int
	Lines []string
}

func compileP1OfficialAnchorState(content string, state map[string]any) (map[string]any, bool, error) {
	world := compileP1WorldAnchor(content)
	characters := compileP1CharacterAnchors(content)
	if len(world) == 0 && len(characters) == 0 {
		return nil, false, nil
	}
	return map[string]any{
		"world_anchor": world,
		"characters":   characters,
	}, true, nil
}

func compileP1WorldAnchor(content string) map[string]any {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	start := -1
	level := 0
	for index, line := range lines {
		if strings.Contains(line, "【世界锚卡 WORLD_ANCHOR_CARD】") {
			start = index + 1
			break
		}
	}
	if start < 0 {
		for index, line := range lines {
			trimmed := strings.TrimSpace(line)
			if p3MarkdownHeadingLevel(trimmed) > 0 && strings.Contains(trimmed, "历史与视觉边界") {
				start = index + 1
				level = p3MarkdownHeadingLevel(trimmed)
				break
			}
		}
	}
	if start < 0 {
		return nil
	}
	section := make([]string, 0)
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		lineLevel := p3MarkdownHeadingLevel(trimmed)
		if strings.HasPrefix(trimmed, "【") || (lineLevel > 0 && (level == 0 || lineLevel <= level)) {
			break
		}
		section = append(section, line)
	}
	result := map[string]any{
		"era_anchor":         officialLabelValue(section, "年份/朝代", "年份", "朝代", "历史锚"),
		"region":             officialLabelValue(section, "地域"),
		"tech_level":         officialLabelValue(section, "技术时代"),
		"forbidden_elements": officialLabelList(section, "禁用元素清单", "禁用元素"),
		"required_elements":  officialLabelList(section, "必现元素清单", "必现元素"),
	}
	for key, value := range result {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				delete(result, key)
			}
		case []any:
			if len(typed) == 0 {
				delete(result, key)
			}
		}
	}
	return result
}

func compileP1CharacterAnchors(content string) map[string]any {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	characters := make(map[string]any)
	var active *officialAnchorCharacter
	flush := func() {
		if active == nil || strings.TrimSpace(active.Name) == "" {
			return
		}
		entry := officialCharacterStateEntry(active.Code, active.Name, active.Lines)
		mergeOfficialStateEntry(characters, active.Name, entry)
	}
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		level := p3MarkdownHeadingLevel(line)
		if level > 0 && active != nil && level <= active.Level {
			flush()
			active = nil
		}
		if level > 0 {
			if match := officialP1CharacterHeading.FindStringSubmatch(line); len(match) == 4 {
				active = &officialAnchorCharacter{
					Code: strings.TrimSpace(match[1]), Name: strings.TrimSpace(match[2]), Level: level,
				}
				continue
			}
		}
		if active != nil {
			active.Lines = append(active.Lines, rawLine)
		}
	}
	flush()
	return characters
}

func compileP2OfficialAssetState(content string, state map[string]any) (map[string]any, bool, error) {
	blocks, err := p2CanvasAssetBlocks(content)
	if err != nil {
		if strings.Contains(err.Error(), "未找到") {
			return nil, false, nil
		}
		return nil, false, err
	}
	characters := make(map[string]any)
	scenes := make(map[string]any)
	props := make(map[string]any)
	for _, block := range blocks {
		firstLine := strings.TrimSpace(strings.SplitN(block.Prompt, "\n", 2)[0])
		titleMatch := canvasAssetHeadingTitle.FindStringSubmatch(firstLine)
		title := block.SourceCode
		if len(titleMatch) == 2 && strings.TrimSpace(titleMatch[1]) != "" {
			title = strings.TrimSpace(titleMatch[1])
		}
		name := officialAssetName(title)
		if name == "" {
			name = block.SourceCode
		}
		category := p2CanvasCategoryFromBlock(title)
		code := canvasBlockAssetCode.FindString(block.Prompt)
		if strings.TrimSpace(code) == "" {
			code = officialAssetCodeForFigure(category, block.SourceCode)
		}
		figNo := canvasFigureNumberValue(block.SourceCode)
		if figNo <= 0 {
			figNo = canvasFigureNumberValue(block.Code)
		}
		switch category {
		case "character", "group":
			entry := officialCharacterStateEntry(code, name, strings.Split(block.Prompt, "\n"))
			entry["fig_no"] = figNo
			if category == "group" {
				entry["group"] = true
			}
			mergeOfficialStateEntry(characters, name, entry)
		case "environment":
			entry := map[string]any{
				"id":     code,
				"fig_no": figNo,
				"coords": officialLabelValue(
					strings.Split(block.Prompt, "\n"),
					"坐标系", "空间坐标系", "场景坐标",
				),
				"set_states": map[string]any{
					"default": officialLabelValue(strings.Split(block.Prompt, "\n"), "SET_STATE", "状态变体"),
				},
				"media_layer": officialLabelValue(strings.Split(block.Prompt, "\n"), "空气介质", "介质层"),
				"lighting":    officialLabelValue(strings.Split(block.Prompt, "\n"), "光照", "时段光影", "主光"),
			}
			mergeOfficialStateEntry(scenes, name, entry)
		case "prop":
			entry := map[string]any{
				"id":            code,
				"fig_no":        figNo,
				"material_desc": officialLabelValue(strings.Split(block.Prompt, "\n"), "材质", "材质描述"),
			}
			mergeOfficialStateEntry(props, name, entry)
		}
	}
	if len(characters) == 0 && len(scenes) == 0 && len(props) == 0 {
		return nil, false, nil
	}
	return map[string]any{
		"characters": characters,
		"scenes":     scenes,
		"props":      props,
	}, true, nil
}

func officialAssetCodeForFigure(category, figure string) string {
	number := canvasFigureNumberValue(figure)
	if number <= 0 {
		return ""
	}
	prefix := map[string]string{
		"character":   "CHR",
		"group":       "GRP",
		"environment": "SCN",
		"prop":        "PRP",
	}[category]
	if prefix == "" {
		return ""
	}
	return fmt.Sprintf("%s-%02d", prefix, number)
}

func officialAssetName(title string) string {
	title = strings.TrimSpace(title)
	if index := strings.IndexAny(title, "（("); index >= 0 {
		title = title[:index]
	}
	return strings.TrimSpace(title)
}

func officialCharacterStateEntry(code, name string, lines []string) map[string]any {
	age := officialLabelValue(lines, "年龄段", "年龄锚")
	bodyParts := []string{
		officialLabelValue(lines, "身高/体型", "体型锚"),
		officialLabelValue(lines, "体态与气质", "体态"),
	}
	body := make([]string, 0, len(bodyParts))
	for _, part := range bodyParts {
		if strings.TrimSpace(part) != "" {
			body = append(body, strings.TrimSpace(part))
		}
	}
	figNo := 0
	if match := regexp.MustCompile(`[0-9]+`).FindString(code); match != "" {
		figNo, _ = strconv.Atoi(match)
	}
	return map[string]any{
		"id":            strings.TrimSpace(code),
		"fig_no":        figNo,
		"age_anchor":    age,
		"body_anchor":   strings.Join(body, "；"),
		"look_3piece":   officialLabelValue(lines, "身份外观三件套"),
		"voice_lock":    officialLabelValue(lines, "Voice Description", "Voice Lock", "声线锁", "声线"),
		"business":      officialLabelValue(lines, "习惯动作账本", "Business"),
		"break_trigger": officialLabelValue(lines, "破功句", "然而，当"),
		"address_map":   map[string]any{},
		"char_states":   map[string]any{},
		"name":          strings.TrimSpace(name),
	}
}

func mergeOfficialStateEntry(target map[string]any, name string, entry map[string]any) {
	previous := anyMap(target[name])
	if len(previous) == 0 {
		target[name] = entry
		return
	}
	for key, value := range entry {
		if officialStateValueEmpty(value) {
			if _, exists := previous[key]; exists {
				continue
			}
		}
		previous[key] = value
	}
	target[name] = previous
}

func officialStateValueEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func officialLabelValue(lines []string, labels ...string) string {
	for _, rawLine := range lines {
		line := officialTrimLabelPrefix(rawLine)
		index := strings.IndexAny(line, "：:")
		if index <= 0 {
			continue
		}
		label := strings.TrimSpace(line[:index])
		if !officialLabelMatches(label, labels) {
			continue
		}
		return strings.TrimSpace(line[index+p3ColonByteLen(line[index:]):])
	}
	return ""
}

func officialTrimLabelPrefix(value string) string {
	return strings.TrimSpace(strings.TrimLeftFunc(strings.TrimSpace(value), func(char rune) bool {
		return unicode.IsSpace(char) ||
			unicode.IsSymbol(char) ||
			char == '\uFE0F' ||
			strings.ContainsRune("-*•", char)
	}))
}

func officialLabelList(lines []string, labels ...string) []any {
	value := officialLabelValue(lines, labels...)
	if value == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(char rune) bool {
		switch char {
		case '、', ',', '，', '；', ';':
			return true
		default:
			return false
		}
	})
	result := make([]any, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func officialLabelMatches(label string, labels []string) bool {
	for _, candidate := range labels {
		if label == candidate || strings.Contains(label, candidate) {
			return true
		}
	}
	return false
}
