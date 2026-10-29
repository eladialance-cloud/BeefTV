import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App as AntApp, ConfigProvider } from "antd";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes } from "react-router";

import ManjuStudioPage, { P0LockPanel } from "@/pages/manju";
import { apiClient } from "@/services/api/request";
import type {
    ManjuAssetRun,
    ManjuLockReceipt,
    ManjuProductionArtifact,
    ManjuProductionRun,
    ManjuProductionRunDetail,
    ManjuProductionStage,
    ManjuProjectDetail,
    ManjuScriptRun,
    ManjuStyleCatalog,
    ManjuUserDeliverable,
} from "@/services/api/manju";

type GateMode =
    | "missing-p0"
    | "missing-p0b"
    | "partial-storyboard"
    | "missing-p4-proof"
    | "board-delivery"
    | "markdown-delivery";

type RecordedRequest = {
    method: string;
    url: string;
    data?: unknown;
};

declare global {
    interface Window {
        __manjuGateRequests: RecordedRequest[];
    }
}

const mode = (new URLSearchParams(window.location.search).get("mode") || "missing-p0") as GateMode;
const projectID = "project-v7426-gate";
const runID = "production-run-v7426-gate";
const now = "2026-10-29T00:00:00Z";

const project = {
    id: projectID,
    userId: "owner-1",
    name: "V7.4.26 门控回归",
    type: "manju",
    aspectRatio: "9:16",
    sourceType: "adaptation",
    description: "",
    stylePresetId: "",
    status: "active",
    revision: 1,
    createdAt: now,
    updatedAt: now,
};

const lock: ManjuLockReceipt = {
    id: "lock-v7426-gate",
    projectId: projectID,
    revision: 1,
    status: "locked",
    snapshot: {
        track: "domestic",
        source: "adaptation",
        videoModel: "seedance-2.0",
        aspectRatio: "9:16",
        deliveryMode: mode === "board-delivery" ? "board" : "markdown",
        seamMode: "",
        targetDurationSeconds: 15,
        segmentCount: 2,
        singleSegmentMaxSeconds: 15,
        styleBase: "live-action",
        styleCardId: "sd-style-009",
        styleCardName: "徐克",
        styleFamily: "T1",
        styleFightRating: 3,
        styleEffectRating: 3,
        styleGrade: "B",
        styleCatalogVersion: "V7.4.26",
        stylePromptParameters: "高反差硬光、广角低机位",
        styleImagePrompt: "写实电影感",
        assetMode: "M1",
        confirmedLocks: ["track", "source", "videoModel", "aspectRatio", "deliveryMode", "styleCard"],
    },
    segmentCount: 2,
    contentHash: "lock-content-hash",
    createdBy: "owner-1",
    createdAt: now,
};

const styleCatalog: ManjuStyleCatalog = {
    version: "V7.4.26",
    bases: [
        { id: "live-action", name: "真人实拍质感" },
        { id: "2d-animation", name: "2D 手绘动画" },
        { id: "3d-cg", name: "3D CG 动画" },
        { id: "ink-2d", name: "水墨二维" },
    ],
    families: [{ id: "T1", name: "刃光派", feature: "高反差硬光与凌厉动作" }],
    items: [{
        id: "sd-style-009",
        number: 9,
        name: "徐克",
        group: "武侠",
        family: "T1",
        fightRating: 3,
        effectRating: 3,
        grade: "B",
        nativeGenre: "武侠古装",
        promptParameters: "高反差硬光、广角低机位",
        avoidanceWarning: "塑料感三渲二",
        compatibleStyleBases: ["live-action"],
    }],
};

