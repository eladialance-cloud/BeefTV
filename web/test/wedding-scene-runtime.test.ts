import { expect, test } from "bun:test";

import {
    confirmWeddingScene,
    failWeddingSceneGeneration,
    markWeddingSceneGenerating,
    recordWeddingSceneVersion,
    selectWeddingSceneVersion,
    weddingSceneJobs,
} from "@/lib/wedding/scene-runtime";
import { createWeddingProductionState, weddingStageBlockers } from "@/lib/wedding/production-state";

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
    couplePresent: true,
    faceMatchMale: true,
    faceMatchFemale: true,
    outfitMatch: true,
    sceneMatch: true,
    extraPerson: false,
    watermarkOrText: false,
    reason: "男女身份、服饰与朝代场景一致。",
    suggestion: "",
    checkedAt: "2026-10-25T04:00:00.000Z",
};

test("maps one scene job per dynasty", () => {
    expect(weddingSceneJobs(production)).toEqual([
        { nodeId: "scene-tang", dynastyId: "tang", kind: "scene" },
        { nodeId: "scene-ming", dynastyId: "ming", kind: "scene" },
    ]);
});

test("recording and switching scene versions clears the old QC and isolates dynasties", () => {
    const generating = markWeddingSceneGenerating(production, weddingSceneJobs(production, "tang")[0]!, "2026-10-25T01:00:00.000Z");
    expect(generating.dynasties[0]?.statuses.scene).toBe("generating");
    expect(generating.dynasties[1]).toEqual(production.dynasties[1]);

    const withOldQc = {
        ...generating,
        dynasties: generating.dynasties.map((dynasty) => dynasty.id === "tang"
            ? { ...dynasty, qc: { ...dynasty.qc, scene: passingQc } }
            : dynasty),
    };
    const generated = recordWeddingSceneVersion(withOldQc, weddingSceneJobs(production, "tang")[0]!, {
        assetId: "scene-asset-v2",
        storageKey: "resource:scene-v2",
        previewUrl: "https://example.com/scene-v2.png",
        taskId: "task-scene-v2",
        createdAt: "2026-10-25T02:00:00.000Z",
    });

    expect(generated.dynasties[0]?.sceneSelection).toBe("scene-asset-v2");
    expect(generated.dynasties[0]?.sceneVersions).toHaveLength(1);
    expect(generated.dynasties[0]?.qc.scene).toBeUndefined();
    expect(generated.dynasties[0]?.statuses.scene).toBe("needs_review");
    expect(generated.dynasties[1]).toEqual(withOldQc.dynasties[1]);

    const switched = selectWeddingSceneVersion(
        generated,
        "tang",
        "scene-asset-v1",
        "2026-10-25T02:10:00.000Z",
    );
    expect(switched.dynasties[0]?.sceneSelection).toBe("scene-asset-v1");
    expect(switched.dynasties[0]?.qc.scene).toBeUndefined();
});

test("scene approval requires both a selected version and passing QC", () => {
    const generated = recordWeddingSceneVersion(production, weddingSceneJobs(production, "tang")[0]!, {
        assetId: "scene-asset-v1",
        previewUrl: "https://example.com/scene-v1.png",
        createdAt: "2026-10-25T02:00:00.000Z",
    });
    expect(weddingStageBlockers(generated, "scenes")).toContain("唐：场景 QC 未通过且没有覆盖记录");

    const confirmed = confirmWeddingScene(generated, "tang", "scene-asset-v1", "2026-10-25T02:20:00.000Z");
    expect(confirmed.dynasties[0]?.sceneSelection).toBe("scene-asset-v1");
    expect(confirmed.dynasties[0]?.statuses.scene).toBe("needs_review");

    const qcPassed = {
        ...confirmed,
        dynasties: confirmed.dynasties.map((dynasty) => dynasty.id === "tang"
            ? { ...dynasty, statuses: { ...dynasty.statuses, scene: "ready" as const }, qc: { ...dynasty.qc, scene: passingQc } }
            : dynasty),
    };
    expect(weddingStageBlockers(qcPassed, "scenes")).toContain("明：场景首帧尚未确认");
    expect(weddingStageBlockers(qcPassed, "scenes")).toContain("明：场景 QC 未通过且没有覆盖记录");
});

test("a failed scene marks only its dynasty", () => {
    const failed = failWeddingSceneGeneration(
        production,
        weddingSceneJobs(production, "tang")[0]!,
        "男女定妆照尚未确认",
        "2026-10-25T03:00:00.000Z",
    );
    expect(failed.dynasties[0]?.statuses.scene).toBe("failed");
    expect(failed.dynasties[1]?.statuses.scene).toBe("ready");
    expect(failed.activity.at(-1)).toMatchObject({
        type: "error",
        stage: "scenes",
        dynastyId: "tang",
    });
});
