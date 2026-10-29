package manju

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var compiledShotLinePattern = regexp.MustCompile(`^(\d{2}):(\d{2})-(\d{2}):(\d{2}) \[镜头\d+\] (.*)$`)

var (
	ErrManjuSingleShotExceedsBudget   = errors.New("P4 单个镜头超过投喂预算")
	ErrManjuSingleShotExceedsDuration = errors.New("P4 单个镜头超过模型单段时长上限")
)

type compiledFeedChunk struct {
	SegmentID string
	Body      string
	Tail      string
}

type parsedCompiledUnit struct {
	SegmentID string
	Prefix    []string
	Shots     []string
	Suffix    []string
	Tail      string
	Original  string
}

func splitCompiledManjuFeedByBudget(feed, model string) (string, error) {
	return splitCompiledManjuFeed(feed, model, false)
}

func splitCompiledManjuFeedByShotBoundary(feed, model string) (string, error) {
	return splitCompiledManjuFeed(feed, model, true)
}

func splitCompiledManjuFeed(feed, model string, allowOversizedSingleShot bool) (string, error) {
	budget := manjuFeedUnitCharacterBudget(model)
	durationLimit := manjuFeedUnitDurationLimit(model)
	matches := manjuUnitHeadingPattern.FindAllStringSubmatchIndex(feed, -1)
	if len(matches) == 0 {
		return feed, nil
	}

	chunks := make([]compiledFeedChunk, 0, len(matches))
	changed := false
	for index, match := range matches {
		end := len(feed)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		unit, err := parseCompiledUnit(feed[match[0]:end])
		if err != nil {
			return "", err
		}
		unitChunks, err := splitParsedCompiledUnit(unit, budget, durationLimit, allowOversizedSingleShot)
		if err != nil {
			return "", err
		}
		if len(unitChunks) > 1 {
			changed = true
		}
		chunks = append(chunks, unitChunks...)
	}
	if !changed {
		return feed, nil
	}

	var out strings.Builder
	out.WriteString(feed[:matches[0][0]])
	total := len(chunks)
	for index, chunk := range chunks {
		if index > 0 {
			out.WriteString("\n\n")
		}
		fmt.Fprintf(&out, "## 【投喂单元 %d/%d · %s】\n", index+1, total, chunk.SegmentID)
		out.WriteString("```text\n")
		out.WriteString(chunk.Body)
		out.WriteString("\n```")
		if strings.TrimSpace(chunk.Tail) != "" {
			out.WriteString("\n\n")
			out.WriteString(strings.TrimSpace(chunk.Tail))
		}
	}
	out.WriteString("\n")
	return out.String(), nil
}

func parseCompiledUnit(section string) (parsedCompiledUnit, error) {
	lines := strings.Split(strings.TrimRight(section, "\r\n"), "\n")
	if len(lines) == 0 {
		return parsedCompiledUnit{}, fmt.Errorf("P4 投喂单元为空")
	}
	header := strings.TrimSpace(lines[0])
	match := manjuUnitHeadingPattern.FindStringSubmatch(header)
	if len(match) < 4 {
		return parsedCompiledUnit{}, fmt.Errorf("无法解析 P4 投喂单元标题: %s", header)
	}
	codeStart := -1
	codeEnd := -1
	for index := 1; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if codeStart < 0 {
			if trimmed == "```text" {
				codeStart = index
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			codeEnd = index
			break
		}
	}
	if codeStart < 0 || codeEnd <= codeStart {
		return parsedCompiledUnit{}, fmt.Errorf("P4 投喂单元 %s 缺少完整 text 代码块", match[3])
	}

	bodyLines := lines[codeStart+1 : codeEnd]
	firstShot := -1
	lastShot := -1
	for index, line := range bodyLines {
		if compiledShotLinePattern.MatchString(strings.TrimSpace(line)) {
			if firstShot < 0 {
				firstShot = index
			}
			lastShot = index
		}
	}
	unit := parsedCompiledUnit{
		SegmentID: strings.TrimSpace(match[3]),
		Tail:      strings.Join(lines[codeEnd+1:], "\n"),
		Original:  section,
	}
	if firstShot < 0 {
		unit.Prefix = append([]string(nil), bodyLines...)
		return unit, nil
	}
	unit.Prefix = append([]string(nil), bodyLines[:firstShot]...)
	unit.Shots = append([]string(nil), bodyLines[firstShot:lastShot+1]...)
	unit.Suffix = append([]string(nil), bodyLines[lastShot+1:]...)
	return unit, nil
}