const scriptRun: ManjuScriptRun = {
    id: "script-run-v7426-gate",
    projectId: projectID,
    branch: "adaptation",
    mode: "STEPWISE",
    inputKind: "novel",
    trendResolution: "",
    status: "COMPLETED",
    currentStage: "",
    lockRevision: lock.revision,
    unitTitle: "第一章",
    sourceChapterIds: [],
    sourceRangeLabel: "第一章",
    sourceSetHash: "source-hash",
    reuseGlobalStages: false,
    createdBy: "owner-1",
    createdAt: now,
    updatedAt: now,
};

const assetRun: ManjuAssetRun = {
    id: "asset-run-v7426-gate",
    projectId: projectID,
    scriptRunId: scriptRun.id,
    reuseAssets: false,
    inheritedAssetCount: 0,
    assetMode: "M1",
    status: "COMPLETED",
    currentStage: "",
    lockRevision: lock.revision,
    createdBy: "owner-1",
    createdAt: now,
    updatedAt: now,
};

function productionStage(
    key: string,
    title: string,
    position: number,
    status: ManjuProductionStage["status"],
    artifactId = "",
): ManjuProductionStage {
    return {
        id: `stage-${key}`,
        key,
        title,
        position,
        status,
        version: 1,
        artifactId,
        confirmedBy: status === "LOCKED" ? "owner-1" : undefined,
        confirmedAt: status === "LOCKED" ? now : undefined,
        updatedAt: now,
    };
}

function artifact(id: string, stageId: string, kind: string, content: string): ManjuProductionArtifact {
    return {
        id,
        stageId,
        kind,
        version: 1,
        content,
        contentHash: `${id}-hash`,
        createdAt: now,
    };
}

function productionFixture(): {
    run: ManjuProductionRun;
    detail: ManjuProductionRunDetail;
} {
    const active = mode !== "board-delivery" && mode !== "markdown-delivery";
    const p0bLocked = mode !== "missing-p0b";
    const p3Awaiting = mode === "partial-storyboard";
    const p4Awaiting = mode === "missing-p4-proof";
    const completed = mode === "board-delivery" || mode === "markdown-delivery";
    const p3Locked = p4Awaiting || completed;
    const p4Locked = completed;
    const p5Locked = completed;
    const currentStage = mode === "missing-p0b"
        ? "decision_anchor"
        : mode === "partial-storyboard"
          ? "director_storyboard"
          : mode === "missing-p4-proof"
            ? "feed_package"
            : "";
    const run: ManjuProductionRun = {
        id: runID,
        projectId: projectID,
        assetRunId: assetRun.id,
        status: completed ? "COMPLETED" : "ACTIVE",
        currentStage,
        lockRevision: lock.revision,
        createdBy: "owner-1",
        createdAt: now,
        updatedAt: now,
    };
    const stages = [
        productionStage(
            "decision_anchor",
            "P0B 决策锚点",
            1,
            p0bLocked ? "LOCKED" : "IN_PROGRESS",
            p0bLocked ? "artifact-p0b" : "",
        ),
        productionStage(
            "director_storyboard",
            "P3 分镜与故事板",
            2,
            p3Awaiting ? "AWAITING_CONFIRMATION" : p3Locked ? "LOCKED" : "NOT_STARTED",
            p3Locked ? "artifact-p3" : p3Awaiting ? "artifact-p3" : "",
        ),
        productionStage(
            "feed_package",
            "P4 投喂封装",
            3,
            p4Awaiting ? "AWAITING_CONFIRMATION" : p4Locked ? "LOCKED" : "NOT_STARTED",
            p4Awaiting || p4Locked ? "artifact-p4" : "",
        ),
        productionStage(
            "qc_handoff",
            "P5 质检与交付",
            4,
            p5Locked ? "LOCKED" : "NOT_STARTED",
            p5Locked ? "artifact-p5" : "",
        ),
    ];
    const artifacts = [
        artifact("artifact-p0b", "stage-decision_anchor", "EMOTION_ANCHOR_TABLE.md", "# P0B\n\n情绪锚点已锁定。"),
        artifact("artifact-p3", "stage-director_storyboard", "STORYBOARD.md", "# P3\n\n分镜与故事板已锁定。"),
        artifact("artifact-p4", "stage-feed_package", "FEED_PACKAGE.md", "# P4\n\n官方投喂稿等待确认。"),
        artifact("artifact-p5", "stage-qc_handoff", "QC_HANDOFF.md", "# P5\n\n主理人看片验收。"),
        artifact("artifact-previs", "stage-feed_package", "PREVIS_STORYBOARD.md", "# 预演故事板\n\nP4.5 已通过 check_p45.py。"),
    ];
    const detail: ManjuProductionRunDetail = {
        run,
        assetRun,
        scriptRun,
        stages,
        artifacts,
        previs: p4Awaiting || completed ? {
            id: "previs-v7426-gate",
            runId: runID,
            artifactId: "artifact-previs",
            status: "READY",
            version: 1,
            createdAt: now,
            updatedAt: now,
        } : undefined,
        images: [
            {
                id: "storyboard-segment-1",
                runId: runID,
                artifactId: "artifact-p3",
                position: 1,
                segmentCode: "段1",
                attempt: 1,
                status: "READY",
                resourceId: "resource-storyboard-1",
                createdAt: now,
                updatedAt: now,
            },
            ...(mode === "partial-storyboard" ? [{
                id: "storyboard-segment-2",
                runId: runID,
                artifactId: "artifact-p3",
                position: 2,
                segmentCode: "段2",
                attempt: 1,
                status: "FAILED" as const,
                error: "故事板图生成失败",
                createdAt: now,
                updatedAt: now,
            }] : [{
                id: "storyboard-segment-2",
                runId: runID,
                artifactId: "artifact-p3",
                position: 2,
                segmentCode: "段2",
                attempt: 1,
                status: "READY" as const,
                resourceId: "resource-storyboard-2",
                createdAt: now,
                updatedAt: now,
            }]),
        ],
        videos: completed ? [{
            position: 1,
            segmentCode: "段1",
            durationSeconds: 15,
            attempt: 1,
            status: "READY",
            resourceId: "resource-video-1",
            createdAt: now,
            updatedAt: now,
        }] : [],
    };
    if (!active) {
        detail.stages = stages;
    }
    return { run, detail };
}

