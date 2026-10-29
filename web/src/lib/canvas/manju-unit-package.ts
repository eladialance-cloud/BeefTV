import type { ManjuScriptRunDetail } from "@/services/api/manju";

export type ManjuUnitPackage = {
    fileName: string;
    markdown: string;
};

function safeFileName(value: string) {
    return (value.trim() || "漫剧改编单元").replace(/[<>:"/\\|?*\u0000-\u001f]/g, "_").slice(0, 80);
}

export function buildManjuUnitPackage(detail: ManjuScriptRunDetail): ManjuUnitPackage | null {
    if (!detail.artifacts?.length) return null;
    const stageByID = new Map(detail.stages.map((stage) => [stage.id, stage]));
    const hasScriptArtifact = detail.artifacts.some((artifact) => {
        const stage = stageByID.get(artifact.stageId);
        return stage?.key === "script_draft" || stage?.key === "polish_draft";
    });
    if (!hasScriptArtifact) return null;
    const artifacts = [...detail.artifacts].sort((left, right) => {
        const leftPosition = stageByID.get(left.stageId)?.position ?? Number.MAX_SAFE_INTEGER;
        const rightPosition = stageByID.get(right.stageId)?.position ?? Number.MAX_SAFE_INTEGER;
        return leftPosition - rightPosition || left.version - right.version;
    });
    const title = detail.run.unitTitle || "漫剧改编单元";
    const lines = [
        `# ${title}`,
        "",
        `- 项目：${detail.run.projectId}`,
        `- 原著范围：${detail.run.sourceRangeLabel || detail.source?.rangeLabel || "未登记"}`,
        `- 来源名称：${detail.source?.title || "未登记"}`,
        `- 来源哈希：${detail.source?.contentHash || "未登记"}`,
        `- P0 revision：${detail.run.lockRevision}`,
        `- 制作方式：${detail.run.mode === "STEPWISE" ? "按阶段确认" : "完整流程"}`,
        `- 单元状态：${detail.run.status === "COMPLETED" ? "已完成" : "进行中"}`,
        "",
    ];
    for (const artifact of artifacts) {
        const stage = stageByID.get(artifact.stageId);
        lines.push(
            "---",
            "",
            `## ${stage?.title || artifact.kind}`,
            "",
            `> 工件：${artifact.kind} · v${artifact.version} · ${artifact.contentHash.slice(0, 12)}`,
            "",
            artifact.content.trim(),
            "",
        );
    }
    return { fileName: `${safeFileName(title)}.md`, markdown: lines.join("\n") };
}
