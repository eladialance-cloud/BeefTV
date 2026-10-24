import { WEDDING_STAGE_IDS, type WeddingItemStatus, type WeddingProductionState, type WeddingStageId } from "@/lib/wedding/types";

export const weddingStageLabels: Record<WeddingStageId, string> = {
    setup: "项目设定",
    dynasties: "朝代规划",
    portraits: "定妆照",
    scenes: "场景首帧",
    videos: "视频导演",
    packaging: "包装制作",
    delivery: "成片交付",
};

export const weddingStatusLabels: Record<WeddingItemStatus, string> = {
    not_started: "未开始",
    ready: "可执行",
    generating: "生成中",
    needs_review: "待审",
    approved: "已确认",
    failed: "失败",
    stale: "已失效",
};

export function WorkbenchStageRail({
    state,
    activeStage,
    onSelect,
}: {
    state: WeddingProductionState;
    activeStage: WeddingStageId;
    onSelect: (stage: WeddingStageId) => void;
}) {
    return (
        <nav className="wedding-workbench-steps" aria-label="婚礼制作阶段">
            {WEDDING_STAGE_IDS.map((stage, index) => {
                const stageState = state.stages[stage];
                return (
                    <button
                        key={stage}
                        type="button"
                        aria-current={activeStage === stage ? "step" : undefined}
                        className={`wedding-workbench-step ${activeStage === stage ? "is-active" : ""}`}
                        onClick={() => onSelect(stage)}
                    >
                        <span>{String(index + 1).padStart(2, "0")}</span>
                        <div>
                            <strong>{weddingStageLabels[stage]}</strong>
                            <small>{weddingStatusLabels[stageState.status]}</small>
                        </div>
                    </button>
                );
            })}
        </nav>
    );
}