const production = productionFixture();

const detail: ManjuProjectDetail = {
    project,
    accessRole: "owner",
    lock: mode === "missing-p0" ? undefined : lock,
    models: {
        id: "models-v7426-gate",
        projectId: projectID,
        text: { channelId: "text-channel", modelKey: "text-model" },
        image: { channelId: "image-channel", modelKey: "image-model" },
        video: { channelId: "video-channel", modelKey: "video-model" },
        updatedBy: "owner-1",
        updatedAt: now,
    },
};

function deliverable(
    kind: ManjuUserDeliverable["kind"],
    fileName: string,
    mimeType: string,
): ManjuUserDeliverable {
    return {
        id: `deliverable-${kind}`,
        projectId: projectID,
        lockRevision: lock.revision,
        productionRunId: runID,
        kind,
        fileName,
        mimeType,
        content: kind === "board" ? "<!doctype html><title>V7.4.26 board</title>" : `# ${kind}`,
        contentHash: `${kind}-sha256`.padEnd(64, "0"),
        contentContractVersion: "V7.4.26-DELIVERY-V2",
        complianceStatus: "COMPLIANT",
        createdAt: now,
    };
}

const deliverables = (() => {
    const rows = [
        deliverable("markdown", "V7.4.26-源稿.md", "text/markdown; charset=utf-8"),
        ...(mode === "board-delivery"
            ? [deliverable("board", "V7.4.26-看板.html", "text/html; charset=utf-8")]
            : []),
        deliverable("previs", "V7.4.26-预演故事板.md", "text/markdown; charset=utf-8"),
        deliverable("manifest", "V7.4.26-交付清单.json", "application/json"),
    ];
    return rows;
})();

