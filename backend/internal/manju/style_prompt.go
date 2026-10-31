package manju

import (
	"fmt"
	"strings"
)

const (
	styleImageMarker       = "### 段 8 · STYLE"
	stylePerformanceMarker = "### 段 9 · PERFORMANCE"
)

// compileStylePromptBlock turns the P0 selection-layer card into the
// model-facing STYLE / PERFORMANCE blocks. Main cards use the embedded
// V7.4.26 authoritative blocks verbatim; other cards use the documented
// 11-slot fallback assembly path.
func compileStylePromptBlock(cardName, promptParameters, referenceAnalysis string) (string, string, error) {
	cardName = strings.TrimSpace(cardName)
	promptParameters = sanitizeStyleNames(promptParameters)
	referenceAnalysis = sanitizeStyleNames(referenceAnalysis)
	raw, err := embeddedManjuSkill.ReadFile("skilldata/v7.4.26/short-drama-director/references/style-bible/style-prefix-blocks.md")
	if err != nil {
		return "", "", fmt.Errorf("读取内嵌画风成品块失败: %w", err)
	}
	stylePrompt := ""
	performancePrompt := ""
	if section := styleBlockSection(string(raw), cardName); section != "" {
		stylePrompt = styleMarkdownCodeBlockAfter(section, styleImageMarker)
		performancePrompt = styleMarkdownCodeBlockAfter(section, stylePerformanceMarker)
	}
	if strings.TrimSpace(stylePrompt) == "" {
		stylePrompt = genericStylePromptBlock(promptParameters)
	}
	if analysis := strings.TrimSpace(referenceAnalysis); analysis != "" {
		stylePrompt = strings.TrimSpace(stylePrompt) + "\n" +
			"参考图风格分析：" + analysis + "\n" +
			"参考图职责：只迁移光线、色彩、材质、镜头与后期质感；不复刻构图、人物身份、服装款式和具体场景。"
	}
	return strings.TrimSpace(stylePrompt), strings.TrimSpace(performancePrompt), nil
}

func styleBlockSection(markdown, cardName string) string {
	if cardName == "" {
		return ""
	}
	headings := markdownSectionBounds(markdown, "## ")
	bestStart := -1
	bestEnd := -1
	for index, heading := range headings {
		title := strings.TrimSpace(strings.TrimPrefix(heading.line, "## "))
		if !strings.HasPrefix(title, cardName+"块") {
			continue
		}
		bestStart = heading.start
		bestEnd = len(markdown)
		if index+1 < len(headings) {
			bestEnd = headings[index+1].start
		}
		break
	}
	if bestStart < 0 {
		return ""
	}
	return markdown[bestStart:bestEnd]
}

type markdownHeadingBound struct {
	line  string
	start int
}

func markdownSectionBounds(markdown, prefix string) []markdownHeadingBound {
	lines := strings.Split(markdown, "\n")
	result := make([]markdownHeadingBound, 0)
	offset := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			result = append(result, markdownHeadingBound{line: trimmed, start: offset})
		}
		offset += len(line) + 1
	}
	return result
}

func styleMarkdownCodeBlockAfter(section, marker string) string {
	start := strings.Index(section, marker)
	if start < 0 {
		return ""
	}
	rest := section[start+len(marker):]
	nextSection := strings.Index(rest, "\n### ")
	if nextSection >= 0 {
		rest = rest[:nextSection]
	}
	fenceStart := strings.Index(rest, "```")
	if fenceStart < 0 {
		return ""
	}
	rest = rest[fenceStart+3:]
	if lineEnd := strings.IndexByte(rest, '\n'); lineEnd >= 0 {
		rest = rest[lineEnd+1:]
	}
	fenceEnd := strings.Index(rest, "```")
	if fenceEnd < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:fenceEnd])
}

