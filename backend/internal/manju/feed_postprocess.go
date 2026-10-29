package manju

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var customStyleAspectPattern = regexp.MustCompile(`(?i)(?:2\.35\s*[:：]\s*1|2\.39\s*[:：]\s*1|21\s*[:：]\s*9|16\s*[:：]\s*9|9\s*[:：]\s*16|4\s*[:：]\s*3|1\s*[:：]\s*1)`)
var customStyleClausePattern = regexp.MustCompile(`[，,；;]+`)
var customStyleRepeatedPunctuation = regexp.MustCompile(`([，,；;])\s*[，,；;]+`)
var compiledLensPattern = regexp.MustCompile(`(?i)(\d{1,3})\s*mm`)

type customStyleFields struct {
	Visual    string
	Motion    string
	Forbidden []string
}

func (r *ManjuSkillRuntime) injectStyleLockBlocks(feed string, options ManjuSkillValidationOptions) (string, error) {
	if strings.TrimSpace(options.StyleSource) == "" {
		return feed, nil
	}
	blocks, found, err := r.officialStyleLockBlocks(options.StyleSource)
	if err != nil {
		return "", err
	}
	if !found {
		lensMin, lensMax := genericStyleLensRange(feed)
		blocks = genericStyleLockBlocksWithLens(
			stylePromptParameters(options.StateJSON, options.StylePromptParameters),
			options.StyleMotionParameters,
			lensMin,
			lensMax,
		)
	}
	matches := manjuUnitHeadingPattern.FindAllStringIndex(feed, -1)
	if len(matches) == 0 {
		return strings.TrimSpace(feed) + "\n\n" + blocks, nil
	}
	var out strings.Builder
	cursor := 0
	for index, match := range matches {
		out.WriteString(feed[cursor:match[0]])
		end := len(feed)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		out.WriteString(injectStyleLockBlocksIntoUnit(feed[match[0]:end], blocks))
		cursor = end
	}
	out.WriteString(feed[cursor:])
	return out.String(), nil
}

func injectStyleLockBlocksIntoUnit(section, blocks string) string {
	styleMarker, performanceMarker, position, err := p4StyleLockMarkers()
	if err != nil || position != "each-unit-text-block" {
		return section
	}
	hasStyle := strings.Contains(section, styleMarker)
	hasPerformance := strings.Contains(section, performanceMarker)
	if hasStyle && hasPerformance {
		return section
	}
	codeStart := strings.Index(section, "```text")
	if codeStart < 0 {
		return section
	}
	contentStart := codeStart + len("```text")
	if newline := strings.IndexByte(section[contentStart:], '\n'); newline >= 0 {
		contentStart += newline + 1
	}
	codeEnd := strings.Index(section[contentStart:], "\n```")
	if codeEnd < 0 {
		return section
	}
	insertAt := contentStart + codeEnd
	insert := strings.TrimSpace(blocks)
	if hasStyle || hasPerformance {
		styleBlock, performanceBlock := splitStyleLockBlocks(blocks)
		parts := make([]string, 0, 2)
		if !hasStyle && strings.TrimSpace(styleBlock) != "" {
			parts = append(parts, strings.TrimSpace(styleBlock))
		}
		if !hasPerformance && strings.TrimSpace(performanceBlock) != "" {
			parts = append(parts, strings.TrimSpace(performanceBlock))
		}
		insert = strings.Join(parts, "\n\n")
	}
	if insert == "" {
		return section
	}
	return section[:insertAt] + "\n\n" + insert + section[insertAt:]
}

func splitStyleLockBlocks(blocks string) (string, string) {
	marker := "\n\n【PERFORMANCE"
	if index := strings.Index(blocks, marker); index >= 0 {
		return strings.TrimSpace(blocks[:index]), strings.TrimSpace(blocks[index+2:])
	}
	return strings.TrimSpace(blocks), ""
}

func (r *ManjuSkillRuntime) officialStyleLockBlocks(styleSource string) (string, bool, error) {
	root, err := r.ensureRoot()
	if err != nil {
		return "", false, err
	}
	path := filepath.Join(root, "short-drama-director", "references", "style-bible", "style-prefix-blocks.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	heading := "## " + strings.TrimSpace(styleSource) + "块"
	start := strings.Index(content, heading)
	if start < 0 {
		return "", false, nil
	}
	end := len(content)
	if next := strings.Index(content[start+len(heading):], "\n## "); next >= 0 {
		end = start + len(heading) + next
	}
	section := content[start:end]
	styleBlock := markdownCodeBlockAfter(section, "### 段 8")
	performanceBlock := markdownCodeBlockAfter(section, "### 段 9")
	if strings.TrimSpace(styleBlock) == "" || strings.TrimSpace(performanceBlock) == "" {
		return "", false, nil
	}
	return strings.TrimSpace(styleBlock) + "\n\n" + strings.TrimSpace(performanceBlock), true, nil
}

func markdownCodeBlockAfter(section, heading string) string {
	headingIndex := strings.Index(section, heading)
	if headingIndex < 0 {
		return ""
	}
	rest := section[headingIndex+len(heading):]
	fenceStart := strings.Index(rest, "```")
	if fenceStart < 0 {
		return ""
	}
	rest = rest[fenceStart+3:]
	if newline := strings.IndexByte(rest, '\n'); newline >= 0 {
		rest = rest[newline+1:]
	}
	fenceEnd := strings.Index(rest, "```")
	if fenceEnd < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:fenceEnd])
}

