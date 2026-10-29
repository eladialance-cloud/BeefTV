package manju

import (
	"strings"
	"testing"
)

func TestFilterCompiledManjuFeedKeepsOnlySegmentAssets(t *testing.T) {
	stateJSON := `{
		"locks":{"asset_mode":"M1"},
		"characters":{
			"林风":{"fig_no":2,"age_anchor":"28岁男性","body_anchor":"精悍体态","look_3piece":"黑色短打","voice_lock":"低沉男声","char_states":{"s1":{"item":"段1握刀 图5","action":"握紧断刀","consequence":"刀刃朝向改变"}}},
			"路人甲":{"fig_no":100,"age_anchor":"30岁男性","body_anchor":"普通体态","look_3piece":"布衣","voice_lock":"无台词","char_states":{}},
			"场记":{"fig_no":102,"age_anchor":"35岁男性","body_anchor":"普通体态","look_3piece":"深色外套","voice_lock":"普通男声","char_states":{"generic":{"item":"手部状态","action":"抬起右手","consequence":"覆盖角色参考图"}}}
		},
		"scenes":{
			"长廊":{"fig_no":1,"coords":"南北向长廊","set_states":{"default":"冷白顶光"},"media_layer":"薄尘","lighting":"冷白顶光·硬光·暗部冷灰补色"}
		},
		"props":{
			"断刀":{"fig_no":3,"material_desc":"钢制断刃"},
			"云梯":{"fig_no":101,"material_desc":"木制长梯"}
		},
		"segments":[{
			"seg_id":"段1",
			"scene":"长廊",
			"segment_type":"对话戏",
			"standing":["林风在长廊中央"],
			"shots":[{"no":1,"start":0,"end":5,"spec":"中景 · 固定","action":"林风握着断刀向前","sound":"脚步","dialogue":"无"}],
			"sfx_summary":"脚步"
		}]
	}`
	feed := `> 素材形态：M1

## 【投喂单元 1/1 · 段1】
` + "```text" + `
【画幅风格】9:16 横屏（aspect_ratio=9:16）
【场景资产】@长廊（参考图1）；@PRP-断刀（参考图3）；@PRP-云梯（参考图101）
【核心人物】@林风（参考图2，声线锁：低沉男声）；@路人甲（参考图100，声线锁：无台词）
【站位声明】林风在长廊中央
【时间轴分镜】
00:00-00:05 [镜头1] 中景 · 固定。林风握着断刀向前。声音：脚步。台词：无。
【音效】脚步
【强制禁止项】画面：禁止穿模。
` + "```" + `

## 素材接线单（M1 编号占位必备 · 平台挂图指南）
旧接线单
`

	filtered, err := filterCompiledManjuFeed(feed, stateJSON)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"@//scn/ - 长廊（图1）",
		"@//chr/ - 林风（图2",
		"@//prp/ - 断刀（图3）",
		"| 图1 | 场景·长廊",
		"| 图2 | 角色·林风",
		"| 图3 | 道具·断刀",
	} {
		if !strings.Contains(filtered, required) {
			t.Fatalf("按段裁剪结果缺少 %q:\n%s", required, filtered)
		}
	}
	for _, forbidden := range []string{"路人甲", "场记", "云梯", "图5", "图100", "图101", "图102", "旧接线单", "aspect_ratio"} {
		if strings.Contains(filtered, forbidden) {
			t.Fatalf("按段裁剪结果残留 %q:\n%s", forbidden, filtered)
		}
	}
}

