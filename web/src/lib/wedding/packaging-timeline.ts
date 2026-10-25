import { normalizeTimelineProject } from "@/lib/timeline/timeline-tracks";
import type { WeddingProductionState } from "@/lib/wedding/types";
import type {
    TimelineClip,
    TimelineDirectMedia,
    TimelineProject,
    TimelineTrack,
} from "@/types/timeline";

export type WeddingPackagingMedia = {
    nodeId: string;
    title: string;
    storageKey?: string;
    url?: string;
    mimeType?: string;
    width?: number;
    height?: number;
    durationMs: number;
};

export type WeddingPackagingVideo = WeddingPackagingMedia & {
    dynastyId: string;
};

export type WeddingPackagingTimelineInput = {
    production: WeddingProductionState;
    intro?: WeddingPackagingMedia;
    outro?: WeddingPackagingMedia;
    videos: WeddingPackagingVideo[];
    bgm?: WeddingPackagingMedia;
};

const VIDEO_TRACK = "video-1";
const AUDIO_TRACK = "audio-1";

export function buildWeddingPackagingTimeline(input: WeddingPackagingTimelineInput): TimelineProject {
    const tracks: TimelineTrack[] = [
        { id: VIDEO_TRACK, kind: "video", label: "成片", order: 0, visible: true },
        { id: AUDIO_TRACK, kind: "audio", label: "BGM", order: 1, visible: true },
    ];
    const clips: TimelineClip[] = [];
    let cursor = 0;
    const appendVisual = (media: WeddingPackagingMedia, kind: "image" | "video", title: string) => {
        const durationMs = Math.max(1, Math.round(media.durationMs || 1000));
        clips.push({
            id: `wedding-clip-${clips.length + 1}`,
            kind,
            nodeId: media.nodeId,
            trackId: VIDEO_TRACK,
            startMs: cursor,
            durationMs,
            sourceStartMs: 0,
            sourceDurationMs: durationMs,
            title,
            directMedia: directMedia(media, kind),
        });
        cursor += durationMs;
    };

    if (input.production.packaging.intro.enabled && input.intro) {
        appendVisual({
            ...input.intro,
            durationMs: input.production.packaging.intro.durationSeconds * 1000,
        }, "image", input.production.packaging.intro.text || "片头标题");
    }
    const videos = [...input.videos].sort((left, right) => {
        const leftOrder = input.production.dynasties.find((item) => item.id === left.dynastyId)?.order ?? 0;
        const rightOrder = input.production.dynasties.find((item) => item.id === right.dynastyId)?.order ?? 0;
        return leftOrder - rightOrder;
    });
    for (const video of videos) appendVisual(video, "video", video.title);
    if (input.production.packaging.outro.enabled && input.outro) {
        appendVisual({
            ...input.outro,
            durationMs: input.production.packaging.outro.durationSeconds * 1000,
        }, "image", input.production.packaging.outro.text || "片尾标题");
    }
    if (input.bgm && input.production.packaging.bgmMode !== "none") {
        clips.push({
            id: "wedding-bgm",
            kind: "audio",
            nodeId: input.bgm.nodeId,
            trackId: AUDIO_TRACK,
            startMs: 0,
            durationMs: cursor,
            sourceStartMs: 0,
            sourceDurationMs: input.bgm.durationMs || cursor,
            title: input.bgm.title || "跨时空主题 BGM",
            volume: 0.55,
            fadeInMs: 1000,
            fadeOutMs: 1500,
            directMedia: directMedia(input.bgm, "audio"),
        });
    }
    return normalizeTimelineProject({
        version: 2,
        tracks,
        clips,
        durationMs: Math.max(0, cursor),
        updatedAt: new Date().toISOString(),
    });
}

function directMedia(media: WeddingPackagingMedia, kind: TimelineDirectMedia["kind"]): TimelineDirectMedia {
    return {
        id: media.nodeId,
        kind,
        title: media.title,
        storageKey: media.storageKey,
        url: media.url,
        dataUrl: media.url,
        durationMs: media.durationMs,
        width: media.width,
        height: media.height,
        mimeType: media.mimeType,
    };
}
