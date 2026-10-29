import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App as AntApp, ConfigProvider } from "antd";
import { createRoot } from "react-dom/client";

import { ScriptPipeline } from "@/pages/manju";
import { apiClient } from "@/services/api/request";
import type {
    ManjuProjectDetail,
    ManjuScriptRun,
    ManjuScriptRunInput,
} from "@/services/api/manju";

declare global {
    interface Window {
        __manjuScriptRunSubmissions: ManjuScriptRunInput[];
    }
}

const project = {
    id: "project-1",
    userId: "owner-1",
    name: "热点路由测试",
    type: "manju",
    aspectRatio: "9:16",
    sourceType: "trend",
    description: "",
    stylePresetId: "",
    status: "active",
    revision: 1,
    createdAt: "2026-10-29T00:00:00Z",
    updatedAt: "2026-10-29T00:00:00Z",
};

const detail = {
    project,
    accessRole: "owner",
    lock: {
        id: "lock-1",
        projectId: project.id,
        revision: 1,
        status: "locked",
        snapshot: {
            track: "domestic",
            source: "trend",
            videoModel: "seedance-2.0",
            aspectRatio: "9:16",
            deliveryMode: "markdown",
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
            stylePromptParameters: "高反差硬光",
            styleImagePrompt: "test style",
            assetMode: "M1",
            confirmedLocks: ["track", "source", "videoModel", "aspectRatio", "deliveryMode", "styleCard"],
        },
        segmentCount: 1,
        contentHash: "lock-hash",
        createdBy: "owner-1",
        createdAt: "2026-10-29T00:00:00Z",
    },
    models: {
        id: "models-1",
        projectId: project.id,
        text: { channelId: "text-channel", modelKey: "text-model" },
        image: { channelId: "", modelKey: "" },
        video: { channelId: "", modelKey: "" },
        updatedBy: "owner-1",
        updatedAt: "2026-10-29T00:00:00Z",
    },
} as ManjuProjectDetail;

const createdRun: ManjuScriptRun = {
    id: "run-1",
    projectId: project.id,
    branch: "original",
    mode: "STEPWISE",
    inputKind: "novel",
    trendResolution: "original",
    status: "ACTIVE",
    currentStage: "source_registration",
    lockRevision: 1,
    unitTitle: "热点方向",
    sourceChapterIds: [],
    sourceRangeLabel: "",
    sourceSetHash: "",
    reuseGlobalStages: false,
    createdBy: "owner-1",
    createdAt: "2026-10-29T00:00:00Z",
    updatedAt: "2026-10-29T00:00:00Z",
};

window.__manjuScriptRunSubmissions = [];
apiClient.request = (async (config: { method?: string; url?: string; data?: ManjuScriptRunInput }) => {
    const method = String(config.method || "get").toLowerCase();
    const url = String(config.url || "");
    let data: unknown = {};
    if (method === "get" && url.endsWith("/manju/projects/project-1/script-runs")) {
        data = { items: window.__manjuScriptRunSubmissions.length ? [createdRun] : [] };
    } else if (method === "get" && url.endsWith("/manju/projects/project-1/script-runs/run-1")) {
        data = { item: { run: createdRun, stages: [], artifacts: [] } };
    } else if (method === "post" && url.endsWith("/manju/projects/project-1/script-runs")) {
        const input = config.data as ManjuScriptRunInput;
        window.__manjuScriptRunSubmissions.push(input);
        data = { item: { run: createdRun, stages: [], artifacts: [] } };
    }
    return { data: { code: 0, data, msg: "ok" }, status: 200, headers: {} };
}) as typeof apiClient.request;

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

createRoot(document.getElementById("root")!).render(
    <ConfigProvider>
        <AntApp>
            <QueryClientProvider client={queryClient}>
                <main style={{ boxSizing: "border-box", width: "min(1280px, 100%)", margin: "0 auto", padding: 20 }}>
                    <ScriptPipeline detail={detail} onChanged={() => undefined} />
                </main>
            </QueryClientProvider>
        </AntApp>
    </ConfigProvider>,
);