func splitParsedCompiledUnit(unit parsedCompiledUnit, budget, durationLimit int, allowOversizedSingleShot bool) ([]compiledFeedChunk, error) {
	if runeLength(unit.Original) <= budget && compiledShotsDuration(unit.Shots) <= durationLimit {
		return []compiledFeedChunk{{
			SegmentID: unit.SegmentID,
			Body:      strings.Join(append(append(append([]string{}, unit.Prefix...), unit.Shots...), unit.Suffix...), "\n"),
			Tail:      unit.Tail,
		}}, nil
	}
	if len(unit.Shots) == 0 {
		return nil, fmt.Errorf("P4 段%s 超过 %d 字且没有可拆分的镜头", unit.SegmentID, budget)
	}

	var groups [][]string
	current := make([]string, 0, len(unit.Shots))
	for shotIndex, shot := range unit.Shots {
		candidate := append(append([]string(nil), current...), shot)
		candidateLength := compiledUnitLength(unit.SegmentID, unit.Prefix, candidate, unit.Suffix)
		candidateDuration := compiledShotsDuration(candidate)
		if len(current) > 0 && (candidateLength > budget || candidateDuration > durationLimit) {
			groups = append(groups, current)
			current = []string{shot}
			singleDuration := compiledShotsDuration(current)
			if singleDuration > durationLimit && !allowOversizedSingleShot {
				return nil, fmt.Errorf(
					"%w: %s 的镜头%d 时长 %d 秒，超过 %d 秒；请在 P3 拆分该镜头或把拆分点移到镜头边界",
					ErrManjuSingleShotExceedsDuration, unit.SegmentID, shotIndex+1, singleDuration, durationLimit,
				)
			}
			continue
		}
		current = candidate
		if candidateLength > budget && !allowOversizedSingleShot {
			return nil, fmt.Errorf(
				"%w: %s 的镜头%d 展开后约 %d 字，超过 %d 字预算；请在 P3 把该镜头拆成多个连续 shot，保留全部动作、台词和音效",
				ErrManjuSingleShotExceedsBudget, unit.SegmentID, shotIndex+1, candidateLength, budget,
			)
		}
		if candidateDuration > durationLimit && !allowOversizedSingleShot {
			return nil, fmt.Errorf(
				"%w: %s 的镜头%d 时长 %d 秒，超过 %d 秒；请在 P3 拆分该镜头或把拆分点移到镜头边界",
				ErrManjuSingleShotExceedsDuration, unit.SegmentID, shotIndex+1, candidateDuration, durationLimit,
			)
		}
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}

	chunks := make([]compiledFeedChunk, 0, len(groups))
	previousLastScale := ""
	for index, group := range groups {
		normalized := normalizeCompiledShots(group, previousLastScale)
		if len(normalized) > 0 {
			previousLastScale = compiledShotScale(normalized[len(normalized)-1])
		}
		body := strings.Join(append(append(append([]string{}, unit.Prefix...), normalized...), unit.Suffix...), "\n")
		segmentID := unit.SegmentID
		if len(groups) > 1 {
			segmentID = fmt.Sprintf("%s-%d", unit.SegmentID, index+1)
		}
		chunks = append(chunks, compiledFeedChunk{
			SegmentID: segmentID,
			Body:      body,
			Tail:      "",
		})
	}
	if len(chunks) > 0 {
		chunks[len(chunks)-1].Tail = unit.Tail
	}
	return chunks, nil
}

func compiledUnitLength(segmentID string, prefix, shots, suffix []string) int {
	normalized := normalizeCompiledShots(shots, "")
	body := strings.Join(append(append(append([]string{}, prefix...), normalized...), suffix...), "\n")
	text := fmt.Sprintf("## 【投喂单元 99/99 · %s】\n```text\n%s\n```", segmentID, body)
	return runeLength(text)
}

func compiledShotsDuration(shots []string) int {
	start := -1
	end := 0
	for _, line := range shots {
		match := compiledShotLinePattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) == 0 {
			continue
		}
		shotStart := compiledTimeSeconds(match[1], match[2])
		shotEnd := compiledTimeSeconds(match[3], match[4])
		if start < 0 || shotStart < start {
			start = shotStart
		}
		if shotEnd > end {
			end = shotEnd
		}
	}
	if start < 0 || end <= start {
		return 0
	}
	return end - start
}