func genericStylePromptBlock(promptParameters string) string {
	promptParameters = fallbackValue(sanitizeStyleNames(promptParameters), "项目已锁定的可投喂视觉参数")
	return strings.Join([]string{
		"【STYLE｜视觉风格 · 全片锁死】",
		"成像底层：以 P0 影像基底为准，真实摄影或对应动画介质统一，禁止跨媒介混用",
		"形体骨相：角色骨相、年龄感、头身比与服装形制跨图一致，禁止换脸与比例漂移",
		"影调区间：沿用锁定风格的光比与明暗结构，暗部保留层次，禁止无源均匀平光",
		"色彩区间：主辅色、饱和度与色温按风格参数统一，禁止每张图自行改色",
		"空气介质：体积光、雾、尘、雨或粒子不超过两层，不得遮挡主体面部",
		"光学区间：镜头焦段、景深和透视保持统一，禁止无动机广角畸变与随机变焦",
		"材质响应：皮肤、布料、金属、木材与石材保留真实纹理和受力反馈",
		"风格参数：" + promptParameters,
		"针对性禁项：禁风格漂移 / 禁跨图材质不一致 / 禁网红磨皮 / 禁文字水印 / 禁复刻参考图构图",
	}, "\n")
}

func customStyleFieldsFromSnapshot(snapshot LockSnapshot) customStyleFields {
	hasExplicitFields := strings.TrimSpace(snapshot.StyleVisualPrompt) != "" ||
		strings.TrimSpace(snapshot.StyleMotionParameters) != "" ||
		len(snapshot.StyleForbiddenItems) > 0
	legacyRaw := ""
	if !hasExplicitFields {
		legacyRaw = strings.TrimSpace(snapshot.StyleCustomDescription)
		if legacyRaw == "" {
			legacyRaw = strings.TrimSpace(snapshot.StylePromptParameters)
		}
	}
	legacy := normalizeCustomStyleFields(legacyRaw)
	visualInput := normalizeCustomStyleFields(snapshot.StyleVisualPrompt)
	motionInput := normalizeCustomStyleFields(snapshot.StyleMotionParameters)

	visual := visualInput.Visual
	if visual == "" {
		visual = legacy.Visual
	}
	motion := make([]string, 0, 6)
	motion = append(motion, legacy.Motion)
	motion = append(motion, visualInput.Motion)
	motion = append(motion, motionInput.Motion)
	motion = append(motion, motionInput.Visual)

	forbidden := make([]string, 0, 12)
	forbidden = append(forbidden, legacy.Forbidden...)
	forbidden = append(forbidden, visualInput.Forbidden...)
	forbidden = append(forbidden, motionInput.Forbidden...)
	forbidden = append(forbidden, normalizeForbiddenStyleItems(snapshot.StyleForbiddenItems)...)

	return customStyleFields{
		Visual:    sanitizeStyleNames(strings.Join(uniqueCustomStyleItems(visual), "，")),
		Motion:    sanitizeStyleNames(strings.Join(uniqueCustomStyleItems(motion...), "；")),
		Forbidden: uniqueCustomStyleItems(forbidden...),
	}
}

func applyCustomStyleFields(snapshot *LockSnapshot) customStyleFields {
	if snapshot == nil {
		return customStyleFields{}
	}
	fields := customStyleFieldsFromSnapshot(*snapshot)
	snapshot.StyleVisualPrompt = fields.Visual
	snapshot.StyleMotionParameters = fields.Motion
	snapshot.StyleForbiddenItems = append([]string(nil), fields.Forbidden...)
	snapshot.StylePromptParameters = fields.Visual
	return fields
}

func normalizeForbiddenStyleItems(values []string) []string {
	result := make([]string, 0, len(values))
	for _, raw := range values {
		raw = normalizeCustomStyleLineBreaks(raw)
		for _, line := range strings.Split(raw, "\n") {
			for _, clause := range splitCustomStyleClauses(line) {
				clause = cleanForbiddenStyleItem(clause)
				if clause != "" {
					result = append(result, clause)
				}
			}
		}
	}
	return uniqueCustomStyleItems(result...)
}

func cleanForbiddenStyleItem(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "风格禁止项：")
	value = strings.TrimPrefix(value, "风格禁止项:")
	for _, prefix := range []string{
		"避免出现", "严禁出现", "禁止出现", "严禁", "禁止", "禁用", "拒绝",
		"排除", "避免", "不得", "不可", "不含", "不要", "去掉", "无需",
	} {
		if strings.HasPrefix(value, prefix) {
			value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
			value = strings.TrimLeft(value, "：:，,；; ")
			break
		}
	}
	return strings.TrimSpace(value)
}

