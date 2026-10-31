package manju

import (
	"errors"
	"strings"
	"testing"
)

func TestV7426AssetPatchRejectsStringCharacterStates(t *testing.T) {
	content := `# P2 资产交付

<!-- MANJU_PROJECT_STATE_PATCH_JSON
{"characters":{"梁承业":{"id":"CHR-001","fig_no":1,"char_states":{"六骑求援":"窄袖骑装，抓令牌与书信，冲出城门"}}},"scenes":{"城门":{"id":"SCN-01","fig_no":2}},"props":{}}
-->`

	err := validateRequiredManjuProjectStatePatch("asset:asset_compile", content)
	if !errors.Is(err, ErrInvalidProjectState) {
		t.Fatalf("string character state must fail the artifact contract, got %v", err)
	}
	if !strings.Contains(err.Error(), "char_states.六骑求援") {
		t.Fatalf("error must identify the invalid character state, got %v", err)
	}
}

func TestV7426AssetPatchAcceptsObjectCharacterStates(t *testing.T) {
	content := `# P2 资产交付

<!-- MANJU_PROJECT_STATE_PATCH_JSON
{"characters":{"梁承业":{"id":"CHR-001","fig_no":1,"char_states":{"S1_helmet":{"item":"头盔","action":"摘下头盔","consequence":"额前头发被压扁"}}}},"scenes":{"城门":{"id":"SCN-01","fig_no":2}},"props":{}}
-->`

	if err := validateRequiredManjuProjectStatePatch("asset:asset_compile", content); err != nil {
		t.Fatalf("object character state must pass the artifact contract: %v", err)
	}
}

func TestV7426P2ProjectStateInstructionUsesOfficialAssetBlocks(t *testing.T) {
	instruction := projectStatePatchInstruction("P2")
	if !strings.Contains(instruction, "官方“图N”资产块") {
		t.Fatalf("P2 must derive state from official asset blocks:\n%s", instruction)
	}
	if strings.Contains(instruction, "MANJU_PROJECT_STATE_PATCH_JSON") {
		t.Fatalf("P2 must not require a platform-only JSON block:\n%s", instruction)
	}
}

func TestP1OfficialAnchorCardsCompileWithoutPlatformJSON(t *testing.T) {
	content := `# P1 剧本交付

【世界锚卡 WORLD_ANCHOR_CARD】
年份/朝代：南宋·绍兴十年
地域：太原城
技术时代：冷兵器时代
社会形态：宋金交战
⛔ 禁用元素清单：玻璃幕墙、电线杆、智能手机
✅ 必现元素清单：城砖、甲片、火把

### 4.1 CHR-001 梁秉文（S）
年龄段：42~48 岁
身高/体型：178cm·清瘦挺直
身份外观三件套：官袍、旧甲、玉带
Voice Description：low steady Mandarin, short commands

### 2.1 CHR-001 梁秉文
年龄段：42~48 岁
身高/体型：178cm·清瘦挺直
身份外观三件套：官袍、旧甲、玉带
`
	patch, found, err := resolveManjuProjectStatePatch("script:ip_development", content, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("official P1 anchor cards must compile into project_state without a JSON block")
	}
	world := anyMap(patch["world_anchor"])
	if anyString(world["era_anchor"]) != "南宋·绍兴十年" {
		t.Fatalf("world anchor was not parsed: %+v", world)
	}
	character := anyMap(anyMap(patch["characters"])["梁秉文"])
	if anyString(character["id"]) != "CHR-001" ||
		anyString(character["body_anchor"]) != "178cm·清瘦挺直" ||
		anyString(character["look_3piece"]) != "官袍、旧甲、玉带" ||
		anyString(character["voice_lock"]) != "low steady Mandarin, short commands" {
		t.Fatalf("character anchor card was not parsed: %+v", character)
	}
}

func TestP1HistoricalVisualBoundaryCompilesWorldAnchorWithoutJSON(t *testing.T) {
	content := `# 01_IP_BIBLE.md

## 6. 历史与视觉边界

- 历史锚：宋金战争时期的太原守城
- 技术时代：冷兵器攻城时代；弓弩、砲车、撞城车
- 必现元素：宋制甲胄；夯土砖石城墙；火把油灯天光
- 禁用元素：现代枪械；电线与电灯；玻璃幕墙
`
	patch, found, err := resolveManjuProjectStatePatch("script:ip_development", content, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("historical and visual boundary must compile into the world anchor")
	}
	world := anyMap(patch["world_anchor"])
	if anyString(world["era_anchor"]) != "宋金战争时期的太原守城" {
		t.Fatalf("historical anchor was not parsed: %+v", world)
	}
	if len(anySlice(world["forbidden_elements"])) != 3 || len(anySlice(world["required_elements"])) != 3 {
		t.Fatalf("historical and visual boundary lists were not parsed: %+v", world)
	}
}

func TestP2OfficialAssetBlocksCompileWithoutPlatformJSON(t *testing.T) {
	content := `# P2 资产交付

#### 图1 · 梁秉文（角色四格）
16:9 角色四格资产板。
年龄段：42~48 岁
身高/体型：178cm·清瘦挺直
身份外观三件套：官袍、旧甲、玉带
Voice Description：low steady Mandarin

#### 图11 · 城墙与垛口（场景母版）
16:9 无人空镜。
坐标系：北为城外，南为城内。
SET_STATE：黎明冷灰、战时烟尘。
空气介质：薄雾与浮尘。
光照：东方冷灰天光。

#### 图11 · 城墙与垛口（场景九宫格）
全图共 9 格。

#### 图19 · 长剑（道具设定图）
道具设定图。
材质：精钢剑身、旧木剑鞘。
`
	patch, found, err := resolveManjuProjectStatePatch("asset:asset_compile", content, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("official P2 asset blocks must compile into project_state without a JSON block")
	}
	character := anyMap(anyMap(patch["characters"])["梁秉文"])
	if anyString(character["id"]) != "CHR-01" || anyString(character["fig_no"]) != "1" {
		t.Fatalf("character asset block was not parsed: %+v", character)
	}
	scene := anyMap(anyMap(patch["scenes"])["城墙与垛口"])
	if anyString(scene["id"]) != "SCN-11" || anyString(scene["fig_no"]) != "11" {
		t.Fatalf("scene master and grid must merge into one scene asset: %+v", scene)
	}
	prop := anyMap(anyMap(patch["props"])["长剑"])
	if anyString(prop["id"]) != "PRP-19" || anyString(prop["material_desc"]) == "" {
		t.Fatalf("prop asset block was not parsed: %+v", prop)
	}
}

func TestProjectStateInstructionsDoNotRequirePlatformJSONForOfficialArtifacts(t *testing.T) {
	for _, stage := range []string{"P1", "P2", "P3"} {
		instruction := projectStatePatchInstruction(stage)
		if strings.Contains(instruction, "MANJU_PROJECT_STATE_PATCH_JSON") {
			t.Fatalf("%s must not require a platform-only JSON block:\n%s", stage, instruction)
		}
	}
}
