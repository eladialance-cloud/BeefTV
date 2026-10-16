// 工作台的纯函数：不碰 DOM、不碰网络，方便用 node --test 直接验证。
// 界面代码在 app.js，只从这里取格式化与常量映射。

export function escapeHtml(value) {
    return String(value ?? "").replace(/[&<>"']/gu, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[char]);
}

export function formatTime(value) {
    if (!value) return "-";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "-";
    const pad = (n) => String(n).padStart(2, "0");
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function formatPoints(value) {
    return Number(value || 0).toLocaleString("zh-CN");
}

const KIND_LABELS = { freeze: "冻结", consume: "消费", unfreeze: "退回", recharge: "充值", grant: "赠送", adjust: "人工调整" };
const STAGE_LABELS = { reserve: "预冻结", settle: "结算", release: "释放", credit: "入账" };

export function kindLabel(kind) {
    return KIND_LABELS[kind] || kind || "-";
}

export function formatFenToYuan(fen) {
    const value = Number(fen);
    if (!Number.isFinite(value)) return "0.00";
    return (value / 100).toFixed(2);
}


export function stageLabel(stage) {
    return STAGE_LABELS[stage] || stage || "-";
}

// resolveApiBase 依次看内联注入、本地存储、默认同源。返回结果去掉结尾斜杠。
export function resolveApiBase({ injected = "", stored = "" } = {}) {
    const candidate = String(injected || "").trim() || String(stored || "").trim();
    if (!candidate) return "/api";
    return candidate.replace(/\/+$/u, "");
}

// 每条人工调整带一个唯一幂等键，防止网络重试把同一笔调整记两次。
export function buildAdjustIdempotencyKey(userId, stamp, nonce) {
    return `adjust:${String(userId || "").trim()}:${stamp}:${nonce}`;
}
