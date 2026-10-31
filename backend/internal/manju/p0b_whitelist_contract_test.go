package manju

import (
	"strings"
	"testing"
)

func TestP0BAcceptsCanonicalWhitelistTables(t *testing.T) {
	if err := validateP0BDecisionArtifact(canonicalP0BArtifact()); err != nil {
		t.Fatalf("canonical A/B whitelist tables must pass: %v", err)
	}
}

func TestP0BRejectsBClassRowWithoutCanonicalHazardName(t *testing.T) {
	content := strings.Replace(
		canonicalP0BArtifact(),
		"| B-04 | 比例失调 | 永不解锁 |",
		"| B-04 | 身体比例异常 | 永不解锁 |",
		1,
	)
	err := validateP0BDecisionArtifact(content)
	if err == nil || !strings.Contains(err.Error(), "B-04") || !strings.Contains(err.Error(), "比例失调") {
		t.Fatalf("B-04 must use its canonical hazard name, got %v", err)
	}
}

func TestP0BRejectsWhitelistUsageMismatch(t *testing.T) {
	content := strings.Replace(
		canonicalP0BArtifact(),
		"A1 正面居中（第⑥项权力凝固）",
		"—",
		1,
	)
	err := validateP0BDecisionArtifact(content)
	if err == nil || !strings.Contains(err.Error(), "A1") || !strings.Contains(err.Error(), "B02") {
		t.Fatalf("A1 declaration must match actual white-list usage, got %v", err)
	}
}

func canonicalP0BArtifact() string {
	return `# P0B 决策锚点

## 情绪锚点表

【大分镜 #B02 · 情绪锚点表】

| 节拍 # | 情绪关键词 | 服务的剧本动作 | 情境 ID | 核心组合摘要（80%定死） | 解锁雷区 |
|---|---|---|---|---|---|
| 节拍 1 | 强敌压阵 | 金军主帅勒马抬手 | E02 | 全景·低角度·回流剪影·静态 | A1 正面居中（第⑥项权力凝固） |
| 节拍 2 | 号令压下 | 手落号角齐鸣 | E06 | 全景·手持快摇 | — |
| 节拍 3 | 箭雨泼城 | 万箭齐发 | E03 | 手持呼吸感·长焦快切 | — |

【大分镜 #B11 · 情绪锚点表】

| 节拍 # | 情绪关键词 | 服务的剧本动作 | 情境 ID | 核心组合摘要（80%定死） | 解锁雷区 |
|---|---|---|---|---|---|
| 节拍 1 | 城头喊话 | 梁秉文对百姓大喊 | E10 | 对称居中·长焦·静态 | A1 正面居中（第②项仪式感）；A7 多人正面平铺（第①项集体宣誓） |
| 节拍 2 | 死寂一息 | 万众被守字压住 | E10 | 对称居中·长焦·静态 | A1 正面居中（第②项仪式感）；A7 多人正面平铺（第①项集体宣誓） |
| 节拍 3 | 万人齐吼 | 万人齐吼守 | E10 | 对称居中·长焦·静态 | A1 正面居中（第②项仪式感）；A7 多人正面平铺（第①项集体宣誓） |

## 构图雷区白名单

### A 类 · 艺术性雷区（可条件解锁）

| 编号 | 雷区 | 判定 | 官方白名单选项 | 配套约束 | 适用大分镜 |
|---|---|---|---|---|---|
| A1 | 正面居中 | 条件解锁 | ⑥ 权力凝固；② 仪式感 | 对称布景、静态运镜、长焦压缩 | B02、B11 |
| A2 | 头顶贴边 | 不解锁 | — | — | — |
| A3 | 死压中线 | 不解锁 | — | — | — |
| A4 | 眼神无空间 | 不解锁 | — | — | — |
| A5 | 断关节 | 不解锁 | — | — | — |
| A6 | 背景过亮 | 不解锁 | — | — | — |
| A7 | 多人正面平铺 | 条件解锁 | ① 全家福 / 法庭 / 仪式 / 集体宣誓 | 对称布景、仪式感、集体凝视 | B11 |

### B 类 · 工程性雷区（永不解锁）

| 编号 | 规范雷区名称 | 判定 | 强制限制 | 适用范围 |
|---|---|---|---|---|
| B-01 | 物体穿身 | 永不解锁 | 限制：背景物体不与角色身体重叠 | 全片 |
| B-02 | 角色服装突变 | 永不解锁 | 限制：服装与发型和上一镜完全一致 | 全片 |
| B-03 | 杂乱前景刺眼物 | 永不解锁 | 限制：前景虚化无刺眼光斑与异物 | 全片 |
| B-04 | 比例失调 | 永不解锁 | 限制：身体各部位比例符合解剖学 | 全片 |
| B-05 | 重复元素无重点 | 永不解锁 | 限制：背景元素有主次层次 | 全片 |
| B-06 | 空鸡蛋构图 | 永不解锁 | 限制：增加前景物件或框架式构图 | 全片 |
| B-07 | 道具状态突变 | 永不解锁 | 限制：道具状态与上一镜连续 | 全片 |`
}