func TestFilterCompiledManjuFeedScopesCharacterStatesToTheirSegments(t *testing.T) {
	stateJSON := `{
		"locks":{"asset_mode":"M1"},
		"characters":{
			"林风":{
				"fig_no":1,
				"voice_lock":"低沉男声",
				"char_states":{
					"seg_1_detached":{"item":"金丝眼镜","action":"摘下金丝眼镜","consequence":"鼻梁留有压痕"},
					"S2_helmet":{"item":"黑色头盔","action":"摘下黑色头盔","consequence":"额前头发被压扁"},
					"unscoped":{"item":"手部状态","action":"抬起右手","consequence":"没有生效区间"}
				}
			}
		},
		"scenes":{"旧宅":{"fig_no":2}},
		"segments":[
			{"seg_id":"段1","scene":"旧宅","standing":["林风在门边"],"shots":[{"no":1,"start":0,"end":4,"action":"林风推门","dialogue":"无"}]},
			{"seg_id":"段2","scene":"旧宅","standing":["林风在厅中"],"shots":[{"no":1,"start":0,"end":4,"action":"林风摘头盔","dialogue":"无"}]},
			{"seg_id":"段3","scene":"旧宅","standing":["林风在桌旁"],"shots":[{"no":1,"start":0,"end":4,"action":"林风落座","dialogue":"无"}]}
		]
	}`
	feed := `> 素材形态：M1

## 【投喂单元 1/3 · 段1】
` + "```text" + `
【画幅风格】16:9 横屏（aspect_ratio=16:9）
【场景资产】@旧宅
【核心人物】@林风
【站位声明】林风在门边
【时间轴分镜】
00:00-00:04 [镜头1] 中景 · 24mm · 平视 · 固定。林风推门。声音：木门。台词：无。
【音效】木门
【强制禁止项】画面：禁字幕。
` + "```" + `

## 【投喂单元 2/3 · 段2】
` + "```text" + `
【画幅风格】16:9 横屏（aspect_ratio=16:9）
【场景资产】@旧宅
【核心人物】@林风
【站位声明】林风在厅中
【时间轴分镜】
00:00-00:04 [镜头1] 中景 · 24mm · 平视 · 固定。林风摘下头盔。声音：衣料。台词：无。
【音效】衣料
【强制禁止项】画面：禁字幕。
` + "```" + `

## 【投喂单元 3/3 · 段3】
` + "```text" + `
【画幅风格】16:9 横屏（aspect_ratio=16:9）
【场景资产】@旧宅
【核心人物】@林风
【站位声明】林风在桌旁
【时间轴分镜】
00:00-00:04 [镜头1] 中景 · 24mm · 平视 · 固定。林风落座。声音：衣料。台词：无。
【音效】衣料
【强制禁止项】画面：禁字幕。
` + "```" + `
`
	filtered, err := filterCompiledManjuFeed(feed, stateJSON)
	if err != nil {
		t.Fatal(err)
	}
	unit1 := betweenManjuUnitFence(t, unitSectionByCode(t, filtered, "段1"))
	unit2 := betweenManjuUnitFence(t, unitSectionByCode(t, filtered, "段2"))
	unit3 := betweenManjuUnitFence(t, unitSectionByCode(t, filtered, "段3"))
	if !strings.Contains(unit1, "金丝眼镜") || strings.Contains(unit1, "黑色头盔") {
		t.Fatalf("段1 只应包含 seg_1 状态:\n%s", unit1)
	}
	if strings.Contains(unit2, "金丝眼镜") || !strings.Contains(unit2, "黑色头盔") {
		t.Fatalf("段2 只应包含 S2 状态:\n%s", unit2)
	}
	for _, forbidden := range []string{"金丝眼镜", "黑色头盔", "手部状态"} {
		if strings.Contains(unit3, forbidden) {
			t.Fatalf("段3 不应继承其他段或未声明区间的状态 %q:\n%s", forbidden, unit3)
		}
	}
}

func unitSectionByCode(t *testing.T, feed, code string) string {
	t.Helper()
	marker := "· " + code + "】"
	start := strings.Index(feed, marker)
	if start < 0 {
		t.Fatalf("缺少投喂单元 %s:\n%s", code, feed)
	}
	start = strings.LastIndex(feed[:start], "\n## ") + 1
	end := len(feed)
	if next := strings.Index(feed[start+4:], "\n## "); next >= 0 {
		end = start + 4 + next
	}
	return feed[start:end]
}