func normalizeCompiledShots(shots []string, seamScale string) []string {
	if len(shots) == 0 {
		return nil
	}
	origin := 0
	if match := compiledShotLinePattern.FindStringSubmatch(strings.TrimSpace(shots[0])); len(match) > 0 {
		origin = compiledTimeSeconds(match[1], match[2])
	}
	result := make([]string, 0, len(shots))
	for index, line := range shots {
		match := compiledShotLinePattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) == 0 {
			result = append(result, line)
			continue
		}
		start := maxInt(0, compiledTimeSeconds(match[1], match[2])-origin)
		end := maxInt(start+1, compiledTimeSeconds(match[3], match[4])-origin)
		normalizedLine := fmt.Sprintf(
			"%s-%s [镜头%d] %s",
			compiledTimeLabel(start), compiledTimeLabel(end), index+1, match[5],
		)
		if index == 0 && seamScale != "" {
			normalizedLine = replaceCompiledShotScale(normalizedLine, seamScale)
		}
		result = append(result, normalizedLine)
	}
	if len(result) > 0 {
		result[len(result)-1] = ensureCompiledShotEndFreeze(result[len(result)-1])
	}
	return result
}

func compiledShotScale(line string) string {
	match := compiledShotLinePattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(match) == 0 {
		return ""
	}
	head := match[5]
	if period := strings.Index(head, "。"); period >= 0 {
		head = head[:period]
	}
	parts := strings.Split(head, " · ")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func replaceCompiledShotScale(line, scale string) string {
	if strings.TrimSpace(scale) == "" {
		return line
	}
	match := compiledShotLinePattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(match) == 0 {
		return line
	}
	rest := match[5]
	period := strings.Index(rest, "。")
	if period < 0 {
		return line
	}
	spec := rest[:period]
	parts := strings.Split(spec, " · ")
	if len(parts) == 0 {
		return line
	}
	parts[0] = strings.TrimSpace(scale)
	return fmt.Sprintf(
		"%s-%s [镜头%s] %s%s",
		compiledTimeLabel(compiledTimeSeconds(match[1], match[2])),
		compiledTimeLabel(compiledTimeSeconds(match[3], match[4])),
		shotNumberFromCompiledLine(line),
		strings.Join(parts, " · "),
		rest[period:],
	)
}

func ensureCompiledShotEndFreeze(line string) string {
	match := compiledShotLinePattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(match) == 0 {
		return line
	}
	rest := match[5]
	tailIndex := compiledShotTailIndex(rest)
	if tailIndex <= 0 {
		return line
	}
	head := rest[:tailIndex]
	firstPeriod := strings.Index(head, "。")
	if firstPeriod < 0 || firstPeriod+len("。") >= len(head) {
		return line
	}
	actionStart := firstPeriod + len("。")
	action := head[actionStart:]
	if strings.Contains(action, "定格") || strings.Contains(action, "空镜") {
		return line
	}
	start := compiledTimeSeconds(match[1], match[2])
	end := compiledTimeSeconds(match[3], match[4])
	settle := maxInt(start, end-1)
	action = strings.TrimRight(action, "；。 ") +
		fmt.Sprintf("；第 %d 秒起运动收束，末 1 秒全体人物保持姿态与站位、动作完全静止、画面定格", settle)
	return fmt.Sprintf("%s-%s [镜头%s] %s%s%s",
		compiledTimeLabel(compiledTimeSeconds(match[1], match[2])),
		compiledTimeLabel(compiledTimeSeconds(match[3], match[4])),
		shotNumberFromCompiledLine(line), head[:actionStart], action, rest[tailIndex:])
}

func shotNumberFromCompiledLine(line string) string {
	match := regexp.MustCompile(`\[镜头(\d+)\]`).FindStringSubmatch(line)
	if len(match) > 1 {
		return match[1]
	}
	return "1"
}

func compiledTimeSeconds(minutes, seconds string) int {
	minuteValue, _ := strconv.Atoi(minutes)
	secondValue, _ := strconv.Atoi(seconds)
	return minuteValue*60 + secondValue
}

func compiledTimeLabel(totalSeconds int) string {
	return fmt.Sprintf("%02d:%02d", totalSeconds/60, totalSeconds%60)
}

func manjuFeedUnitCharacterBudget(model string) int {
	model = strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(model, "2.0") || strings.Contains(model, "h3") || strings.Contains(model, "minimax") {
		return 1900
	}
	return 2600
}

func manjuFeedUnitDurationLimit(model string) int {
	model = strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(model, "2.0") || strings.Contains(model, "h3") || strings.Contains(model, "minimax") {
		return 15
	}
	return 30
}

func runeLength(value string) int {
	return len([]rune(value))
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