func stylePromptParameters(stateJSON, fallback string) string {
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	var state struct {
		Locks struct {
			StyleCard string `json:"style_card"`
		} `json:"locks"`
	}
	if json.Unmarshal([]byte(stateJSON), &state) == nil {
		if value := strings.TrimSpace(state.Locks.StyleCard); value != "" {
			return value
		}
	}
	return "电影级写实影像基底"
}

func genericStyleLockBlocks(styleParameters, styleMotionParameters string) string {
	return genericStyleLockBlocksWithLens(styleParameters, styleMotionParameters, 24, 85)
}

func genericStyleLockBlocksWithLens(styleParameters, styleMotionParameters string, lensMin, lensMax int) string {
	style, performance := genericStylePromptBlocksWithLens(styleParameters, styleMotionParameters, lensMin, lensMax)
	return style + "\n\n" + performance
}

func genericStylePromptBlocks(styleParameters, styleMotionParameters string) (string, string) {
	return genericStylePromptBlocksWithLens(styleParameters, styleMotionParameters, 24, 85)
}

func genericStylePromptBlocksWithLens(styleParameters, styleMotionParameters string, lensMin, lensMax int) (string, string) {
	if lensMin <= 0 {
		lensMin = 24
	}
	if lensMax < lensMin {
		lensMax = 85
	}
	styleParameters = fallbackValue(strings.TrimSpace(styleParameters), "电影级写实影像基底")
	targetBans := "禁无源彩光 / 禁正面平光 / 禁塑料磨皮 / 禁角色换脸 / 禁材质漂移"
	style := fmt.Sprintf(`【STYLE｜视觉风格 · 全片锁死】
成像底层：电影级数字影像采集，%s，颗粒强度 [低, 中]
形体骨相：6.5~7.5 头身真实立体骨相，面部结构稳定，服装形制连续
影调区间：光比 [1:4, 1:9]，方向性主光塑形，阴影侧保留环境细节，禁均匀平光
色彩区间：主调随场景冷暖分离，色温 [2800K, 6500K]，饱和度 [低, 中]
空气介质：自然烟尘、雨雾或火光悬浮颗粒（≤2 层），只由画面内真实光源照亮
光学区间：镜头 [%dmm, %dmm]，广角交代空间，中长焦锁定人物反应
材质响应：服装织物、木材、金属、石材与皮肤保持真实物理反射和磨损层次
特效语言：物理气浪、火星、尘土、水花和碎屑优先，禁止无来源发光特效
针对性禁项：%s`, styleParameters, lensMin, lensMax, targetBans)
	performance := `【PERFORMANCE｜表演与动作 · 全片锁死】
帧率区间：[24fps, 60fps]，常态以极缓慢推、移、跟随制造呼吸感
动作遵循起势→爆发→短暂停顿→收势四拍，接触瞬间给出明确受力反馈
每段至少保留一次全身或中景空间参照，禁止无交代瞬移与原地悬浮`
	if styleMotionParameters = strings.TrimSpace(styleMotionParameters); styleMotionParameters != "" {
		performance += "\n运镜参数：" + styleMotionParameters
	}
	return style, performance
}

func genericStyleLensRange(feed string) (int, int) {
	lensMin := 24
	lensMax := 85
	for _, match := range compiledLensPattern.FindAllStringSubmatch(feed, -1) {
		if len(match) < 2 {
			continue
		}
		value, err := strconv.Atoi(match[1])
		if err != nil || value < 8 || value > 300 {
			continue
		}
		if value < lensMin {
			lensMin = value
		}
		if value > lensMax {
			lensMax = value
		}
	}
	return lensMin, lensMax
}

func normalizeCustomStyleParameters(value string) (string, string) {
	fields := normalizeCustomStyleFields(value)
	return fields.Visual, strings.Join(fields.Forbidden, "；")
}

