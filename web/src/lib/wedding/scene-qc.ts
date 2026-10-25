import type {
    WeddingProductionState,
    WeddingSceneQc,
} from "@/lib/wedding/types";

export function sceneQcPrompt(dynastyName: string) {
    return `You are the hard QC gate for a ${dynastyName} dynasty wedding scene.
Compare the two confirmed portrait references with the generated couple scene.
Return JSON only. Do not use markdown. Use exactly these fields:
{
  "verdict": "pass" | "fail",
  "couplePresent": boolean,
  "faceMatchMale": boolean,
  "faceMatchFemale": boolean,
  "outfitMatch": boolean,
  "sceneMatch": boolean,
  "extraPerson": boolean,
  "watermarkOrText": boolean,
  "reason": "short factual Chinese explanation",
  "suggestion": "short actionable Chinese retry advice, empty when pass"
}
Set a face/outfit/scene boolean to false on visible mismatch. Set extraPerson or watermarkOrText to true when present. Never guess pass when an image is missing or unreadable.`;
}

export function parseWeddingSceneQc(text: string, checkedAt: string): WeddingSceneQc {
    const start = text.indexOf("{");
    const end = text.lastIndexOf("}");
    if (start < 0 || end <= start) throw new Error("场景 QC 没有返回 JSON");
    let raw: unknown;
    try {
        raw = JSON.parse(text.slice(start, end + 1));
    } catch {
        throw new Error("场景 QC JSON 格式错误");
    }
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) throw new Error("场景 QC 结果必须是对象");
    const record = raw as Record<string, unknown>;
    const couplePresent = booleanField(record, "couplePresent");
    const faceMatchMale = booleanField(record, "faceMatchMale");
    const faceMatchFemale = booleanField(record, "faceMatchFemale");
    const outfitMatch = booleanField(record, "outfitMatch");
    const sceneMatch = booleanField(record, "sceneMatch");
    const extraPerson = booleanField(record, "extraPerson");
    const watermarkOrText = booleanField(record, "watermarkOrText");
    const hardPass = couplePresent
        && faceMatchMale
        && faceMatchFemale
        && outfitMatch
        && sceneMatch
        && !extraPerson
        && !watermarkOrText;
    return {
        verdict: hardPass ? "pass" : "fail",
        couplePresent,
        faceMatchMale,
        faceMatchFemale,
        outfitMatch,
        sceneMatch,
        extraPerson,
        watermarkOrText,
        reason: stringField(record, "reason"),
        suggestion: stringField(record, "suggestion"),
        checkedAt,
    };
}

export function applyWeddingSceneQc(
    state: WeddingProductionState,
    dynastyId: string,
    qc: WeddingSceneQc,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === dynastyId);
    if (!dynasty) return state;
    dynasty.qc.scene = { ...qc, checkedAt: qc.checkedAt || now };
    dynasty.qc.overrides.scene = undefined;
    dynasty.statuses.scene = qc.verdict === "pass" && dynasty.sceneSelection ? "ready" : "needs_review";
    refreshSceneStage(next, now);
    return next;
}

export function errorWeddingSceneQc(
    state: WeddingProductionState,
    dynastyId: string,
    reason: string,
    now: string,
): WeddingProductionState {
    return applyWeddingSceneQc(state, dynastyId, {
        verdict: "error",
        couplePresent: false,
        faceMatchMale: false,
        faceMatchFemale: false,
        outfitMatch: false,
        sceneMatch: false,
        extraPerson: false,
        watermarkOrText: false,
        reason: reason.trim() || "场景 QC 执行失败",
        suggestion: "检查视觉模型配置和参考素材后重试",
        checkedAt: now,
    }, now);
}

export function overrideWeddingSceneQc(
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
    dynasty.qc.overrides.scene = { reason: normalized, acceptedAt: now };
    dynasty.statuses.scene = dynasty.sceneSelection ? "ready" : "needs_review";
    refreshSceneStage(next, now);
    return next;
}

function refreshSceneStage(state: WeddingProductionState, now: string) {
    const ready = state.dynasties.length > 0 && state.dynasties.every((dynasty) => {
        const qcReady = dynasty.qc.scene?.verdict === "pass" || Boolean(dynasty.qc.overrides.scene?.reason.trim());
        return Boolean(dynasty.sceneSelection) && qcReady;
    });
    state.stages.scenes = { status: ready ? "ready" : "needs_review", updatedAt: now };
}

function booleanField(record: Record<string, unknown>, key: string) {
    const value = record[key];
    if (typeof value !== "boolean") throw new Error(`场景 QC 字段 ${key} 必须是布尔值`);
    return value;
}

function stringField(record: Record<string, unknown>, key: string) {
    const value = record[key];
    if (typeof value !== "string") throw new Error(`场景 QC 字段 ${key} 必须是字符串`);
    return value.trim();
}