function ok(data: unknown, status = 200) {
    return { data: { code: 0, data, msg: "ok" }, status, headers: {} };
}

function rejected(message: string) {
    return { data: { code: 40901, data: null, msg: message }, status: 409, headers: {} };
}

window.__manjuGateRequests = [];
apiClient.request = (async (config: { method?: string; url?: string; data?: unknown }) => {
    const method = String(config.method || "get").toLowerCase();
    const url = String(config.url || "");
    const path = new URL(url, "http://manju.test").pathname;
    window.__manjuGateRequests.push({ method, url, data: config.data });

    if (method === "get" && path.endsWith(`/manju/projects/${projectID}`)) {
        return ok({ item: detail });
    }
    if (method === "get" && path.endsWith("/manju/style-catalog")) {
        return ok(styleCatalog);
    }
    if (method === "get" && path.endsWith(`/manju/projects/${projectID}/rule-library`)) {
        return ok({
            item: {
                version: "V7.4.26",
                embedded: true,
                fileCount: 170,
                contentHash: "v7426-rule-hash",
                files: [],
                officialScripts: [],
            },
        });
    }
    if (method === "get" && path.endsWith(`/manju/projects/${projectID}/script-runs`)) {
        return ok({ items: [scriptRun] });
    }
    if (method === "get" && path.endsWith(`/manju/projects/${projectID}/asset-runs`)) {
        return ok({ items: [assetRun] });
    }
    if (method === "get" && path.endsWith(`/manju/projects/${projectID}/production-runs`)) {
        return ok({ items: [production.run] });
    }
    if (method === "get" && path.endsWith(`/manju/projects/${projectID}/production-runs/${runID}`)) {
        return ok({ item: production.detail });
    }
    if (method === "get" && path.endsWith(`/manju/projects/${projectID}/production-runs/${runID}/deliverables`)) {
        return ok({ items: deliverables });
    }
    if (method === "get" && path.endsWith(`/manju/projects/${projectID}/canvas-imports`)) {
        return ok({ items: [] });
    }
    if (method === "post" && path.endsWith(`/production-runs/${runID}/stages/director_storyboard/confirm`)) {
        if (mode === "partial-storyboard") {
            return rejected("P3 故事板图片未全部就绪，不能锁定进入 P4。");
        }
    }
    if (method === "post" && path.endsWith(`/production-runs/${runID}/stages/feed_package/confirm`)) {
        if (mode === "missing-p4-proof") {
            return rejected("P4 机检凭证缺失，不能锁定进入 P5。");
        }
    }
    if (method === "put" && path.endsWith(`/manju/projects/${projectID}/lock`)) {
        return ok({ item: lock });
    }
    if (method === "get") {
        return ok({ items: [] });
    }
    return ok({});
}) as typeof apiClient.request;

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
const root = createRoot(document.getElementById("root")!);
const shell = (children: React.ReactNode) => (
    <ConfigProvider>
        <AntApp>
            <QueryClientProvider client={queryClient}>
                {children}
            </QueryClientProvider>
        </AntApp>
    </ConfigProvider>
);

if (mode === "missing-p0") {
    root.render(shell(
        <main style={{ boxSizing: "border-box", width: "min(1280px, 100%)", margin: "0 auto", padding: 20 }}>
            <P0LockPanel detail={detail} onSaved={() => undefined} />
        </main>,
    ));
} else {
    const stage = mode === "board-delivery" || mode === "markdown-delivery"
        ? "p5"
        : mode === "missing-p4-proof"
          ? "p4"
          : "p3";
    root.render(shell(
        <MemoryRouter initialEntries={[`/manju/${projectID}?tab=stages&stage=${stage}`]}>
            <Routes>
                <Route path="/manju/:projectId" element={<ManjuStudioPage />} />
            </Routes>
        </MemoryRouter>,
    ));
}