func TestProductionSegmentVisualAssetCodesIncludesP2Assets(t *testing.T) {
	state := map[string]any{
		"characters": map[string]any{
			"林风": map[string]any{"id": "CHR-林澈"},
			"路人": map[string]any{"id": "CHR-路人"},
		},
		"scenes": map[string]any{
			"长廊": map[string]any{"id": "SCN-长廊"},
		},
		"props": map[string]any{
			"断刀": map[string]any{"id": "PRP-断刀"},
			"云梯": map[string]any{"id": "PRP-云梯"},
		},
	}
	segment := map[string]any{
		"seg_id":   "段1",
		"scene":    "长廊",
		"standing": []any{"林风在长廊中央握着断刀"},
		"shots": []any{map[string]any{
			"spec": "中景", "action": "林风握刀向前", "sound": "脚步", "dialogue": "无",
		}},
	}
	codes := productionSegmentVisualAssetCodes(state, segment, 8)
	for _, required := range []string{"CHR-林澈", "SCN-长廊", "PRP-断刀"} {
		found := false
		for _, code := range codes {
			if code == required {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("P4 分段视觉资产缺少 %s: %+v", required, codes)
		}
	}
	for _, code := range codes {
		if code == "CHR-路人" || code == "PRP-云梯" {
			t.Fatalf("P4 分段视觉资产混入无关资产 %s: %+v", code, codes)
		}
	}
}

func TestFilterCompiledManjuFeedClearsCharactersForSceneOnlyUnit(t *testing.T) {
	stateJSON := `{
		"locks":{"asset_mode":"M1","model":"2.0"},
		"characters":{
			"梁崇":{"fig_no":1,"voice_lock":"低沉男声","char_states":{}},
			"汶瑾":{"fig_no":2,"voice_lock":"清亮男声","char_states":{}},
			"梁母":{"fig_no":3,"voice_lock":"中年女声","char_states":{}}
		},
		"scenes":{
			"太原城外与城头":{"fig_no":4,"lighting":"晨雾冷光"}
		},
		"props":{},
		"segments":[{
			"seg_id":"段1",
			"scene":"太原城外与城头",
			"segment_type":"定场群像",
			"standing":[],
			"shots":[{"no":1,"start":0,"end":5,"spec":"大远景 · 高空俯角 · 缓慢后拉","action":"晨雾中晋北孤城，城外黑帐连营铺向地平线","sound":"号角、鼓点、风雪","dialogue":"无"}],
			"sfx_summary":"号角、鼓点、风雪"
		}]
	}`
	feed := `> 素材形态：M1

## 【投喂单元 1/1 · 段1】
` + "```text" + `
【画幅风格】16:9 横屏（aspect_ratio=16:9）
【场景资产】@太原城外与城头（参考图4）
【核心人物】@梁崇（参考图1）；@汶瑾（参考图2）；@梁母（参考图3）
【站位声明】
【时间轴分镜】
00:00-00:05 [镜头1] 大远景 · 高空俯角 · 缓慢后拉。晨雾中晋北孤城，城外黑帐连营铺向地平线。声音：号角、鼓点、风雪。台词：无。
【音效】号角、鼓点、风雪
【强制禁止项】画面：禁字幕、禁水印；音频：禁背景音乐、禁旁白。
` + "```" + `
`

	filtered, err := filterCompiledManjuFeed(feed, stateJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filtered, "【核心人物】无（本单元无人入画）") {
		t.Fatalf("scene-only unit must clear unrelated characters:\n%s", filtered)
	}
	for _, unexpected := range []string{"@梁崇", "@汶瑾", "@梁母", "| 图1 |", "| 图2 |", "| 图3 |"} {
		if strings.Contains(filtered, unexpected) {
			t.Fatalf("scene-only unit retained %q:\n%s", unexpected, filtered)
		}
	}
	if !strings.Contains(filtered, "@//scn/ - 太原城外与城头（图4）") {
		t.Fatalf("scene reference was lost:\n%s", filtered)
	}
}

func TestVisualCanvasCodesAcceptV7426Namespaces(t *testing.T) {
	codes := visualCanvasCodes(
		"【场景资产】@//scn/ - 长廊（图1）",
		"【核心人物】@//chr/ - 林风（图2，声线锁：低沉男声）",
		"【道具】@//prp/ - 断刀（图3）",
	)
	for _, required := range []string{"SCN-长廊", "CHR-林风", "PRP-断刀"} {
		found := false
		for _, code := range codes {
			if code == required {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("V7.4.26 资产命名空间未解析出 %s: %v", required, codes)
		}
	}
}

func TestFilterCompiledManjuFeedMatchesShortCharacterNames(t *testing.T) {
	stateJSON := `{
		"locks":{"asset_mode":"M1","model":"2.5"},
		"characters":{
			"梁汶瑾":{"fig_no":1,"voice_lock":"soft timbre","char_states":{}},
			"梁汶玥":{"fig_no":2,"voice_lock":"clear young female","char_states":{}}
		},
		"scenes":{
			"垛口（城头）":{"fig_no":17,"lighting":"火把暖光"}
		},
		"props":{},
		"segments":[{
			"seg_id":"段1",
			"scene":"垛口（城头）",
			"segment_type":"climax",
			"standing":["汶瑾站垛口旁","汶玥跪坐"],
			"shots":[{"no":1,"start":0,"end":4,"spec":"中景 · 正面平视 · 固定","action":"汶瑾转身挡在汶玥前","sound":"风雪","dialogue":"拿着——！"}],
			"sfx_summary":"风雪"
		}]
	}`
	feed := `> 素材形态：M1

## 【投喂单元 1/1 · 段1】
` + "```text" + `
【画幅风格】9:16 竖屏（aspect_ratio=9:16）
【场景资产】@垛口（城头）（参考图17）
【核心人物】@梁汶瑾（参考图1）；@梁汶玥（参考图2）
【站位声明】汶瑾站垛口旁；汶玥跪坐
【时间轴分镜】
00:00-00:04 [镜头1] 中景 · 正面平视 · 固定。汶瑾转身挡在汶玥前。声音：风雪。台词：拿着——！。
【音效】风雪
【强制禁止项】画面：禁止角色面部变形或肢体穿模。
` + "```" + `
`

	filtered, err := filterCompiledManjuFeed(feed, stateJSON)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"@//chr/ - 梁汶瑾（图1",
		"@//chr/ - 梁汶玥（图2",
		"| 图1 | 角色·梁汶瑾",
		"| 图2 | 角色·梁汶玥",
	} {
		if !strings.Contains(filtered, required) {
			t.Fatalf("短名角色未绑定完整角色资产 %q:\n%s", required, filtered)
		}
	}
	if strings.Contains(filtered, "【核心人物】无（本单元无人入画）") {
		t.Fatalf("短名角色被错误清空:\n%s", filtered)
	}
}

func TestFilterCompiledManjuFeedResolvesSemanticSceneNameWithoutLeakingProps(t *testing.T) {
	stateJSON := `{
		"locks":{"asset_mode":"M1","model":"2.5"},
		"characters":{
			"梁崇":{"fig_no":3,"voice_lock":"stone voice","char_states":{}}
		},
		"scenes":{
			"梁家宗祠":{"fig_no":13,"lighting":"烛火暖色与青灰冷色对撞"}
		},
		"props":{
			"叔父家书":{"fig_no":26,"material_desc":"泛黄信纸"},
			"五支笔":{"fig_no":27,"material_desc":"竹杆毛笔"}
		},
		"segments":[{
			"seg_id":"段1",
			"scene":"宗祠之夜",
			"segment_type":"对话戏",
			"standing":["梁崇居中"],
			"shots":[{"no":1,"start":0,"end":4,"spec":"中景 · 正面平视 · 固定","action":"梁崇立于牌位前","sound":"风雪","dialogue":"只许登科"}],
			"sfx_summary":"风雪"
		}]
	}`
	feed := `> 素材形态：M1

## 【投喂单元 1/1 · 段1】
` + "```text" + `
【画幅风格】9:16 横屏（aspect_ratio=9:16）
【场景资产】@宗祠之夜（；；）；@PRP-叔父家书（参考图26）；@PRP-五支笔（参考图27）
【核心人物】@梁崇（参考图3，声线锁：stone voice）
【站位声明】梁崇居中
【时间轴分镜】
00:00-00:04 [镜头1] 中景 · 正面平视 · 固定。梁崇立于牌位前。声音：风雪。台词：只许登科。
【音效】风雪
【强制禁止项】画面：禁止角色面部变形或肢体穿模。
` + "```" + `
`

	filtered, err := filterCompiledManjuFeed(feed, stateJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filtered, "@//scn/ - 梁家宗祠（图13）") {
		t.Fatalf("语义场景名未映射到 P2 场景资产:\n%s", filtered)
	}
	for _, unexpected := range []string{"@宗祠之夜", "@PRP-叔父家书", "参考图26", "参考图27"} {
		if strings.Contains(filtered, unexpected) {
			t.Fatalf("未实际引用的道具泄漏进场景资产 %q:\n%s", unexpected, filtered)
		}
	}
	if !strings.Contains(filtered, "| 图13 | 场景·梁家宗祠") {
		t.Fatalf("场景资产未进入 M1 接线单:\n%s", filtered)
	}
}

func TestFilterCompiledManjuFeedDoesNotAlterShotText(t *testing.T) {
	stateJSON := `{
		"locks":{"asset_mode":"M1","model":"2.5"},
		"characters":{
			"林风":{"fig_no":1,"voice_lock":"低沉微哑青年男声","char_states":{}}
		},
		"scenes":{
			"地下拳馆走廊":{"fig_no":2,"lighting":"顶灯冷白硬光·暗部冷灰补色"}
		},
		"props":{},
		"segments":[{
			"seg_id":"段1",
			"scene":"地下拳馆走廊",
			"segment_type":"战斗动作段",
			"standing":["林风位于画面左侧，背靠混凝土墙面"],
			"shots":[
				{"no":1,"start":0,"end":4,"spec":"中近景 · 侧45度 · 缓推","action":"林风右手拇指攥紧战术手套，听到名字后手指骤停，下颌咬肌绷紧，目光钉向对手。","sound":"[SFX: 皮质摩擦声]","dialogue":"【林风（压着怒意）：“陈会长，这表你认得吧？”】"},
				{"no":2,"start":4,"end":9,"spec":"中景 · 侧45度 · 跟移","action":"林风沉腰屈膝，右脚猛蹬地面，拧胯转腰送肩轰出重拳，陈枭交叉架臂卸力，胸口衣料受击凹陷，整个人向后滑退半步。","sound":"[SFX: 拳肉撞击声]","dialogue":"无"},
				{"no":3,"start":9,"end":15,"spec":"全景 · 斜侧仰视 · 缓慢拉远","action":"林风与陈枭隔着水泥走廊对峙，顶灯硬光拉出长影，两人呼吸声逐渐加重，画面停住。","sound":"[SFX: 呼吸回音]","dialogue":"无"}
			],
			"sfx_summary":"拳肉撞击、鞋底滑水、粗重呼吸"
		}]
	}`
	const actionAnchor = "镜末动作锚点保留图7"
	verboseAction := strings.Join([]string{
		"林风先以极慢的速度压低重心，肩背肌肉保持绷紧，视线从地面抬到对手下颌",
		"随后右脚向前踏出半步，膝盖弯曲，腰胯同步转动，拳头沿身体中线爆发推出",
		"陈枭双臂在胸前交叉，前臂与来拳正面接触，西装布料向内凹陷，脚下碾过浅水洼",
		"撞击后两人同时停顿，呼吸、衣料摩擦、鞋底滑水与远处电流嗡鸣重叠成连续环境声",
		"镜头保持侧向观察，背景墙面、铁门、灯管与地面反光都保持固定空间关系",
	}, "；") + "；" + actionAnchor
	shotLines := make([]string, 0, 4)
	for index, timeRange := range []string{"00:00-00:04", "00:04-00:08", "00:08-00:12", "00:12-00:16"} {
		dialogue := "无"
		if index == 0 {
			dialogue = "【林风（压着怒意·低沉微哑）：“陈会长，这表你认得吧？”】"
		}
		shotLines = append(shotLines, timeRange+" [镜头"+string(rune('1'+index))+"] 中近景 · 侧45度 · 缓推。"+
			strings.Repeat(verboseAction+"；", 4)+"动作收束。声音：[SFX: 撞击与呼吸]。台词："+dialogue+"。")
	}
	feed := `> 素材形态：M1

## 【投喂单元 1/1 · 段1】
` + "```text" + `
【画幅风格】9:16 竖屏，古代战场，电影级写实，战斗动作段
【场景资产】@地下拳馆走廊（参考图2）
【核心人物】@林风（参考图1，声线锁：低沉微哑青年男声）
【站位声明】林风位于画面左侧，背靠混凝土墙面
【时间轴分镜】
` + strings.Join(shotLines, "\n") + `
【音效】拳肉撞击、鞋底滑水、粗重呼吸
【强制禁止项】画面：禁止角色面部变形或肢体穿模；文字：无字幕、无水印、无LOGO；音频：禁止背景音乐盖过人声。
` + "```" + `
`

	filtered, err := filterCompiledManjuFeed(feed, stateJSON)
	if err != nil {
		t.Fatal(err)
	}
	content := betweenManjuUnitFence(t, filtered)
	if got := len([]rune(content)); got <= 2600 {
		t.Fatalf("测试稿应保持超预算，供官方机检明确拒绝，实际 %d 字", got)
	}
	if !strings.Contains(content, actionAnchor) {
		t.Fatalf("P4 后处理不得删除镜头动作或正文图号:\n%s", content)
	}
	const dialogue = "【林风（压着怒意·低沉微哑）：“陈会长，这表你认得吧？”】"
	if !strings.Contains(content, dialogue) {
		t.Fatalf("P4 后处理不得删除或改写逐字台词:\n%s", content)
	}
	for _, required := range []string{
		"00:00-00:04 [镜头1] 中近景 · 侧45度 · 缓推。",
		"00:12-00:16 [镜头4] 中近景 · 侧45度 · 缓推。",
		"声音：[SFX: 撞击与呼吸]。",
		"【音效】拳肉撞击、鞋底滑水、粗重呼吸",
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("P4 后处理破坏了必需结构 %q:\n%s", required, content)
		}
	}
}

func betweenManjuUnitFence(t *testing.T, feed string) string {
	t.Helper()
	start := strings.Index(feed, "```text")
	if start < 0 {
		t.Fatalf("feed has no unit fence:\n%s", feed)
	}
	start += len("```text")
	if newline := strings.IndexByte(feed[start:], '\n'); newline >= 0 {
		start += newline + 1
	}
	end := strings.Index(feed[start:], "```")
	if end < 0 {
		t.Fatalf("feed unit fence is not closed:\n%s", feed)
	}
	return feed[start : start+end]
}
