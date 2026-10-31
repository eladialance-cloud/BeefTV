#!/usr/bin/env python3
"""Generate BeefTV's P3 and P4 shared contracts from pinned V7.4.26 sources."""

from __future__ import annotations

import argparse
import ast
import hashlib
import json
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
SKILL_ROOT = ROOT / "backend/internal/manju/skilldata/v7.4.26"
OUTPUT_DIR = ROOT / "backend/internal/manju/output_contracts"
P3_OUTPUT = OUTPUT_DIR / "p3-storyboard-v1.json"
P4_OUTPUT = OUTPUT_DIR / "p4-feed-v1.json"

P3_SOURCES = [
    ("storyboard-engine", "sd-storyboard-engine/SKILL.md"),
    ("storyboard-template", "sd-storyboard-engine/references/storyboard-sheet-template.md"),
    ("prompt-feeding-checklist", "short-drama-director/references/prompt-feeding-checklist.md"),
    ("feed-compiler", "short-drama-director/scripts/compile_feed.py"),
    ("prompt-validator", "short-drama-director/scripts/validate_prompt.py"),
    ("contract-auditor", "short-drama-director/scripts/audit_contract.py"),
]

P4_SOURCES = [
    ("feed-compiler", "short-drama-director/scripts/compile_feed.py"),
    ("prompt-validator", "short-drama-director/scripts/validate_prompt.py"),
    ("gate-checker", "sd-feed-engine/scripts/gate_check.py"),
    ("contract-auditor", "short-drama-director/scripts/audit_contract.py"),
    ("model-adapters", "short-drama-director/references/model-adapters.md"),
    ("prompt-feeding-checklist", "short-drama-director/references/prompt-feeding-checklist.md"),
    ("storyboard-engine", "sd-storyboard-engine/SKILL.md"),
    ("storyboard-template", "sd-storyboard-engine/references/storyboard-sheet-template.md"),
]


def sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def contract_sources(items: list[tuple[str, str]]) -> list[dict[str, str]]:
    sources: list[dict[str, str]] = []
    for role, relative in items:
        path = SKILL_ROOT / relative
        if not path.is_file():
            raise SystemExit(f"official source missing: {path}")
        sources.append(
            {
                "role": role,
                "path": relative,
                "sha256": sha256_file(path),
            }
        )
    return sources


def read_literal_assignments(source: str, names: set[str]) -> dict[str, Any]:
    tree = ast.parse(source)
    result: dict[str, Any] = {}
    for node in tree.body:
        if not isinstance(node, ast.Assign) or len(node.targets) != 1:
            continue
        target = node.targets[0]
        if isinstance(target, ast.Name) and target.id in names:
            result[target.id] = ast.literal_eval(node.value)
    missing = names - result.keys()
    if missing:
        raise SystemExit(f"validator constants missing: {', '.join(sorted(missing))}")
    return result