func normalizeCustomStyleFields(value string) customStyleFields {
	value = normalizeCustomStyleLineBreaks(value)
	if value == "" {
		return customStyleFields{}
	}
	visual := make([]string, 0, 8)
	motion := make([]string, 0, 4)
	forbidden := make([]string, 0, 8)
	for _, raw := range strings.Split(value, "\n") {
		line := cleanCustomStyleClause(raw)
		if line == "" {
			continue
		}
		if looksLikeForbiddenStyleList(line) {
			forbidden = append(forbidden, splitCustomStyleClauses(line)...)
			continue
		}
		if looksLikeMotionStyleLine(line) {
			motion = append(motion, normalizeMotionStyleLine(line))
			continue
		}
		for _, rawClause := range customStyleClausePattern.Split(line, -1) {
			clause := cleanCustomStyleClause(rawClause)
			if clause == "" {
				continue
			}
			switch {
			case isForbiddenStyleClause(clause):
				forbidden = append(forbidden, clause)
			case looksLikeMotionStyleClause(clause):
				motion = append(motion, clause)
			default:
				visual = append(visual, clause)
			}
		}
	}
	return customStyleFields{
		Visual:    strings.Join(uniqueCustomStyleItems(visual...), "，"),
		Motion:    strings.Join(uniqueCustomStyleItems(motion...), "；"),
		Forbidden: uniqueCustomStyleItems(forbidden...),
	}
}

func normalizeCustomStyleLineBreaks(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer(
		"\r\n", "\n",
		"\r", "\n",
		"\u2028", "\n",
		"\u2029", "\n",
	)
	return replacer.Replace(value)
}

func cleanCustomStyleClause(value string) string {
	value = customStyleAspectPattern.ReplaceAllString(strings.TrimSpace(value), "")
	value = strings.ReplaceAll(value, "变形宽银幕画幅", "")
	value = strings.ReplaceAll(value, "宽银幕画幅", "")
	value = customStyleRepeatedPunctuation.ReplaceAllString(value, "$1")
	return strings.Trim(value, "，,；; /")
}

func splitCustomStyleClauses(value string) []string {
	result := make([]string, 0, 8)
	for _, raw := range customStyleClausePattern.Split(value, -1) {
		clause := cleanCustomStyleClause(raw)
		if clause != "" {
			result = append(result, clause)
		}
	}
	return result
}

func looksLikeForbiddenStyleList(value string) bool {
	lower := strings.ToLower(value)
	hits := 0
	for _, term := range []string{
		"现代元素", "鲜艳色彩", "干净崭新", "棚拍感", "塑料质感", "过度磨皮",
		"商业广告感", "卡通", "动画", "cg感", "游戏cg", "三渲二",
		"modern elements", "vivid saturated colors", "clean new costumes",
		"studio lighting look", "plastic armor", "over-smoothed skin",
		"commercial ad look", "cartoon", "cgi-looking",
	} {
		if strings.Contains(lower, term) {
			hits++
		}
	}
	return hits >= 2
}

func looksLikeMotionStyleLine(value string) bool {
	if strings.HasPrefix(value, "【") {
		end := strings.Index(value, "】")
		if end > 0 {
			label := strings.ToLower(strings.TrimSpace(value[1:end]))
			return strings.Contains(label, "手持") ||
				strings.Contains(label, "固定") ||
				strings.Contains(label, "运动") ||
				strings.Contains(label, "运镜") ||
				strings.Contains(label, "机位") ||
				strings.Contains(label, "镜头")
		}
	}
	return looksLikeMotionStyleClause(value)
}

func looksLikeMotionStyleClause(value string) bool {
	lower := strings.ToLower(value)
	for _, term := range []string{
		"运镜", "手持摄影", "手持拍摄", "handheld camera", "脚架固定", "三脚架",
		"locked-off tripod", "跟拍", "tracking shot", "轨道", "dolly", "推镜",
		"push-in", "拉镜", "pull-out", "摇镜", "pan shot", "航拍", "aerial drone",
		"无人机", "升降镜头", "crane shot", "变焦", "zoom lens", "摄影机运动",
	} {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func normalizeMotionStyleLine(value string) string {
	if !strings.HasPrefix(value, "【") {
		return value
	}
	end := strings.Index(value, "】")
	if end < 0 {
		return value
	}
	label := strings.TrimSpace(value[len("【"):end])
	return "运镜·" + label + "：" + strings.TrimSpace(value[end+len("】"):])
}

func isForbiddenStyleClause(value string) bool {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{
		"禁止", "严禁", "禁用", "拒绝", "排除", "避免", "不得", "不可",
		"不含", "不要", "去掉", "避免出现", "无需",
	} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return looksLikeForbiddenStyleList(value)
}

func uniqueCustomStyleItems(values ...string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func normalizeCompiledManjuAspectOrientation(feed, aspect string) string {
	aspect = strings.TrimSpace(aspect)
	if aspect == "" {
		return feed
	}
	orientation := "横屏"
	if aspect == "9:16" {
		orientation = "竖屏"
	}
	lines := strings.Split(feed, "\n")
	for index, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "【画幅风格】") {
			continue
		}
		line = strings.ReplaceAll(line, aspect+" 横屏", aspect+" "+orientation)
		line = strings.ReplaceAll(line, aspect+" 竖屏", aspect+" "+orientation)
		lines[index] = line
	}
	return strings.Join(lines, "\n")
}
