import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App as AntApp, ConfigProvider } from "antd";
import { createRoot } from "react-dom/client";

import { SourceLibraryStepsPanel } from "@/pages/manju/source-library-steps-panel";
import { apiClient } from "@/services/api/request";

const sourceDocument = {
    id: "doc-1",
    projectId: "project-1",
    title: "雨夜旧宅",
    authorizationScope: "仅用于本项目改编",
    fileName: "雨夜旧宅.txt",
    contentHash: "abcdef1234567890abcdef",
    characterCount: 24500,
    chapterCount: 2,
    createdBy: "owner",
    createdAt: "2026-10-21T00:00:00Z",
    updatedAt: "2026-10-21T00:00:00Z",
};

const chapters = [
    {
        id: "chapter-1", projectId: "project-1", documentId: "doc-1", position: 1,
        title: "第一章 雨夜", rangeLabel: "第一章 雨夜", contentHash: "chapter-a",
        characterCount: 12000, createdAt: "2026-10-21T00:00:00Z",
    },
    {
        id: "chapter-2", projectId: "project-1", documentId: "doc-1", position: 2,
        title: "第二章 旧照片", rangeLabel: "第二章 旧照片", contentHash: "chapter-b",
        characterCount: 12500, createdAt: "2026-10-21T00:00:00Z",
    },
];

const baseRun = {
    id: "run-1", projectId: "project-1", branch: "adaptation", mode: "STEPWISE",
    status: "COMPLETED", currentStage: "", lockRevision: 1, unitTitle: "第一章改编",
    sourceChapterIds: ["chapter-1"], sourceRangeLabel: "第一章 雨夜",
    sourceSetHash: "old-library-hash", reuseGlobalStages: false,
    createdBy: "owner", createdAt: "2026-10-21T00:00:00Z", updatedAt: "2026-10-21T00:00:00Z",
};

apiClient.request = (async (config: { method?: string; url?: string }) => {
    const method = String(config.method || "get").toLowerCase();
    const url = String(config.url || "");
    let data: unknown = {};
    if (method === "get" && url.endsWith("/source-documents")) data = { items: [sourceDocument] };
    else if (method === "get" && url.endsWith("/source-documents/doc-1")) data = { item: { document: sourceDocument, chapters } };
    else if (method === "get" && url.endsWith("/source-library-state")) data = { item: { contentHash: "new-library-hash", documentCount: 1, chapterCount: 2 } };
    else if (method === "post" && url.endsWith("/source-documents")) data = { item: { document: sourceDocument, chapters } };
    else if (method === "post" && url.endsWith("/script-runs")) data = { item: { run: { id: "run-1", unitTitle: "第一章改编" }, stages: [], artifacts: [] } };
    return { data: { code: 0, data, msg: "ok" }, status: 200, headers: {} };
}) as typeof apiClient.request;

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
const stale = new URLSearchParams(window.location.search).has("stale");

createRoot(document.getElementById("root")!).render(
    <ConfigProvider>
        <AntApp>
            <QueryClientProvider client={queryClient}>
                <main style={{ boxSizing: "border-box", width: "min(1440px, 100%)", margin: "0 auto", padding: 20 }}>
                    <SourceLibraryStepsPanel
                        projectId="project-1"
                        canEdit
                        inputKind="novel"
                        completedRuns={stale ? [baseRun] : []}
                        hasActiveRun={false}
                        onCreated={() => undefined}
                    />
                </main>
            </QueryClientProvider>
        </AntApp>
    </ConfigProvider>,
);