def build_p3_contract() -> dict[str, Any]:
    validator_source = (
        SKILL_ROOT / "short-drama-director/scripts/validate_prompt.py"
    ).read_text(encoding="utf-8")
    validator_constants = read_literal_assignments(
        validator_source,
        {"GORE_WORDS"},
    )
    return {
        "schemaVersion": "beef.tv/manju/p3-storyboard-contract/v1",
        "rulesVersion": "V7.4.26",
        "sources": contract_sources(P3_SOURCES),
        "requiredSections": [
            "## 故事板表",
            "## 分镜总表",
            "## 台词精简建议清单",
            "## P3 交付清单",
        ],
        "requiredSectionMessages": {
            "## 分镜总表": "P3 成果缺少《分镜总表》",
            "## 故事板表": "P3 成果缺少《故事板表》，故事板必须逐段列出动作相位",
            "## P3 交付清单": "P3 成果缺少《P3 交付清单》",
            "## 台词精简建议清单": "P3 成果缺少《台词精简建议清单》",
        },
        "storyboard": {
            "headingTemplate": "### 第N段｜段落标题",
            "tableHeaders": [
                "段号",
                "面板ID",
                "动作相位",
                "镜头信息",
                "站位与视线",
                "资产引用",
                "面板边界备注",
            ],
            "assetReferenceRule": "资产引用优先写 P2《素材接线单》中的 `图N`",
            "phases": ["起始", "过程", "接触", "顶点", "结果", "反应", "出口"],
            "minimumLayouts": {
                "dialogue": "4格/2×2",
                "action": "6格/2×3",
                "complexBattle": "8~9格/2×4或3×3",
            },
            "panelBoundaryIsNotCut": True,
            "deliveryChecklistFields": [
                "剧情分镜格/覆盖",
                "故事板图格数",
                "具体说明/豁免理由",
            ],
        },
        "timeline": {
            "shotSpec": "景别 · 焦段 · 机位角度 · 运镜（用途标签）",
            "requiredShotFields": ["no", "start", "end", "spec", "action", "sound", "dialogue"],
            "integerSeconds": True,
            "continuous": True,
            "resetEachSegmentToZero": True,
            "dialogueEmptyValue": "无",
            "dialogueRule": "只填写画内真实逐字台词，不得写入流程状态、交接备注或制作说明",
        },
        "assetNamespaces": {
            "scene": "@//scn/",
            "character": "@//chr/",
            "prop": "@//prp/",
        },
        "assetRefs": {
            "required": True,
            "namespaces": ["scenes", "characters", "props"],
            "limitByModel": {"2.5": 30, "2.0": 9, "h3": 9},
            "instruction": (
                "asset_refs 只填写本段实际出现或直接影响画面生成的场景、人物和道具；"
                "不得填写项目全量资产。预算公式：1 个场景 + 去重人物数 + 去重道具数 ≤ 当前模型单段上限；"
                "2.0/H3 上限 9，2.5 上限 30。不得为了保险同时列入群像与其所属成员、"
                "同一资产的多个文字别名或非本段必需的衍生版本。超过上限时，必须回到 P3 "
                "按镜头边界拆分或减少非必要参考，不得交给 P4 自动删除。"
                "asset_refs.character_states 只列出本段真正适用的角色状态键；未列出的状态不得注入。"
            ),
        },
        "safety": {
            "source": "short-drama-director/scripts/validate_prompt.py GORE_WORDS",
            "highRiskWords": validator_constants["GORE_WORDS"],
            "instruction": (
                "P3 正文、分镜动作、台词、音效和 asset_refs 均不得出现上述 C9 高危词。"
                "必须在成句时直接改写为可见、非血腥、非违法或非裸露的物理行为；"
                "禁止用否定式、引号、拼音或谐音保留高危词。"
            ),
        },
        "c23f": {
            "fullBodyCharacterLimit": 1,
            "actionIntentLimit": 2,
            "splitAtShotBoundary": True,
        },
        "styleConstraint": {
            "source": "P0 style lock",
            "lensField": "shots[].spec",
            "rangeOwner": "STYLE block",
            "instructionTemplate": (
                "所有 shot.spec 的焦段必须落在该区间；P0B 核心机位组合与本硬锁冲突时，以本硬锁为准，"
                "保留景别、机位角度和运镜语义，重新选择区间内焦段；越界时必须回 P3 修正，"
                "不得由 P4 或视频生成阶段静默改焦段。"
            ),
        },
        "budget": {
            "softByModel": {"2.5": 2600, "2.0": 1900, "h3": 1900},
            "hardByModel": {"2.5": 3000, "2.0": 2000, "h3": 2000},
            "instruction": (
                "P4 字数处理链：每段可投喂正文预算 Seedance 2.5 ≤2600 字符（硬上限 3000），"
                "Seedance 2.0 / H3 ≤1900 字符（硬上限 2000）。默认 C 导演轨纯净版；先执行 7 步压缩，"
                "只删除管理话术、低收益后缀、重复限制和空泛表达，素材用途、人物锚定、场景位置关系、"
                "景深焦点锁、摄影机行为锁、台词语气、可执行光影、段末状态必须保留。仍超限才按镜头边界拆段；"
                "禁止在单个镜头内部硬拆，拆分点必须落在镜头之间。任意单个 shot 经 P4 展开后仍超预算时，"
                "回 P3 按 prompt-feeding-checklist 逐镜精修：特写与表演幅度绑定、长台词拆镜、"
                "相邻机位不得重复、台词按 6 字/秒校验。不得删除动作、台词、音效或资产内容。"
            ),
        },
        "prompt": {
            "heading": "任务：输出 P3《分镜与故事板表》。",
            "requiredItems": [
                "必须包含 `## 分镜总表`，逐段逐镜列出镜号、整数秒时长，并按 V7.4.26 五要素写镜行：`景别 · 焦段 · 机位角度 · 运镜（用途标签）`，同时填写画面内容、台词、音效和资产引用。",
                "必须包含 `## 故事板表`，每段先用 `### 第N段｜段落标题` 标记，紧接 Markdown 表格，表头固定为 `段号 | 面板ID | 动作相位 | 镜头信息 | 站位与视线 | 资产引用 | 面板边界备注`，逐段逐格填写。",
                "资产引用优先写 P2《素材接线单》中的 `图N`，并兼容 `@//scn/`、`@//chr/`、`@//prp/` 三族命名空间。",
                "`MANJU_PROJECT_STATE_PATCH_JSON` 的每个 segment 必须声明 `asset_refs`，且只填写本段实际引用的人物、场景和道具；不得填写项目全量资产。预算必须满足：1 个场景 + 人物数 + 道具数 ≤ 当前模型单段上限。",
                "`asset_refs.character_states` 必须按人物列出本段适用的状态键；不得把角色的全部 char_states 重复写入每段。",
                "必须包含 `## P3 交付清单`，使用 Markdown 表格逐段列出 `剧情分镜格/覆盖`、`故事板图格数` 和 `具体说明/豁免理由`。",
                "必须包含 `## 台词精简建议清单`，默认全量承接上游台词；要精简只能给建议并等待用户决定。",
                "每段镜头的 start/end 必须从 0 秒重新起算，只表示本投喂单元的段内时长，禁止沿用全剧累计时间码。",
                "不得计算不存在的时长，不得把面板边界当成剪辑点，不得声称图片已经生成。",
                "逐镜执行 C23f：同一镜只建立 1 个全身人物；第二个人物入画或交接必须拆到下一条 shot，单镜动作意图不超过 2 项。禁止“侍卫与护卫入画，全身站位并立”这类单镜双全身引入，必须拆为两条 shot，每条只写一个“画面主体”。",
                "`shots[].dialogue` 只填写画内真实逐字台词；没有逐字台词时固定填写 `无`，不得写入流程状态、交接备注或制作说明。",
            ],
        },
        "projectState": {
            "stageKey": "production:director_storyboard",
            "instruction": (
                "必须按 sd-storyboard-engine 的导演稿与故事板规则，在 Markdown 末尾追加 "
                "MANJU_PROJECT_STATE_PATCH_JSON 回写块。segments 必须覆盖全部投喂段；shots 必须整秒、连续、"
                "可由 compile_feed.py 直接编译，不得写入 locks 或 receipt。每段镜头的 start/end 必须从 0 秒重新起算，"
                "只表示本投喂单元的段内时长，禁止沿用全剧累计时间码；每段末镜 end 不得超当前模型单段上限。"
                "镜行必须按 V7.4.26 五要素写清：景别 · 焦段 · 机位角度 · 运镜（用途标签）。"
                "逐镜执行 C23f：同一镜只建立 1 个全身人物，第二个人物入画或交接必须拆到下一镜，"
                "单镜动作意图不超过 2 项。shots[].dialogue 只填写画内真实逐字台词；没有逐字台词时固定填写“无”，"
                "不得写入流程状态、交接备注或制作说明。每个 segment 必须声明 asset_refs，只填写本段实际出现"
                "或直接影响画面生成的场景、人物和道具；不得填写项目全量资产。预算公式：1 个场景 + 去重人物数"
                "+ 去重道具数 ≤ 当前模型单段上限；2.0/H3 为 9，2.5 为 30。不得为了保险同时列入群像与其成员、"
                "文字别名或非本段必需的衍生版本。超过上限时必须回 P3 按镜头边界拆段或减少非必要参考。"
            ),
            "example": {
                "segments": [
                    {
                        "seg_id": "段1",
                        "scene": "场景名",
                        "asset_refs": {
                            "scenes": ["场景名"],
                            "characters": ["角色名"],
                            "props": [],
                            "character_states": {
                                "角色名": ["状态键"],
                            },
                        },
                        "segment_type": "",
                        "emotion_anchor_id": "",
                        "camera_combo_80": "",
                        "forbidden_zone_unlock": "",
                        "standing": [],
                        "storyboard_panels": [],
                        "shots": [
                            {
                                "no": 1,
                                "start": 0,
                                "end": 4,
                                "spec": "景别 · 焦段 · 机位角度 · 运镜（用途标签）",
                                "action": "",
                                "sound": "",
                                "dialogue": "无",
                            }
                        ],
                        "sfx_summary": "",
                    }
                ]
            },
        },
    }


