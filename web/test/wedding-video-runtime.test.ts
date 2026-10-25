import { expect, test } from "bun:test";

import { createWeddingProductionState, weddingStageBlockers } from "@/lib/wedding/production-state";
import {
    confirmWeddingVideo,
    failWeddingVideoGeneration,
    markWeddingVideoGenerating,
    recordWeddingVideoVersion,
    selectWeddingVideoVersion,
    weddingVideoJobs,
} from "@/lib/wedding/video-runtime";

const production = createWeddingProductionState({
    template: { id: "template-1", key: "cross-era-wedding-volc", version: "1.0.0" },
    source: {
        groomPhotoNodeId: "groom-photo",
        bridePhotoNodeId: "bride-photo",
        maleName: "林屿",
        femaleName: "周棠",
        story: "杭州西湖相识",
        consentConfirmed: true,
        ratio: "16:9",
        resolution: "720p",
    },
    dynasties: [
        {
            id: "tang",
            name: "唐",
            order: 0,
            portraitMaleNodeId: "male-tang",
            portraitFemaleNodeId: "female-tang",
            sceneNodeId: "scene-tang",
            videoNodeId: "video-tang",
        },
        {
            id: "ming",
            name: "明",
            order: 1,
            portraitMaleNodeId: "male-ming",
            portraitFemaleNodeId: "female-ming",
            sceneNodeId: "scene-ming",
            videoNodeId: "video-ming",
        },
    ],
    now: "2026-10-25T00:00:00.000Z",
});
const passingQc = {
    verdict: "pass" as const,
    faceStable: true,
    outfitStable: true,
    actionReadable: true,
    extraPerson: false,
    watermarkOrText: false,
    reason: "人物、服饰和动作连续。",
    checkedAt: "2026-10-25T06:00:00.000Z",
};

test("maps one video job per dynasty", () => {
    expect(weddingVideoJobs(production)).toEqual([
        { nodeId: "video-tang", dynastyId: "tang", kind: "video" },
        { nodeId: "video-ming", dynastyId: "ming", kind: "video" },
    ]);
});

test("record and version selection clear old video QC and isolate packaging", () => {
    const generating = markWeddingVideoGenerating(production, weddingVideoJobs(production, "tang")[0]!, "2026-10-25T01:00:00.000Z");
    expect(generating.dynasties[0]?.statuses.video).toBe("generating");
    expect(generating.dynasties[1]).toEqual(production.dynasties[1]);

    const withOldQc = {
        ...generating,
        packaging: { ...generating.packaging, status: "approved" as const },
        dynasties: generating.dynasties.map((dynasty) => dynasty.id === "tang"
            ? { ...dynasty, qc: { ...dynasty.qc, video: passingQc } }
            : dynasty),
    };
    const generated = recordWeddingVideoVersion(withOldQc, weddingVideoJobs(production, "tang")[0]!, {
        assetId: "video-asset-v2",
        storageKey: "resource:video-v2",
        previewUrl: "https://example.com/video-v2.mp4",
        posterUrl: "https://example.com/video-v2.jpg",
        taskId: "task-video-v2",
        durationMs: 8000,
        createdAt: "2026-10-25T02:00:00.000Z",
    });
    expect(generated.dynasties[0]?.videoSelection).toBe("video-asset-v2");
    expect(generated.dynasties[0]?.videoVersions).toHaveLength(1);
    expect(generated.dynasties[0]?.qc.video?.verdict).toBe("pending");
    expect(generated.dynasties[0]?.statuses.video).toBe("needs_review");
    expect(generated.packaging.status).toBe("stale");
    expect(generated.stages.packaging.status).toBe("stale");

    const switched = selectWeddingVideoVersion(
        generated,
        "tang",
        "video-asset-v1",
        "2026-10-25T02:10:00.000Z",
    );
    expect(switched.dynasties[0]?.videoSelection).toBe("video-asset-v1");
    expect(switched.dynasties[0]?.qc.video?.verdict).toBe("pending");
});

test("video approval requires a selected version and QC evidence", () => {
    const generated = recordWeddingVideoVersion(production, weddingVideoJobs(production, "tang")[0]!, {
        assetId: "video-asset-v1",
        previewUrl: "https://example.com/video-v1.mp4",
        createdAt: "2026-10-25T02:00:00.000Z",
    });
    const confirmed = confirmWeddingVideo(generated, "tang", "video-asset-v1", "2026-10-25T02:20:00.000Z");
    expect(confirmed.dynasties[0]?.statuses.video).toBe("needs_review");
    expect(weddingStageBlockers(confirmed, "videos")).toContain("唐：视频 QC 未通过且没有覆盖记录");

    const qcPassed = {
        ...confirmed,
        dynasties: confirmed.dynasties.map((dynasty) => dynasty.id === "tang"
            ? { ...dynasty, statuses: { ...dynasty.statuses, video: "ready" as const }, qc: { ...dynasty.qc, video: passingQc } }
            : dynasty),
    };
    expect(weddingStageBlockers(qcPassed, "videos")).toContain("明：视频尚未确认");
    expect(weddingStageBlockers(qcPassed, "videos")).toContain("明：视频 QC 未通过且没有覆盖记录");
});

test("failed video marks only its dynasty and final delivery", () => {
    const failed = failWeddingVideoGeneration(
        production,
        weddingVideoJobs(production, "tang")[0]!,
        "参考场景不可用",
        "2026-10-25T03:00:00.000Z",
    );
    expect(failed.dynasties[0]?.statuses.video).toBe("failed");
    expect(failed.dynasties[1]?.statuses.video).toBe("ready");
    expect(failed.stages.delivery.status).toBe("stale");
    expect(failed.activity.at(-1)).toMatchObject({
        type: "error",
        stage: "videos",
        dynastyId: "tang",
    });
});
