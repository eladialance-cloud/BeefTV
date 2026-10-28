import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App as AntApp, ConfigProvider } from "antd";
import { createRoot } from "react-dom/client";

import { P0LockPanel } from "@/pages/manju";
import { apiClient } from "@/services/api/request";
import type { ManjuLockInput, ManjuLockReceipt, ManjuProjectDetail, ManjuStyleCatalog } from "@/services/api/manju";

declare global {
    interface Window {
        __manjuLockSubmissions: ManjuLockInput[];
    }
}

const catalog: ManjuStyleCatalog = {
    version: "V7.4.26",
    bases: [
        { id: "live-action", name: "真人实拍质感" },
        { id: "2d-animation", name: "2D 手绘动画" },
        { id: "3d-cg", name: "3D CG 动画" },
        { id: "ink-2d", name: "水墨二维" },
    ],
    families: [
        { id: "T1", name: "刃光派", feature: "高反差硬光与凌厉动作" },
    ],
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
        promptParameters: "高反差硬光、广角低机位、体积雾、凌厉升格",
        avoidanceWarning: "塑料感三渲二、游戏CG均匀打光",
        compatibleStyleBases: ["live-action"],
    }],
};

const project = {
    id: "project-1",
    userId: "owner-1",
    name: "P0 explicit lock fixture",
    type: "manju",
    aspectRatio: "9:16",
    sourceType: "adaptation",
    description: "",
    stylePresetId: "",
    status: "active",
    revision: 1,
    createdAt: "2026-10-28T00:00:00Z",
    updatedAt: "2026-10-28T00:00:00Z",
};

const legacyLock = {
    id: "legacy-lock",
    projectId: project.id,
    revision: 1,
    status: "locked",
    snapshot: {
        track: "domestic",
        source: "adaptation",
        videoModel: "seedance-2.0",
        aspectRatio: "9:16",
        deliveryMode: "markdown",
        seamMode: "",
        targetDurationSeconds: 15,
        segmentCount: 1,
        singleSegmentMaxSeconds: 15,
        styleBase: "live-action",
        styleCardId: "sd-style-009",
        styleCardName: "徐克",
        styleFamily: "T1",
        styleFightRating: 3,
        styleEffectRating: 3,
        styleGrade: "B",
        styleCatalogVersion: "V7.4.26",
        stylePromptParameters: "高反差硬光、广角低机位、体积雾、凌厉升格",
        styleImagePrompt: "legacy style prompt",
        assetMode: "M1",
    },
    segmentCount: 1,
    contentHash: "legacy-hash",
    createdBy: "owner-1",
    createdAt: "2026-10-28T00:00:00Z",
} satisfies ManjuLockReceipt;

function receiptFor(input: ManjuLockInput): ManjuLockReceipt {
    const card = catalog.items.find((item) => item.id === input.styleCardId);
    const segmentCount = Math.max(1, Math.ceil(input.targetDurationSeconds / (input.videoModel === "seedance-2.5" ? 30 : 15)));
    return {
        id: "saved-lock",
        projectId: project.id,
        revision: 2,
        status: "locked",
        snapshot: {
            ...input,
            seamMode: input.seamMode || "",
            segmentCount,
            singleSegmentMaxSeconds: input.videoModel === "seedance-2.5" ? 30 : 15,
            styleCardName: card?.name || "自定义画风",
            styleFamily: card?.family || "CUSTOM",
            styleFightRating: card?.fightRating || 0,
            styleEffectRating: card?.effectRating || 0,
            styleGrade: card?.grade || "C",
            styleCatalogVersion: catalog.version,
            stylePromptParameters: card?.promptParameters || input.styleVisualPrompt || "",
            styleImagePrompt: "saved style prompt",
        },
        segmentCount,
        contentHash: "saved-hash",
        createdBy: "owner-1",
        createdAt: "2026-10-28T00:00:00Z",
    } as ManjuLockReceipt;
}

window.__manjuLockSubmissions = [];
apiClient.request = (async (config: { method?: string; url?: string; data?: ManjuLockInput }) => {
    const method = String(config.method || "get").toLowerCase();
    const url = String(config.url || "");
    let data: unknown = {};
    if (method === "get" && url.endsWith("/manju/style-catalog")) {
        data = catalog;
    } else if (method === "put" && url.endsWith("/manju/projects/project-1/lock")) {
        const input = config.data as ManjuLockInput;
        window.__manjuLockSubmissions.push(input);
        data = { item: receiptFor(input) };
    }
    return { data: { code: 0, data, msg: "ok" }, status: 200, headers: {} };
}) as typeof apiClient.request;

const detail: ManjuProjectDetail = {
    project,
    accessRole: "owner",
    lock: new URLSearchParams(window.location.search).has("legacy") ? legacyLock : undefined,
};
const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

createRoot(document.getElementById("root")!).render(
    <ConfigProvider>
        <AntApp>
            <QueryClientProvider client={queryClient}>
                <main style={{ boxSizing: "border-box", width: "min(1440px, 100%)", margin: "0 auto", padding: 20 }}>
                    <P0LockPanel detail={detail} onSaved={() => undefined} />
                </main>
            </QueryClientProvider>
        </AntApp>
    </ConfigProvider>,
);
