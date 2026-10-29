import { describe, expect, test } from "bun:test";

import {
    createManjuScriptRun,
    createManjuAssetRun,
    createManjuSourceDocument,
    getManjuSourceLibraryState,
    getManjuSourceDocument,
    listManjuSourceDocuments,
} from "@/services/api/manju";
import { apiClient } from "@/services/api/request";

type Recorded = { method?: string; url?: string; data?: unknown };

async function stubApi(run: (recorded: Recorded[]) => Promise<void>) {
    const original = apiClient.request;
    const recorded: Recorded[] = [];
    apiClient.request = (async (config: Recorded) => {
        recorded.push({ method: config.method, url: config.url, data: config.data });
        return { data: { code: 0, data: { item: { ok: true }, items: [] }, msg: "ok" }, status: 200, headers: {} };
    }) as typeof apiClient.request;
    try {
        await run(recorded);
    } finally {
        apiClient.request = original;
    }
}

describe("原著库 API", () => {
    test("上传和读取原著使用项目级 source-documents 路径", async () => {
        await stubApi(async (recorded) => {
            await listManjuSourceDocuments("project/1");
            await getManjuSourceDocument("project/1", "doc/1");
            await createManjuSourceDocument("project/1", {
                title: "雨夜旧宅", authorizationScope: "本项目改编", fileName: "雨夜旧宅.txt", content: "第一章 正文",
            });
            expect(recorded[0]).toMatchObject({ method: "get", url: "/manju/projects/project%2F1/source-documents" });
            expect(recorded[1]).toMatchObject({ method: "get", url: "/manju/projects/project%2F1/source-documents/doc%2F1" });
            expect(recorded[2]).toMatchObject({
                method: "post",
                url: "/manju/projects/project%2F1/source-documents",
                data: { title: "雨夜旧宅", fileName: "雨夜旧宅.txt" },
            });
        });
    });

    test("创建第二章时提交章节与基础单元，P0 不在请求体中重复提交", async () => {
        await stubApi(async (recorded) => {
            await createManjuScriptRun("project-1", {
                mode: "STEPWISE",
                title: "第二章改编",
                sourceDocumentId: "doc-1",
                chapterIds: ["chapter-2"],
                baseRunId: "run-1",
                reuseGlobalStages: true,
            });
            expect(recorded[0]).toMatchObject({
                method: "post",
                url: "/manju/projects/project-1/script-runs",
                data: {
                    title: "第二章改编",
                    sourceDocumentId: "doc-1",
                    chapterIds: ["chapter-2"],
                    baseRunId: "run-1",
                    reuseGlobalStages: true,
                },
            });
            expect(recorded[0].data).not.toHaveProperty("lockRevision");
        });
    });

    test("读取原著库哈希并显式提交 P2 资产继承", async () => {
        await stubApi(async (recorded) => {
            await getManjuSourceLibraryState("project-1");
            await createManjuAssetRun("project-1", {
                scriptRunId: "run-2",
                assetMode: "M2",
                baseAssetRunId: "asset-run-1",
                reuseAssets: true,
            });
            expect(recorded[0]).toMatchObject({
                method: "get",
                url: "/manju/projects/project-1/source-library-state",
            });
            expect(recorded[1]).toMatchObject({
                method: "post",
                url: "/manju/projects/project-1/asset-runs",
                data: {
                    scriptRunId: "run-2",
                    assetMode: "M2",
                    baseAssetRunId: "asset-run-1",
                    reuseAssets: true,
                },
            });
        });
    });
});