def build_p4_contract() -> dict[str, Any]:
    validator_source = (
        SKILL_ROOT / "short-drama-director/scripts/validate_prompt.py"
    ).read_text(encoding="utf-8")
    constants = read_literal_assignments(
        validator_source,
        {"SEVEN_SEGMENTS", "FEED_COLUMNS_7", "C23_B2_LIMIT"},
    )
    if list(constants["SEVEN_SEGMENTS"]) != list(constants["FEED_COLUMNS_7"]):
        raise SystemExit("validator seven-section constants disagree")
    return {
        "schemaVersion": "beef.tv/manju/p4-feed-contract/v1",
        "rulesVersion": "V7.4.26",
        "sources": contract_sources(P4_SOURCES),
        "sections": constants["SEVEN_SEGMENTS"],
        "validator": {
            "script": "short-drama-director/scripts/validate_prompt.py",
            "requiredArgs": ["--by-segment", "--json"],
            "outputMode": "new",
            "failClosed": True,
        },
        "gate": {
            "script": "sd-feed-engine/scripts/gate_check.py",
            "mode": "asset-mode",
            "defaultAssets": 9,
            "failClosed": True,
        },
        "executionOrder": [
            "short-drama-director/scripts/compile_feed.py",
            "short-drama-director/scripts/validate_prompt.py",
            "sd-feed-engine/scripts/gate_check.py",
            "short-drama-director/scripts/audit_contract.py",
        ],
        "shotLine": {
            "requiredElements": ["景别", "焦段", "机位角度", "运镜（用途标签）"],
            "soundField": "声音：",
            "dialogueField": "台词：",
            "emptyDialogue": "无",
            "actionIntentLimit": constants["C23_B2_LIMIT"],
        },
        "styleLock": {
            "injectPosition": "each-unit-text-block",
            "styleMarker": "【STYLE｜视觉风格 · 全片锁死】",
            "performanceMarker": "【PERFORMANCE｜表演与动作 · 全片锁死】",
            "requiredTokens": ["光比", "镜头"],
        },
        "deferredContractIssues": {
            "owner": "P4.5",
            "codes": ["CT5-P45"],
            "reason": "P4.5 是独立附件并由专用阶段和 check_p45.py 校验，不得反向阻断 P4 文本编译。",
        },
        "repairRoutes": {
            "C23a": "beef_tv_adapter",
            "C23b": "P3",
            "C23d2": "beef_tv_adapter",
            "C23f": "P3",
            "CT5-P45": "P4.5",
        },
    }


def render(contract: dict[str, Any]) -> str:
    return json.dumps(contract, ensure_ascii=False, indent=2) + "\n"


def write_or_check(path: Path, rendered: str, check: bool, label: str) -> None:
    if check:
        if not path.is_file():
            raise SystemExit(f"missing generated contract: {path}")
        if path.read_text(encoding="utf-8") != rendered:
            raise SystemExit(f"{label} contract is stale; rerun without --check")
        print(f"{label}=PASS path={path}")
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="\n") as handle:
        handle.write(rendered)
    print(f"{label}=GENERATED path={path}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    write_or_check(P3_OUTPUT, render(build_p3_contract()), args.check, "MANJU_P3_CONTRACT")
    write_or_check(P4_OUTPUT, render(build_p4_contract()), args.check, "MANJU_P4_CONTRACT")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