func prependStylePrompt(stylePrompt, taskPrompt string) string {
	stylePrompt = strings.TrimSpace(stylePrompt)
	taskPrompt = strings.TrimSpace(taskPrompt)
	if stylePrompt == "" {
		return taskPrompt
	}
	if taskPrompt == "" {
		return stylePrompt
	}
	return stylePrompt + "\n\n【本任务｜在全局风格锁内执行】\n" + taskPrompt
}

func annotateStyleBase(stylePrompt, styleBase string) string {
	stylePrompt = strings.TrimSpace(stylePrompt)
	base := styleBaseModelDirective(styleBase)
	if stylePrompt == "" {
		return base
	}
	const header = "【STYLE｜视觉风格 · 全片锁死】"
	if strings.HasPrefix(stylePrompt, header) {
		return header + "\n" + base + strings.TrimPrefix(stylePrompt, header)
	}
	return base + "\n" + stylePrompt
}

func styleBaseModelDirective(styleBase string) string {
	switch strings.TrimSpace(styleBase) {
	case StyleBaseLiveAction:
		return "影像基底：真人实拍质感，真实摄影机成像，自然皮肤、真实材质与现场光响应"
	case StyleBase2DAnimation:
		return "影像基底：2D 手绘动画，统一线稿、平涂或厚涂介质，禁止混入真人毛孔与 3D 塑料质感"
	case StyleBase3DCG:
		return "影像基底：3D CG 动画，统一三维建模骨相与 PBR 材质，禁止退化为 2D 平涂或真人实拍"
	case StyleBaseInk2D:
		return "影像基底：水墨二维，宣纸、墨阶、笔触与留白体系统一，禁止退化为普通彩漫"
	default:
		return "影像基底：按 P0 锁定介质统一，禁止跨媒介混用"
	}
}

func hydrateLockStylePrompts(snapshot *LockSnapshot) error {
	if snapshot == nil {
		return nil
	}
	if strings.TrimSpace(snapshot.StyleCardName) == "" {
		if card, ok := getStyleCardByID(strings.TrimSpace(snapshot.StyleCardID)); ok {
			snapshot.StyleCardName = card.Name
			snapshot.StyleFamily = card.Family
			snapshot.StyleFightRating = card.FightRating
			snapshot.StyleEffectRating = card.EffectRating
			snapshot.StyleGrade = card.Grade
			snapshot.StyleCatalogVersion = StyleCatalogVersion
		}
	}
	if strings.TrimSpace(snapshot.StylePromptParameters) == "" {
		snapshot.StylePromptParameters = strings.TrimSpace(snapshot.StyleCustomDescription)
	}
	customStyle := strings.TrimSpace(snapshot.StyleCardName) == "自定义画风" ||
		strings.TrimSpace(snapshot.StyleCardID) == StyleCustomCardID
	if customStyle {
		legacyNeedsMigration := strings.TrimSpace(snapshot.StyleVisualPrompt) == "" ||
			strings.TrimSpace(snapshot.StyleMotionParameters) == "" ||
			snapshot.StyleForbiddenItems == nil
		applyCustomStyleFields(snapshot)
		if legacyNeedsMigration {
			snapshot.StyleImagePrompt = ""
		}
	}
	if strings.TrimSpace(snapshot.StyleImagePrompt) != "" {
		return nil
	}
	if customStyle {
		stylePrompt, performancePrompt := genericStylePromptBlocks(
			snapshot.StyleVisualPrompt,
			snapshot.StyleMotionParameters,
		)
		if analysis := strings.TrimSpace(snapshot.StyleReferenceAnalysis); analysis != "" {
			stylePrompt = strings.TrimSpace(stylePrompt) + "\n" +
				"参考图风格分析：" + sanitizeStyleNames(analysis) + "\n" +
				"参考图职责：只迁移光线、色彩、材质、镜头与后期质感；不复刻构图、人物身份、服装款式和具体场景。"
		}
		snapshot.StyleImagePrompt = annotateStyleBase(stylePrompt, snapshot.StyleBase)
		snapshot.StylePerformancePrompt = performancePrompt
		return nil
	}
	stylePrompt, performancePrompt, err := compileStylePromptBlock(
		snapshot.StyleCardName,
		snapshot.StylePromptParameters,
		snapshot.StyleReferenceAnalysis,
	)
	if err != nil {
		return err
	}
	snapshot.StyleImagePrompt = annotateStyleBase(stylePrompt, snapshot.StyleBase)
	snapshot.StylePerformancePrompt = performancePrompt
	return nil
}
