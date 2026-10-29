package manju

import (
	"regexp"
	"strings"
)

var compiledManagementTalkPattern = regexp.MustCompile(`(?:接下一大分镜|接下一段|承接上一段|接分镜\s*\d+)(?:[，,；;。]\s*)?`)

var compiledCameraShortener = strings.NewReplacer(
	"摄影机极缓慢地向前推进", "极缓推",
	"非常非常缓慢地向前推进", "缓慢推进",
	"镜头保持固定不动", "固定",
)

var compiledLowValueClauses = map[string]bool{
	"整体保持真实自然": true,
	"确保画面连贯":   true,
	"保证动作流畅":   true,
}

type manjuFeedCompressionStats struct {
	ManagementRemoved       int
	LowValueClausesRemoved  int
	DuplicateClausesRemoved int
	CameraPhrasesShortened  int
}

func compressCompiledManjuFeed(feed, model string) (string, manjuFeedCompressionStats) {
	budget := manjuFeedUnitCharacterBudget(model)
	matches := manjuUnitHeadingPattern.FindAllStringSubmatchIndex(feed, -1)
	if len(matches) == 0 {
		return feed, manjuFeedCompressionStats{}
	}

	var out strings.Builder
	var stats manjuFeedCompressionStats
	out.WriteString(feed[:matches[0][0]])
	for index, match := range matches {
		end := len(feed)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		section := feed[match[0]:end]
		if runeLength(compiledUnitBody(section)) <= budget {
			out.WriteString(section)
			continue
		}
		compressed, sectionStats := compressCompiledManjuUnit(section)
		out.WriteString(compressed)
		stats.ManagementRemoved += sectionStats.ManagementRemoved
		stats.LowValueClausesRemoved += sectionStats.LowValueClausesRemoved
		stats.DuplicateClausesRemoved += sectionStats.DuplicateClausesRemoved
		stats.CameraPhrasesShortened += sectionStats.CameraPhrasesShortened
	}
	return out.String(), stats
}

func compiledUnitBody(section string) string {
	start := strings.Index(section, "```text")
	if start < 0 {
		return section
	}
	start += len("```text")
	if newline := strings.IndexByte(section[start:], '\n'); newline >= 0 {
		start += newline + 1
	}
	end := strings.Index(section[start:], "```")
	if end < 0 {
		return section[start:]
	}
	return section[start : start+end]
}

func compressCompiledManjuUnit(section string) (string, manjuFeedCompressionStats) {
	lines := strings.Split(section, "\n")
	var stats manjuFeedCompressionStats
	for index, line := range lines {
		match := compiledShotLinePattern.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		rest := match[5]
		tailIndex := compiledShotTailIndex(rest)
		if tailIndex <= 0 {
			continue
		}
		head := rest[:tailIndex]
		firstPeriod := strings.Index(head, "。")
		if firstPeriod < 0 || firstPeriod+len("。") >= len(head) {
			continue
		}
		actionStart := firstPeriod + len("。")
		spec := head[:firstPeriod]
		action := head[actionStart:]
		compressed, actionStats := compressManjuShotAction(action)
		shortenedSpec := compiledCameraShortener.Replace(spec)
		if shortenedSpec != spec {
			actionStats.CameraPhrasesShortened++
		}
		if compressed == action && shortenedSpec == spec {
			continue
		}
		stats.ManagementRemoved += actionStats.ManagementRemoved
		stats.LowValueClausesRemoved += actionStats.LowValueClausesRemoved
		stats.DuplicateClausesRemoved += actionStats.DuplicateClausesRemoved
		stats.CameraPhrasesShortened += actionStats.CameraPhrasesShortened
		prefixLength := len(line) - len(rest)
		lines[index] = line[:prefixLength] + shortenedSpec + "。" + compressed + rest[tailIndex:]
	}
	return strings.Join(lines, "\n"), stats
}

func compiledShotTailIndex(rest string) int {
	soundIndex := strings.Index(rest, "。声音：")
	dialogueIndex := strings.Index(rest, "。台词：")
	switch {
	case soundIndex < 0:
		return dialogueIndex
	case dialogueIndex < 0:
		return soundIndex
	case dialogueIndex < soundIndex:
		return dialogueIndex
	default:
		return soundIndex
	}
}

func compressManjuShotAction(action string) (string, manjuFeedCompressionStats) {
	var stats manjuFeedCompressionStats
	managementMatches := compiledManagementTalkPattern.FindAllStringIndex(action, -1)
	compressed := compiledManagementTalkPattern.ReplaceAllString(action, "")
	stats.ManagementRemoved = len(managementMatches)

	clauses := strings.Split(compressed, "；")
	seen := make(map[string]bool, len(clauses))
	unique := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		key := strings.Trim(clause, "，,。； ")
		if compiledLowValueClauses[key] {
			stats.LowValueClausesRemoved++
			continue
		}
		shortened := compiledCameraShortener.Replace(clause)
		if shortened != clause {
			stats.CameraPhrasesShortened++
		}
		clause = shortened
		key = strings.Trim(clause, "，,。； ")
		if key == "" {
			continue
		}
		if seen[key] {
			stats.DuplicateClausesRemoved++
			continue
		}
		seen[key] = true
		unique = append(unique, clause)
	}
	compressed = strings.Trim(strings.Join(unique, "；"), "；，, ")
	if compressed == "" {
		return action, stats
	}
	return compressed, stats
}
