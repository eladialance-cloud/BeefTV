#!/usr/bin/env python3
"""Generate the BeefTV P2 asset-block output contract from the pinned V7.4.26 checker."""

from __future__ import annotations

import argparse
import ast
import hashlib
import json
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
SOURCE = (
    ROOT
    / "backend/internal/manju/skilldata/v7.4.26/short-drama-director/scripts/check_asset_blocks.py"
)
OUTPUT = ROOT / "backend/internal/manju/output_contracts/asset-block-v1.json"


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
        raise SystemExit(f"checker constants missing: {', '.join(sorted(missing))}")
    return result


def build_contract(source: str, source_hash: str) -> dict[str, Any]:
    constants = read_literal_assignments(
        source,
        {
            "CROSS_REF_PATTERNS",
            "SCENE_GRID_FIELDS",
            "PROP_FIELDS",
            "PROP_FIELDS_ALT",
            "GROUP_FIELDS",
            "COMPLEX_SCENE_MARKERS",
        },
    )
    cross_refs = [label for _pattern, label in constants["CROSS_REF_PATTERNS"]]
    scene_fields = constants["SCENE_GRID_FIELDS"]
    prop_fields = constants["PROP_FIELDS"]
    return {
        "schemaVersion": "beef.tv/manju/asset-block-contract/v1",
        "rulesVersion": "V7.4.26",
        "source": {
            "path": "short-drama-director/scripts/check_asset_blocks.py",
            "sha256": source_hash,
        },
        "block": {
            "headingTemplate": "#### 图N · 名称（类型）",
            "headingLevels": [2, 6],
            "headingPrefix": "图N",
            "examples": [
                "#### 图1 · 林澈（角色四格）",
                "#### 图2 · 旧宅（场景母版）",
                "#### 图3 · 旧宅（场景九宫格）",
                "#### 图4 · 断刀（道具设定图）",
                "#### 图5 · 城头守军（群像资产卡）",
                "#### 图6 · 城头攻防（战斗站位版）",
            ],
            "aspectRatio": "16:9",
            "selfContained": True,
            "forbiddenCrossReferences": cross_refs,
            "minBlocks": 3,
        },
        "kinds": {
            "character": {
                "label": "角色块",
                "all": [
                    "全图共 4 格",
                    "五官 10 项锁定",
                    "声纹",
                    "≤8 词音色速览标签",
                    "fifth panel",
                    "different face",
                    "身份外观三件套",
                    "Voice Description",
                ],
                "forbidden": ["主图"],
            },
            "sceneMaster": {
                "label": "场景母版块",
                "all": ["16:9 无人空镜"],
            },
            "sceneGrid": {
                "label": "场景九宫格块",
                "all": [
                    *scene_fields,
                    "全图共 9 格",
                    "1~9 标注降级对照表",
                ],
                "any": [
                    "仅允许摄影机移动",
                    "允许按子区域切换机位",
                ],
                "promptFields": [
                    "【参考场景锁定模式】正文必须二选一原文写入：模式A「仅允许摄影机移动」或模式B「允许按子区域切换机位」；不得只写字段名。"
                ],
            },
            "prop": {
                "label": "道具块",
                "all": [
                    "道具设定图",
                    *prop_fields,
                    "禁止手持",
                ],
                "fieldAliases": constants["PROP_FIELDS_ALT"],
            },
            "groupAsset": {
                "label": "群像资产卡",
                "all": [
                    "全图共 3 格",
                    "3 格横向并排",
                    *constants["GROUP_FIELDS"],
                    "B 级群体格内不刻画个体五官",
                ],
            },
            "complexScene": {
                "label": "复杂场景调度块",
                "all": constants["COMPLEX_SCENE_MARKERS"],
            },
        },
    }


def load_source() -> tuple[str, str]:
    raw = SOURCE.read_bytes()
    return raw.decode("utf-8"), hashlib.sha256(raw).hexdigest()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="verify the committed contract is current")
    args = parser.parse_args()

    source, source_hash = load_source()
    contract = build_contract(source, source_hash)
    rendered = json.dumps(contract, ensure_ascii=False, indent=2) + "\n"

    if args.check:
        if not OUTPUT.exists():
            raise SystemExit(f"missing generated contract: {OUTPUT}")
        current = OUTPUT.read_text(encoding="utf-8")
        if current != rendered:
            raise SystemExit("asset-block contract is stale; rerun without --check")
        print(
            "MANJU_ASSET_BLOCK_CONTRACT_CHECK=PASS "
            f"sha256={source_hash} output={OUTPUT}"
        )
        return 0

    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    with OUTPUT.open("w", encoding="utf-8", newline="\n") as handle:
        handle.write(rendered)
    print(
        "MANJU_ASSET_BLOCK_CONTRACT_GENERATED=PASS "
        f"sha256={source_hash} output={OUTPUT}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
