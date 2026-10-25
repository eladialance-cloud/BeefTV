import { expect, test } from "bun:test";

import { buildWeddingPackagingTimeline } from "@/lib/wedding/packaging-timeline";
import { createWeddingProductionState } from "@/lib/wedding/production-state";

const production = createWeddingProductionState({
    template: { id: "template-1", key: "cross-era-wedding-volc", version: "1.0.0" },
    source: {
        groomPhotoNodeId: "groom",
        bridePhotoNodeId: "bride",
        maleName: "林屿",
        femaleName: "周棠",
        story: "",
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
production.packaging.intro.text = "片头标题";
production.packaging.outro.text = "片尾标题";

test("builds a deterministic title-video-title timeline with BGM", () => {
    const timeline = buildWeddingPackagingTimeline({
        production,
        intro: {
            nodeId: "intro-card",
            title: "片头标题",
            storageKey: "resource:intro",
            url: "https://example.com/intro.png",
            mimeType: "image/png",
            width: 1280,
            height: 720,
            durationMs: 4000,
        },
        outro: {
            nodeId: "outro-card",
            title: "片尾标题",
            storageKey: "resource:outro",
            url: "https://example.com/outro.png",
            mimeType: "image/png",
            width: 1280,
            height: 720,
            durationMs: 5000,
        },
        videos: [
            {
                nodeId: "asset-tang",
                dynastyId: "tang",
                title: "唐 · 朝代视频",
                storageKey: "resource:tang",
                url: "https://example.com/tang.mp4",
                mimeType: "video/mp4",
                width: 1280,
                height: 720,
                durationMs: 8000,
            },
            {
                nodeId: "asset-ming",
                dynastyId: "ming",
                title: "明 · 朝代视频",
                storageKey: "resource:ming",
                url: "https://example.com/ming.mp4",
                mimeType: "video/mp4",
                width: 1280,
                height: 720,
                durationMs: 10000,
            },
        ],
        bgm: {
            nodeId: "bgm-asset",
            title: "跨时空主题 BGM",
            storageKey: "resource:bgm",
            url: "https://example.com/bgm.mp3",
            mimeType: "audio/mpeg",
            durationMs: 27000,
        },
    });

    expect(timeline.clips.filter((clip) => clip.kind !== "audio").map((clip) => clip.title)).toEqual([
        "片头标题",
        "唐 · 朝代视频",
        "明 · 朝代视频",
        "片尾标题",
    ]);
    expect(timeline.clips.find((clip) => clip.kind === "audio")).toMatchObject({
        title: "跨时空主题 BGM",
        startMs: 0,
        durationMs: 27000,
    });
    expect(timeline.durationMs).toBe(27000);
    expect(timeline.tracks.map((track) => track.kind)).toEqual(["video", "audio", "subtitle"]);
});
