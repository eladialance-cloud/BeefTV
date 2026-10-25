import type {
    WeddingProductionState,
    WeddingVideoQc,
} from "@/lib/wedding/types";

export function videoQcPrompt(dynastyName: string) {
    return `You are the hard QC gate for a ${dynastyName} dynasty wedding video.
Compare the selected video with the two confirmed portraits and the approved scene frame.
Return JSON only. Do not use markdown. Use exactly these fields:
{
  "verdict": "pass" | "fail",
  "faceStable": boolean,
  "outfitStable": boolean,
  "actionReadable": boolean,
  "extraPerson": boolean,
  "watermarkOrText": boolean,
  "reason": "short factual Chinese explanation"
}
Set faceStable false for identity drift or morphing. Set outfitStable false for costume discontinuity. Set actionReadable false when motion cannot be understood. Set extraPerson or watermarkOrText true when present. Never guess pass when the video is unreadable.`;
}

export function parseWeddingVideoQc(text: string, checkedAt: string): WeddingVideoQc {
    const start = text.indexOf("{");
    const end = text.lastIndexOf("}");
    if (start < 0 || end <= start) throw new Error("视频 QC 没有返回 JSON");
    let raw: unknown;
    try {
        raw = JSON.parse(text.slice(start, end + 1));
    } catch {
        throw new Error("视频 QC JSON 格式错误");
    }
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) throw new Error("视频 QC 结果必须是对象");
    const record = raw as Record<string, unknown>;
    const faceStable = booleanField(record, "faceStable");
    const outfitStable = booleanField(record, "outfitStable");
    const actionReadable = booleanField(record, "actionReadable");
    const extraPerson = booleanField(record, "extraPerson");
    const watermarkOrText = booleanField(record, "watermarkOrText");
    const hardPass = faceStable
        && outfitStable
        && actionReadable
        && !extraPerson
        && !watermarkOrText;
    return {
        verdict: hardPass ? "pass" : "fail",
        faceStable,
        outfitStable,
        actionReadable,
        extraPerson,
        watermarkOrText,
        reason: stringField(record, "reason"),
        checkedAt,
    };
}

export function applyWeddingVideoQc(
    state: WeddingProductionState,
    dynastyId: string,
    qc: WeddingVideoQc,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === dynastyId);
    if (!dynasty) return state;
    dynasty.qc.video = { ...qc, checkedAt: qc.checkedAt || now };
    dynasty.qc.overrides.video = undefined;
    dynasty.statuses.video = qc.verdict === "pass" && dynasty.videoSelection ? "ready" : "needs_review";
    refreshVideoStage(next, now);
    return next;
}

export function errorWeddingVideoQc(
    state: WeddingProductionState,
    dynastyId: string,
    reason: string,
    now: string,
): WeddingProductionState {
    return applyWeddingVideoQc(state, dynastyId, {
        verdict: "error",
        faceStable: false,
        outfitStable: false,
        actionReadable: false,
        extraPerson: false,
        watermarkOrText: false,
        reason: reason.trim() || "视频 QC 执行失败",
        checkedAt: now,
    }, now);
}

export function overrideWeddingVideoQc(
    state: WeddingProductionState,
    dynastyId: string,
    reason: string,
    now: string,
): WeddingProductionState {
    const normalized = reason.trim();
    if (!normalized) return state;
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === dynastyId);
    if (!dynasty) return state;
    dynasty.qc.overrides.video = { reason: normalized, acceptedAt: now };
    dynasty.statuses.video = dynasty.videoSelection ? "ready" : "needs_review";
    refreshVideoStage(next, now);
    return next;
}

function refreshVideoStage(state: WeddingProductionState, now: string) {
    const ready = state.dynasties.length > 0 && state.dynasties.every((dynasty) => {
        const qcReady = dynasty.qc.video?.verdict === "pass" || Boolean(dynasty.qc.overrides.video?.reason.trim());
        return Boolean(dynasty.videoSelection) && qcReady;
    });
    state.stages.videos = { status: ready ? "ready" : "needs_review", updatedAt: now };
}

function booleanField(record: Record<string, unknown>, key: string) {
    const value = record[key];
    if (typeof value !== "boolean") throw new Error(`视频 QC 字段 ${key} 必须是布尔值`);
    return value;
}

function stringField(record: Record<string, unknown>, key: string) {
    const value = record[key];
    if (typeof value !== "string") throw new Error(`视频 QC 字段 ${key} 必须是字符串`);
    return value.trim();
}
