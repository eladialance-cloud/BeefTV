// VisiDeep AI 管理后台：登录、钱包流水，以及用户积分与对象存储配置。
//
// 这是给网页版（server profile）用的独立前端，刻意不放进 web/src：
// web/src 的本地桌面边界测试明确禁止托管 UI 入口出现在桌面产物里。
// 本文件零依赖，直接以静态站点方式部署即可（见 README.md）。

import { buildAdjustIdempotencyKey, escapeHtml, formatFenToYuan, formatMicrocredits, formatPoints, formatTime, kindLabel, normalizeRmbCost, resolveApiBase, stageLabel } from "./format.js?v=20261018-4";

const API_BASE = (() => {
    const injected = typeof window !== "undefined" && typeof window.__BEEFTV_API_BASE__ === "string" ? window.__BEEFTV_API_BASE__ : "";
    return resolveApiBase({ injected, stored: localStorage.getItem("beeftv.apiBase") || "" });
})();

// 这个静态站就是管理后台，只服务管理员。用户端（注册登录、项目库、画布、充值）
// 是另一个应用，代码不放在这里，两边只共用后端 JSON 接口。
const ROUTES = [
    { path: "/admin/overview", label: "运营概览", section: "概览", icon: "chart" },
    { path: "/admin/users", label: "用户管理", section: "用户", icon: "users" },
    { path: "/admin/registration", label: "注册策略与邀请码", section: "用户", icon: "ticket" },
    { path: "/admin/credits", label: "积分账户", section: "用户", icon: "wallet" },

    { path: "/admin/projects", label: "项目管理", section: "内容", icon: "folder" },
    { path: "/admin/assets", label: "素材库", section: "内容", icon: "image" },

    { path: "/admin/workflows", label: "工作流模板", section: "创作模板", icon: "workflow" },
    { path: "/admin/prompts", label: "提示词库", section: "创作模板", icon: "sparkles" },
    { path: "/admin/styles", label: "画风预设", section: "创作模板", icon: "palette" },

    { path: "/admin/models", label: "模型渠道", section: "模型", icon: "cpu" },
    { path: "/admin/pricing", label: "价格规则", section: "模型", icon: "tag" },

    { path: "/admin/topup", label: "套餐与积分包", section: "商业化", icon: "card" },
    { path: "/admin/orders", label: "订单", section: "商业化", icon: "list" },
    { path: "/admin/ledger", label: "积分明细", section: "商业化", icon: "receipt" },
    { path: "/admin/payment", label: "支付渠道", section: "商业化", icon: "card" },

    { path: "/admin/site", label: "站点配置", section: "系统", icon: "settings" },
    { path: "/admin/storage", label: "对象存储", section: "系统", icon: "database" },
    { path: "/admin/audit", label: "审计日志", section: "系统", icon: "shield" },
];

const HOME_PATH = ROUTES[0].path;
const ADMIN_REMEMBER_KEY = "visideep.admin.login.remember";

function readRememberedAdminCredentials() {
    try {
        const raw = localStorage.getItem(ADMIN_REMEMBER_KEY);
        if (!raw) return null;
        const parsed = JSON.parse(raw);
        const username = typeof parsed?.username === "string" ? parsed.username : "";
        const password = typeof parsed?.password === "string" ? parsed.password : "";
        return username || password ? { username, password } : null;
    } catch {
        return null;
    }
}

function rememberAdminCredentials(username, password) {
    try {
        localStorage.setItem(ADMIN_REMEMBER_KEY, JSON.stringify({ username, password }));
    } catch {
        // 浏览器禁用本地存储时登录仍然成立，只是不再记住凭据。
    }
}

function forgetAdminCredentials() {
    try {
        localStorage.removeItem(ADMIN_REMEMBER_KEY);
    } catch {
        // 本地存储不可用时忽略。
    }
}

const state = {
    user: null,
    authMode: "login",
    accounts: [],
    storage: [],
    payments: [],
    pricingKind: "channel_tier",
    pricingPreset: null,
    pricingRules: [],
    pricingChannels: [],
    pricingBatchChannelId: "",
    pricingChannelModels: [],
    adminProducts: [],
    adminOrders: [],
    reconciliation: null,
    channels: [],
    channelTotal: 0,
    channelKeyword: "",
    channelStatus: "",
    selectedChannelId: "",
    channelModels: [],
    channelModelKeyword: "",
    channelModelCapability: "",
    channelModelCanvasStatus: "",
    channelModelTestingId: "",
    creationItems: [],
    creationKind: "workflow",
    creationTotal: 0,
    creationKeyword: "",
    creationStatus: "",
    registrationPolicy: null,
    inviteCodes: [],
    inviteTotal: 0,
    inviteKeyword: "",
    inviteStatus: "",
    adminOverview: null,
    adminOverviewDays: 7,
    adminUsers: [],
    adminUserTotal: 0,
    adminUserPage: 1,
    adminUserKeyword: "",
    adminUserRole: "",
    adminUserStatus: "",
    adminUserModalOpen: false,
    adminProjects: [],
    adminProjectTotal: 0,
    adminProjectPage: 1,
    adminProjectKind: "domain",
    adminProjectKeyword: "",
    adminProjectStatus: "",
    adminAssets: [],
    adminAssetTotal: 0,
    adminAssetPage: 1,
    adminAssetKeyword: "",
    adminAssetKind: "",
    adminAssetStatus: "",
    adminLedger: [],
    adminLedgerTotal: 0,
    adminLedgerPage: 1,
    adminLedgerKeyword: "",
    adminLedgerKind: "",
    adminLedgerModel: "",
    adminLedgerToken: "",
    adminLedgerStart: "",
    adminLedgerEnd: "",
    adminAudit: [],
    adminAuditTotal: 0,
    adminAuditPage: 1,
    adminAuditKeyword: "",
    adminAuditAction: "",
    adminAuditStart: "",
    adminAuditEnd: "",
    siteConfig: null,
    busy: false,
    error: "",
    notice: "",
};

state.channelCatalogChannelId = "";
state.channelCatalog = [];

class ApiError extends Error {
    constructor(message, status) {
        super(message);
        this.status = status;
    }
}

async function api(path, options = {}) {
    const response = await fetch(`${API_BASE}${path}`, {
        method: options.method || "GET",
        credentials: "include",
        headers: options.body ? { "Content-Type": "application/json" } : undefined,
        body: options.body ? JSON.stringify(options.body) : undefined,
    });
    let payload = null;
    try {
        payload = await response.json();
    } catch {
        payload = null;
    }
    if (!payload || typeof payload.code !== "number") {
        throw new ApiError(`后端返回异常（HTTP ${response.status}）`, response.status);
    }
    if (payload.code !== 0) {
        throw new ApiError(payload.msg || "请求失败", response.status);
    }
    return payload.data;
}

/* ---------------- 会话 ---------------- */

async function loadSession() {
    try {
        const data = await api("/auth/me");
        state.user = data.user;
        return true;
    } catch (error) {
        if (error instanceof ApiError && error.status === 401) return false;
        throw error;
    }
}

async function login(username, password) {
    const data = await api("/login", { method: "POST", body: { username, password } });
    state.user = data.user;
}

async function register(username, displayName, password, inviteCode = "") {
    const data = await api("/register", { method: "POST", body: { username, displayName, password, inviteCode } });
    state.user = data.user;
}

async function logout() {
    try {
        await api("/logout", { method: "POST" });
    } finally {
        state.user = null;
        state.authMode = "login";
        state.accounts = [];
        state.storage = [];
        state.payments = [];
        state.pricingRules = [];
        state.adminProducts = [];
        state.adminOrders = [];
        state.reconciliation = null;
    }
}

/* ---------------- 数据 ---------------- */


async function loadAccounts() {
    const data = await api("/admin/credits/accounts");
    state.accounts = data.items || [];
}

async function adjustCredits(userId, amount, reason) {
    return api("/admin/credits/adjust", {
        method: "POST",
        body: { userId, amountMicrocredits: amount, reason, idempotencyKey: buildAdjustIdempotencyKey(userId, Date.now(), Math.random().toString(36).slice(2, 8)) },
    });
}

function appendFilter(params, name, value) {
    const normalized = String(value || "").trim();
    if (normalized) params.set(name, normalized);
}

async function loadAdminOverview() {
    const data = await api(`/admin/overview?days=${encodeURIComponent(state.adminOverviewDays)}`);
    state.adminOverview = data.metrics || null;
}

async function loadAdminUsers() {
    const params = new URLSearchParams({ page: String(state.adminUserPage), pageSize: "30" });
    appendFilter(params, "keyword", state.adminUserKeyword);
    appendFilter(params, "role", state.adminUserRole);
    appendFilter(params, "status", state.adminUserStatus);
    const data = await api(`/admin/users?${params.toString()}`);
    state.adminUsers = data.items || [];
    state.adminUserTotal = data.total || 0;
}

async function createAdminUser(input) {
    return api("/admin/users", { method: "POST", body: input });
}

async function updateAdminUser(id, input) {
    return api(`/admin/users/${encodeURIComponent(id)}`, { method: "PATCH", body: input });
}

async function resetAdminUserPassword(id, password) {
    return api(`/admin/users/${encodeURIComponent(id)}/password`, { method: "POST", body: { password } });
}

async function loadAdminProjects() {
    const params = new URLSearchParams({ page: String(state.adminProjectPage), pageSize: "30" });
    appendFilter(params, "kind", state.adminProjectKind);
    appendFilter(params, "keyword", state.adminProjectKeyword);
    appendFilter(params, "status", state.adminProjectStatus);
    const data = await api(`/admin/projects?${params.toString()}`);
    state.adminProjects = data.items || [];
    state.adminProjectTotal = data.total || 0;
}

async function archiveAdminProject(id, kind, reason) {
    return api(`/admin/projects/${encodeURIComponent(id)}/archive`, { method: "POST", body: { kind, reason } });
}

async function restoreAdminProject(id, kind) {
    return api(`/admin/projects/${encodeURIComponent(id)}/restore`, { method: "POST", body: { kind } });
}

async function loadAdminAssets() {
    const params = new URLSearchParams({ page: String(state.adminAssetPage), pageSize: "30" });
    appendFilter(params, "keyword", state.adminAssetKeyword);
    appendFilter(params, "kind", state.adminAssetKind);
    appendFilter(params, "status", state.adminAssetStatus);
    const data = await api(`/admin/assets?${params.toString()}`);
    state.adminAssets = data.items || [];
    state.adminAssetTotal = data.total || 0;
}

async function blockAdminAsset(id, reason) {
    return api(`/admin/assets/${encodeURIComponent(id)}/block`, { method: "POST", body: { reason } });
}

async function unblockAdminAsset(id) {
    return api(`/admin/assets/${encodeURIComponent(id)}/unblock`, { method: "POST", body: {} });
}

async function loadAdminLedger() {
    const params = new URLSearchParams({ page: String(state.adminLedgerPage), pageSize: "30" });
    appendFilter(params, "keyword", state.adminLedgerKeyword);
    appendFilter(params, "kind", state.adminLedgerKind);
    appendFilter(params, "model", state.adminLedgerModel);
    appendFilter(params, "token", state.adminLedgerToken);
    appendFilter(params, "start", state.adminLedgerStart);
    appendFilter(params, "end", state.adminLedgerEnd);
    const data = await api("/admin/credit-ledger?" + params.toString());
    state.adminLedger = data.items || [];
    state.adminLedgerTotal = data.total || 0;
}

async function loadAdminAudit() {
    const params = new URLSearchParams({ page: String(state.adminAuditPage), pageSize: "30" });
    appendFilter(params, "keyword", state.adminAuditKeyword);
    appendFilter(params, "action", state.adminAuditAction);
    appendFilter(params, "start", state.adminAuditStart);
    appendFilter(params, "end", state.adminAuditEnd);
    const data = await api(`/admin/audit?${params.toString()}`);
    state.adminAudit = data.items || [];
    state.adminAuditTotal = data.total || 0;
}

async function loadAdminSiteConfig() {
    const data = await api("/admin/site");
    state.siteConfig = data.config || null;
}

async function saveAdminSiteConfig(input) {
    const data = await api("/admin/site", { method: "PUT", body: input });
    state.siteConfig = data.config || null;
}

async function loadStorage() {
    const data = await api("/admin/storage-locations");
    state.storage = data.items || [];
}

async function createStorage(input) {
    return api("/admin/storage-locations", { method: "POST", body: input });
}

async function testStorage(id) {
    return api(`/admin/storage-locations/${encodeURIComponent(id)}/test`, { method: "POST" });
}

async function setDefaultStorage(id) {
    return api(`/admin/storage-locations/${encodeURIComponent(id)}/default`, { method: "POST" });
}

async function deleteStorage(id) {
    return api(`/admin/storage-locations/${encodeURIComponent(id)}`, { method: "DELETE" });
}

async function loadPayments() {
    const data = await api("/admin/payment-providers");
    state.payments = data.items || [];
}

async function savePayment(id, input) {
    if (id) return api(`/admin/payment-providers/${encodeURIComponent(id)}`, { method: "PUT", body: input });
    return api("/admin/payment-providers", { method: "POST", body: input });
}

async function deletePayment(id) {
    return api(`/admin/payment-providers/${encodeURIComponent(id)}`, { method: "DELETE" });
}

async function loadPricing(kind) {
    const data = await api(`/admin/pricing/${encodeURIComponent(kind)}`);
    state.pricingKind = kind;
    state.pricingRules = data.items || [];
}

async function savePricing(kind, id, input) {
    if (id) return api(`/admin/pricing/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`, { method: "PUT", body: input });
    return api(`/admin/pricing/${encodeURIComponent(kind)}`, { method: "POST", body: input });
}

async function deletePricing(kind, id) {
    return api(`/admin/pricing/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`, { method: "DELETE" });
}

async function loadPricingChannelModels(channelId) {
    if (!channelId) {
        state.pricingChannelModels = [];
        return;
    }
    const data = await api(`/admin/channels/${encodeURIComponent(channelId)}/models`);
    state.pricingChannelModels = data.items || [];
}

async function loadPricingChannels() {
    const data = await api("/admin/channels?page=1&pageSize=200");
    state.pricingChannels = data.channels || [];
    if (!state.pricingChannels.some((item) => item.id === state.pricingBatchChannelId)) {
        state.pricingBatchChannelId = state.pricingChannels.length ? state.pricingChannels[0].id : "";
    }
    const preferredChannelId = state.pricingPreset?.channelId || state.pricingBatchChannelId;
    if (state.pricingChannels.some((item) => item.id === preferredChannelId)) {
        state.pricingBatchChannelId = preferredChannelId;
    } else if (!state.pricingChannels.some((item) => item.id === state.pricingBatchChannelId)) {
        state.pricingBatchChannelId = state.pricingChannels.length ? state.pricingChannels[0].id : "";
    }
    await loadPricingChannelModels(state.pricingBatchChannelId);
}

async function loadPricingWorkspace(kind) {
    state.pricingKind = kind;
    const tasks = [loadPricing(kind)];
    if (kind === "channel_tier") tasks.push(loadPricingChannels());
    await Promise.all(tasks);
}

async function loadAdminProducts() {
    const data = await api("/admin/topup-products");
    state.adminProducts = data.items || [];
}

async function saveAdminProduct(id, input) {
    if (id) return api(`/admin/topup-products/${encodeURIComponent(id)}`, { method: "PUT", body: input });
    return api("/admin/topup-products", { method: "POST", body: input });
}

async function deleteAdminProduct(id) {
    return api(`/admin/topup-products/${encodeURIComponent(id)}`, { method: "DELETE" });
}

async function loadAdminOrders() {
    const [orders, reconciliation] = await Promise.all([
        api("/admin/payment-orders?limit=100"),
        api("/admin/payment-orders/reconciliation"),
    ]);
    state.adminOrders = orders.items || [];
    state.reconciliation = reconciliation.item || null;
}

async function markOrderPaid(id, providerTradeNo) {
    return api(`/admin/payment-orders/${encodeURIComponent(id)}/mark-paid`, { method: "POST", body: { providerTradeNo } });
}

async function closeOrder(id, reason) {
    return api(`/admin/payment-orders/${encodeURIComponent(id)}/close`, { method: "POST", body: { reason } });
}

async function expireOrders() {
    return api("/admin/payment-orders/expire", { method: "POST" });
}

async function refundOrder(id, reason, providerRefundNo) {
    return api(`/admin/payment-orders/${encodeURIComponent(id)}/refund`, {
        method: "POST",
        body: { reason, providerRefundNo },
    });
}

async function loadChannels() {
    const params = new URLSearchParams({ page: "1", pageSize: "200" });
    if (state.channelKeyword) params.set("keyword", state.channelKeyword);
    if (state.channelStatus) params.set("status", state.channelStatus);
    const data = await api(`/admin/channels?${params.toString()}`);
    state.channels = data.channels || [];
    state.channelTotal = data.total || 0;
    if (!state.channels.some((item) => item.id === state.selectedChannelId)) {
        state.selectedChannelId = state.channels.length ? state.channels[0].id : "";
    }
    await loadChannelModels();
}

async function loadChannelModels() {
    if (!state.selectedChannelId) {
        state.channelModels = [];
        return;
    }
    const [data, pricing] = await Promise.all([
        api(`/admin/channels/${encodeURIComponent(state.selectedChannelId)}/models`),
        api("/admin/pricing/channel_tier").catch(() => ({ items: [] })),
    ]);
    state.channelModels = data.items || [];
    state.pricingKind = "channel_tier";
    state.pricingRules = pricing.items || [];
}

async function saveChannel(id, input) {
    if (id) return api(`/admin/channels/${encodeURIComponent(id)}`, { method: "PUT", body: input });
    return api("/admin/channels", { method: "POST", body: input });
}

async function duplicateChannel(id) {
    return api(`/admin/channels/${encodeURIComponent(id)}/duplicate`, { method: "POST" });
}

async function deleteChannel(id) {
    return api(`/admin/channels/${encodeURIComponent(id)}`, { method: "DELETE" });
}

async function saveChannelModel(channelId, id, input) {
    return api(`/admin/channels/${encodeURIComponent(channelId)}/models`, { method: "POST", body: { id, ...input } });
}

async function deleteChannelModel(channelId, id) {
    return api(`/admin/channels/${encodeURIComponent(channelId)}/models`, { method: "DELETE", body: { ids: [id] } });
}

async function testAdminChannelModel(channelId, model) {
    return api(`/admin/channels/${encodeURIComponent(channelId)}/models/test`, {
        method: "POST",
        body: {
            modelKey: model.modelKey,
            providerModelKey: model.providerModelKey || "",
            displayName: model.displayName || "",
            icon: model.icon || "",
            capability: model.capability,
            protocol: model.protocol,
            enabled: model.enabled,
            capabilityConfig: model.capabilityConfig || undefined,
            variants: model.variants || [],
        },
    });
}

async function fetchChannelModels(channelId) {
    return api(`/admin/channels/${encodeURIComponent(channelId)}/models/fetch`, { method: "POST" });
}

async function previewChannelModels(channelId) {
    return api(`/admin/channels/${encodeURIComponent(channelId)}/models/preview`, { method: "POST" });
}

async function loadCreationCatalog(kind) {
    const params = new URLSearchParams({ kind, page: "1", pageSize: "100" });
    if (state.creationKeyword) params.set("keyword", state.creationKeyword);
    if (state.creationStatus) params.set("status", state.creationStatus);
    const data = await api(`/admin/creation-catalog?${params.toString()}`);
    state.creationItems = data.items || [];
    state.creationTotal = data.total || 0;
}

async function saveCreationCatalog(id, input) {
    if (id) return api(`/admin/creation-catalog/${encodeURIComponent(id)}`, { method: "PUT", body: input });
    return api("/admin/creation-catalog", { method: "POST", body: input });
}

async function deleteCreationCatalog(id) {
    return api(`/admin/creation-catalog/${encodeURIComponent(id)}`, { method: "DELETE" });
}

async function loadRegistrationPolicy() {
    const data = await api("/admin/registration/policy");
    state.registrationPolicy = data.policy || null;
}

async function saveRegistrationPolicy(input) {
    const data = await api("/admin/registration/policy", { method: "PUT", body: input });
    state.registrationPolicy = data.policy || null;
}

async function loadInviteCodes() {
    const params = new URLSearchParams({ page: "1", pageSize: "100" });
    if (state.inviteKeyword) params.set("keyword", state.inviteKeyword);
    if (state.inviteStatus) params.set("status", state.inviteStatus);
    const data = await api(`/admin/invite-codes?${params.toString()}`);
    state.inviteCodes = data.items || [];
    state.inviteTotal = data.total || 0;
}

async function generateInviteCodes(input) {
    const data = await api("/admin/invite-codes", { method: "POST", body: input });
    return data.items || [];
}

async function setInviteCodeStatus(id, enabled) {
    return api(`/admin/invite-codes/${encodeURIComponent(id)}/${enabled ? "enable" : "disable"}`, { method: "POST" });
}

async function deleteInviteCode(id) {
    return api(`/admin/invite-codes/${encodeURIComponent(id)}`, { method: "DELETE" });
}

/* ---------------- 小工具 ---------------- */

const ICONS = {
    wallet: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7.5A2.5 2.5 0 0 1 5.5 5H18a2 2 0 0 1 2 2v1H5.5"/><path d="M3 7.5V17a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2H5.5A2.5 2.5 0 0 1 3 7.5Z"/><circle cx="16.5" cy="13.5" r="1"/></svg>',
    users: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M16 19v-1.5a3.5 3.5 0 0 0-3.5-3.5h-5A3.5 3.5 0 0 0 4 17.5V19"/><circle cx="10" cy="8" r="3.2"/><path d="M20 19v-1.5a3.5 3.5 0 0 0-2.6-3.4M15.5 5.2a3.2 3.2 0 0 1 0 5.6"/></svg>',
    database: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="6" rx="7.5" ry="3"/><path d="M4.5 6v12c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3V6"/><path d="M4.5 12c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3"/></svg>',
    logout: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M15 12H3.5"/><path d="m7 8-4 4 4 4"/><path d="M11 4h6a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-6"/></svg>',
    card: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="5" width="19" height="14" rx="2"/><path d="M2.5 9.5h19"/><path d="M6.5 14.5h3"/></svg>',
    list: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M8 6h12M8 12h12M8 18h12"/><circle cx="4" cy="6" r="1"/><circle cx="4" cy="12" r="1"/><circle cx="4" cy="18" r="1"/></svg>',
    tag: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M20 13 13 20 4 11V4h7l9 9Z"/><circle cx="8.5" cy="8.5" r="1"/></svg>',
    user: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>',
    lock: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="10.5" width="16" height="10" rx="2.5"/><path d="M8 10.5V8a4 4 0 0 1 8 0v2.5"/></svg>',
    eye: '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z"/><circle cx="12" cy="12" r="3"/></svg>',
    "eye-off": '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="m3 3 18 18"/><path d="M10.6 6.2A9.4 9.4 0 0 1 12 6c6 0 9.5 6 9.5 6a17.4 17.4 0 0 1-3.4 4.1"/><path d="M6.4 7.9A17 17 0 0 0 2.5 12S6 18 12 18a9.5 9.5 0 0 0 3.4-.6"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2"/></svg>',
    chart: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M4 20V10M10 20V4M16 20v-7M22 20H2"/></svg>',
    ticket: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 9V6.5A1.5 1.5 0 0 1 4.5 5h15A1.5 1.5 0 0 1 21 6.5V9a3 3 0 0 0 0 6v2.5a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 17.5V15a3 3 0 0 0 0-6Z"/><path d="M12 7.5v2M12 14.5v2"/></svg>',
    folder: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h3.5l2 2.5H19a2 2 0 0 1 2 2V17a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/></svg>',
    image: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4.5" width="18" height="15" rx="2"/><circle cx="8.5" cy="10" r="1.6"/><path d="m4 17 4.5-4.5 3.5 3.5 3-2.5L20 17"/></svg>',
    workflow: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3.5" width="6" height="5" rx="1.5"/><rect x="15" y="15.5" width="6" height="5" rx="1.5"/><path d="M6 8.5v5a3 3 0 0 0 3 3h6"/></svg>',
    sparkles: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3.5 13.6 8 18 9.6 13.6 11.2 12 15.7 10.4 11.2 6 9.6 10.4 8Z"/><path d="M18.5 15.5l.7 2 2 .7-2 .7-.7 2-.7-2-2-.7 2-.7Z"/></svg>',
    palette: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3.5c4.7 0 8.5 3.3 8.5 7.4 0 2.6-2.2 4.1-4.4 4.1h-1.3a2 2 0 0 0-1.4 3.4c.5.5.2 1.6-.9 1.9-4.1 1.1-8.9-2.3-8.9-7.5C3.6 7 7.3 3.5 12 3.5Z"/><circle cx="8" cy="10" r="1"/><circle cx="12" cy="7.5" r="1"/><circle cx="16" cy="10" r="1"/></svg>',
    cpu: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="6" width="12" height="12" rx="2"/><rect x="9.5" y="9.5" width="5" height="5" rx="1"/><path d="M10 3v3M14 3v3M10 18v3M14 18v3M3 10h3M3 14h3M18 10h3M18 14h3"/></svg>',
    receipt: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M6 3h12v18l-2-1.4-2 1.4-2-1.4L10 21l-2-1.4L6 21Z"/><path d="M9.5 8h5M9.5 12h5"/></svg>',
    settings: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 14.5a1.7 1.7 0 0 0 .4 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-2.9 1.2v.2a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.6 1.7 1.7 0 0 0-1.9.4l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0-1.2-2.9H2.9a2 2 0 1 1 0-4h.1A1.7 1.7 0 0 0 4.6 8a1.7 1.7 0 0 0-.4-1.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.9.4H9a1.7 1.7 0 0 0 1-1.6V2a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 2.9 1.2l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.4 1.9v.1a1.7 1.7 0 0 0 1.6 1h.2a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.6 1Z"/></svg>',
    shield: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3.5 19 6v6c0 4.2-2.9 7.2-7 8.5-4.1-1.3-7-4.3-7-8.5V6Z"/><path d="m9.5 12 1.8 1.8 3.7-3.6"/></svg>',
};

function icon(name) {
    return ICONS[name] || "";
}

function currentPath() {
    const raw = (window.location.hash || "").replace(/^#/u, "");
    return raw || HOME_PATH;
}

function navigate(path) {
    window.location.hash = path;
}

function clearFlash() {
    state.error = "";
    state.notice = "";
}

/* ---------------- 渲染 ---------------- */

const app = document.getElementById("app");

function render() {
    if (!state.user) {
        renderLogin();
        return;
    }
    if (state.user.role !== "admin") {
        renderNoAccess();
        return;
    }
    const route = ROUTES.find((item) => item.path === currentPath()) || ROUTES[0];
    app.innerHTML = `
        <div class="shell">
            <aside class="sidebar">
                <div class="sidebar-brand">
                    <span class="auth-brand-mark"><img src="./assets/visideep-ai.png" alt="" /></span>
                    <span class="auth-brand-name">VisiDeep AI 管理后台</span>
                </div>
                <nav class="sidebar-nav">
                    ${ROUTES.map((item, index, list) => {
                        const header = item.section !== list[index - 1]?.section ? `<div class="nav-section">${escapeHtml(item.section)}</div>` : "";
                        return `${header}<a class="nav-item" href="#${item.path}" ${item.path === route.path ? 'aria-current="page"' : ""}>${icon(item.icon)}<span>${escapeHtml(item.label)}</span></a>`;
                    }).join("")}
                </nav>
                <div class="sidebar-foot">
                    <div class="who">
                        <span class="who-name">${escapeHtml(state.user.displayName || state.user.username)}</span>
                        <span class="who-role">管理员</span>
                    </div>
                    <button class="btn btn-sm" data-action="logout">${icon("logout")}退出登录</button>
                </div>
            </aside>
            <main class="main" id="view"></main>
        </div>`;
    const view = document.getElementById("view");
    if (route.path === "/admin/credits") renderAdminUsers(view);
    else if (route.path === "/admin/overview") renderAdminOverview(view);
    else if (route.path === "/admin/users") renderAdminUserManagement(view);
    else if (route.path === "/admin/projects") renderAdminProjects(view);
    else if (route.path === "/admin/assets") renderAdminAssets(view);
    else if (route.path === "/admin/ledger") renderAdminLedger(view);
    else if (route.path === "/admin/site") renderAdminSite(view);
    else if (route.path === "/admin/audit") renderAdminAudit(view);
    else if (route.path === "/admin/registration") renderAdminRegistration(view);
    else if (route.path === "/admin/topup") renderAdminTopup(view);
    else if (route.path === "/admin/orders") renderAdminOrders(view);
    else if (route.path === "/admin/storage") renderAdminStorage(view);
    else if (route.path === "/admin/payment") renderAdminPayment(view);
    else if (route.path === "/admin/pricing") renderAdminPricing(view);
    else if (route.path === "/admin/models") renderAdminChannels(view);
    else if (route.path === "/admin/workflows") renderAdminCreationCatalog(view, "workflow");
    else if (route.path === "/admin/prompts") renderAdminCreationCatalog(view, "prompt");
    else if (route.path === "/admin/styles") renderAdminCreationCatalog(view, "style");
}

function renderFlash() {
    if (state.error) return `<div class="banner banner-error">${escapeHtml(state.error)}</div>`;
    if (state.notice) return `<div class="banner banner-ok">${escapeHtml(state.notice)}</div>`;
    return "";
}

function renderLogin() {
    const mode = state.authMode === "register" ? "register" : "login";
    const isRegister = mode === "register";
    const remembered = isRegister ? null : readRememberedAdminCredentials();
    app.innerHTML = `
        <div class="auth-page">
            <aside class="auth-visual">
                <img class="auth-visual-img" src="./assets/login-hero.jpg" alt="" />
                <div class="auth-visual-scrim"></div>
                <div class="auth-visual-copy">
                    <div class="auth-visual-brand"><img class="auth-mark" src="./assets/visideep-ai.png" alt="" />VisiDeep AI</div>
                    <p class="auth-visual-tag">释放您的无限创意潜能，打造独一无二的 AI 短剧作品。</p>
                </div>
            </aside>
            <section class="auth-panel">
                <div class="auth-form-wrap">
                    <div class="auth-mobile-brand"><img class="auth-mark" src="./assets/visideep-ai.png" alt="" />VisiDeep AI</div>
                    <h1 class="auth-title">${isRegister ? "创建账号" : "欢迎登录"}</h1>
                    <p class="auth-sub">${isRegister ? "注册后即可创建你的 AI 短剧项目" : "登录后即可进入工作台继续创作"}</p>
                    <div id="flash" class="auth-flash">${renderFlash()}</div>
                    <form id="auth-form" autocomplete="on">
                        <label class="auth-field">
                            <span class="auth-label">账号</span>
                            <span class="auth-input">${icon("user")}<input name="username" autocomplete="username" placeholder="请输入手机号 / 用户名" value="${escapeHtml(remembered?.username || "")}" required /></span>
                        </label>
                        ${isRegister ? `<label class="auth-field"><span class="auth-label">昵称（可选）</span><span class="auth-input">${icon("user")}<input name="displayName" autocomplete="nickname" placeholder="请输入昵称" /></span></label>` : ""}
                        ${isRegister ? `<label class="auth-field"><span class="auth-label">邀请码（仅邀请注册时需要）</span><span class="auth-input">${icon("ticket")}<input name="inviteCode" autocomplete="off" placeholder="请输入邀请码" /></span></label>` : ""}
                        <label class="auth-field">
                            <span class="auth-label">密码</span>
                            <span class="auth-input">${icon("lock")}<input id="auth-password" name="password" type="password" autocomplete="${isRegister ? "new-password" : "current-password"}" placeholder="请输入密码" value="${escapeHtml(remembered?.password || "")}" required /><button class="auth-eye" type="button" data-action="toggle-password" aria-label="显示密码">${icon("eye")}</button></span>
                        </label>
                        ${isRegister ? "" : `<label class="auth-remember"><input name="remember" type="checkbox" ${remembered ? "checked" : ""} />记住账号密码</label>`}
                        <button class="auth-submit" type="submit" ${state.busy ? "disabled" : ""}>${isRegister ? "注册并登录" : "登录"}</button>
                    </form>
                    <p class="auth-switch">${isRegister ? "已有账号？" : "没有账号？"}<button type="button" data-action="auth-mode" data-mode="${isRegister ? "login" : "register"}">${isRegister ? "去登录" : "去注册"}</button></p>
                </div>
            </section>
        </div>`;
}

function registrationModeLabel(mode) {
    if (mode === "invite") return "仅邀请码注册";
    if (mode === "closed") return "关闭注册";
    return "开放注册";
}

function inviteCodeStateTag(item) {
    if (item.status !== "active") return '<span class="tag tag-muted">已停用</span>';
    if (item.expiresAt && new Date(item.expiresAt).getTime() <= Date.now()) return '<span class="tag tag-warn">已过期</span>';
    if (item.maxUses > 0 && item.usedCount >= item.maxUses) return '<span class="tag tag-warn">已用完</span>';
    return '<span class="tag tag-ok">可用</span>';
}

function renderInviteCodes() {
    if (!state.inviteCodes.length) return '<div class="empty">还没有邀请码，先在上方批量生成。</div>';
    return `
        <div class="table-wrap">
            <table>
                <thead><tr><th>邀请码</th><th>批次</th><th class="num">已用 / 上限</th><th>有效期</th><th>状态</th><th>备注</th><th></th></tr></thead>
                <tbody>
                    ${state.inviteCodes.map((item) => `<tr>
                        <td><code>${escapeHtml(item.code)}</code></td>
                        <td class="muted">${escapeHtml(item.batch || "-")}</td>
                        <td class="num">${escapeHtml(String(item.usedCount || 0))} / ${item.maxUses > 0 ? escapeHtml(String(item.maxUses)) : "不限"}</td>
                        <td class="muted">${item.expiresAt ? escapeHtml(formatTime(item.expiresAt)) : "永久"}</td>
                        <td>${inviteCodeStateTag(item)}</td>
                        <td class="muted">${escapeHtml(item.note || "-")}</td>
                        <td><div class="btn-row">
                            <button class="btn btn-sm" data-action="copy-invite" data-code="${escapeHtml(item.code)}">复制</button>
                            <button class="btn btn-sm" data-action="toggle-invite" data-id="${escapeHtml(item.id)}" data-enabled="${item.status === "active" ? "false" : "true"}">${item.status === "active" ? "停用" : "启用"}</button>
                            <button class="btn btn-sm btn-danger" data-action="delete-invite" data-id="${escapeHtml(item.id)}">删除</button>
                        </div></td>
                    </tr>`).join("")}
                </tbody>
            </table>
        </div>`;
}

function renderAdminRegistration(view) {
    const policy = state.registrationPolicy || { mode: "open", defaultBonusCredits: 0, inviteDefaultMaxUses: 1, inviteDefaultValidDays: 30 };
    view.innerHTML = `
        <div class="page-head">
            <div><h1 class="page-title">注册策略与邀请码</h1><p class="page-sub">注册开关由管理员决定，邀请码的生成、启停和核销次数都可在后台管理。</p></div>
            <button class="btn btn-sm" data-action="refresh">刷新</button>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel">
            <h2 class="panel-title">注册策略</h2>
            <form id="registration-policy-form">
                <div class="form-grid">
                    <label class="field"><span class="field-label">注册模式</span><select class="select" name="mode">
                        <option value="open" ${policy.mode === "open" ? "selected" : ""}>开放注册</option>
                        <option value="invite" ${policy.mode === "invite" ? "selected" : ""}>仅邀请码注册</option>
                        <option value="closed" ${policy.mode === "closed" ? "selected" : ""}>关闭注册</option>
                    </select></label>
                    <label class="field"><span class="field-label">新用户默认赠送积分</span><input class="input" name="defaultBonusCredits" type="number" step="1" min="0" value="${escapeHtml(String(policy.defaultBonusCredits || 0))}" /></label>
                    <label class="field"><span class="field-label">邀请码默认可用次数</span><input class="input" name="inviteDefaultMaxUses" type="number" step="1" min="1" value="${escapeHtml(String(policy.inviteDefaultMaxUses || 1))}" /></label>
                    <label class="field"><span class="field-label">邀请码默认有效期（天，0 为永久）</span><input class="input" name="inviteDefaultValidDays" type="number" step="1" min="0" value="${escapeHtml(String(policy.inviteDefaultValidDays ?? 30))}" /></label>
                </div>
                <div class="btn-row"><span class="muted">当前：${escapeHtml(registrationModeLabel(policy.mode))}</span><button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>保存策略</button></div>
            </form>
        </div>
        <div class="panel">
            <h2 class="panel-title">批量生成邀请码</h2>
            <form id="invite-generate-form">
                <div class="form-grid">
                    <label class="field"><span class="field-label">生成数量（1-500）</span><input class="input" name="count" type="number" min="1" max="500" value="10" required /></label>
                    <label class="field"><span class="field-label">批次（留空自动生成）</span><input class="input" name="batch" placeholder="例如：2026-10 活动" /></label>
                    <label class="field"><span class="field-label">码前缀（最多 8 位，可选）</span><input class="input" name="prefix" maxlength="8" placeholder="例如：VIP" /></label>
                    <label class="field"><span class="field-label">每码可用次数</span><input class="input" name="maxUses" type="number" min="1" value="${escapeHtml(String(policy.inviteDefaultMaxUses || 1))}" /></label>
                    <label class="field"><span class="field-label">有效期（天，0 为永久）</span><input class="input" name="validDays" type="number" min="0" value="${escapeHtml(String(policy.inviteDefaultValidDays ?? 30))}" /></label>
                    <label class="field"><span class="field-label">备注（可选）</span><input class="input" name="note" /></label>
                </div>
                <div class="btn-row"><label class="muted"><input type="checkbox" name="autoEnabled" checked /> 生成后直接启用</label><button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>生成邀请码</button></div>
            </form>
        </div>
        <div class="panel">
            <h2 class="panel-title">邀请码列表（${state.inviteTotal}）</h2>
            <form id="invite-filter-form" class="filter-row">
                <input class="input" name="keyword" placeholder="邀请码 / 批次 / 备注" value="${escapeHtml(state.inviteKeyword)}" />
                <select class="select" name="status"><option value="">全部状态</option><option value="active" ${state.inviteStatus === "active" ? "selected" : ""}>可用</option><option value="disabled" ${state.inviteStatus === "disabled" ? "selected" : ""}>已停用</option><option value="expired" ${state.inviteStatus === "expired" ? "selected" : ""}>已过期</option></select>
                <button class="btn btn-sm" type="submit">查询</button>
            </form>
            ${renderInviteCodes()}
        </div>`;
}

function renderNoAccess() {
    app.innerHTML = `
        <div class="auth-page">
            <aside class="auth-visual">
                <img class="auth-visual-img" src="./assets/login-hero.jpg" alt="" />
                <div class="auth-visual-scrim"></div>
                <div class="auth-visual-copy">
                    <div class="auth-visual-brand"><img class="auth-mark" src="./assets/visideep-ai.png" alt="" />VisiDeep AI</div>
                    <p class="auth-visual-tag">管理后台只对管理员开放。</p>
                </div>
            </aside>
            <section class="auth-panel">
                <div class="auth-form-wrap">
                    <h1 class="auth-title">没有管理后台权限</h1>
                    <p class="auth-sub">当前账号 ${escapeHtml(state.user.displayName || state.user.username)} 是普通用户。请换管理员账号登录，或到用户端使用画布。</p>
                    <button class="auth-submit" type="button" data-action="logout">退出登录</button>
                </div>
            </section>
        </div>`;
}

function metric(label, value, suffix = "") {
    return `<div class="metric"><div class="metric-label">${escapeHtml(label)}</div><div class="metric-value">${escapeHtml(value)}${suffix ? `<small>${escapeHtml(suffix)}</small>` : ""}</div></div>`;
}


function userRoleLabel(role) {
    return role === "admin" ? "管理员" : "普通用户";
}

function userStatusTag(status) {
    return status === "active" ? '<span class="tag tag-ok">正常</span>' : '<span class="tag tag-warn">已停用</span>';
}

function formatBytes(value) {
    const bytes = Number(value || 0);
    if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
    const units = ["B", "KB", "MB", "GB", "TB"];
    const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const amount = bytes / 1024 ** index;
    return `${amount >= 10 || index === 0 ? amount.toFixed(0) : amount.toFixed(1)} ${units[index]}`;
}

function adminPager(entity, page, total, pageSize = 30) {
    const pages = Math.max(1, Math.ceil(Number(total || 0) / pageSize));
    const current = Math.min(Math.max(1, Number(page || 1)), pages);
    return `<div class="pager"><span class="muted">第 ${current} / ${pages} 页，共 ${formatPoints(total)} 条</span><div class="btn-row"><button class="btn btn-sm" data-action="admin-page" data-entity="${entity}" data-page="${current - 1}" ${current <= 1 ? "disabled" : ""}>上一页</button><button class="btn btn-sm" data-action="admin-page" data-entity="${entity}" data-page="${current + 1}" ${current >= pages ? "disabled" : ""}>下一页</button></div></div>`;
}

function renderAdminOverview(view) {
    const metrics = state.adminOverview || {};
    const successRate = `${(Number(metrics.taskSuccessRate || 0) * 100).toFixed(1)}%`;
    view.innerHTML = `
        <div class="page-head">
            <div><h1 class="page-title">运营概览</h1><p class="page-sub">按周期查看用户、创作、积分、营收和存储的整体运行状态。</p></div>
            <div class="btn-row" role="group" aria-label="统计周期">
                ${[7, 30, 90].map((days) => `<button class="btn btn-sm ${state.adminOverviewDays === days ? "btn-primary" : ""}" data-action="overview-days" data-days="${days}">近 ${days} 天</button>`).join("")}
                <button class="btn btn-sm" data-action="refresh" ${state.busy ? "disabled" : ""}>刷新</button>
            </div>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel">
            <h2 class="panel-title">用户与创作</h2>
            <div class="metric-row">
                ${metric("新增用户", formatPoints(metrics.newUsers))}
                ${metric("活跃用户", formatPoints(metrics.activeUsers))}
                ${metric("新建项目", formatPoints(metrics.projects))}
                ${metric("新建画布", formatPoints(metrics.canvases))}
                ${metric("生成任务", formatPoints(metrics.tasks))}
                ${metric("任务成功率", successRate)}
            </div>
        </div>
        <div class="grid-2">
            <div class="panel">
                <h2 class="panel-title">积分与营收</h2>
                <div class="metric-row">
                    ${metric("消耗积分", formatMicrocredits(metrics.consumedCredits))}
                    ${metric("充值积分", formatMicrocredits(metrics.rechargeCredits))}
                    ${metric("支付收入", `￥${formatFenToYuan(metrics.revenueFen)}`)}
                    ${metric("待支付订单", formatPoints(metrics.pendingOrders))}
                </div>
            </div>
            <div class="panel">
                <h2 class="panel-title">内容与存储</h2>
                <div class="metric-row">
                    ${metric("新增素材", formatPoints(metrics.assets))}
                    ${metric("全站存储", formatBytes(metrics.storageBytes))}
                </div>
                <p class="panel-note">统计口径来自真实业务表；生成任务与积分结算分开计数，便于定位成本异常。</p>
            </div>
        </div>`;
}

function renderUserCreateModal() {
    if (!state.adminUserModalOpen) return "";
    return `<div class="modal-backdrop" id="user-create-modal">
        <section class="modal" role="dialog" aria-modal="true" aria-labelledby="user-create-title">
            <div class="modal-head"><div><h2 class="panel-title" id="user-create-title">添加用户</h2><p class="modal-sub">创建账号不会替换当前管理员会话；初始积分单独写入不可变流水。</p></div><button class="btn btn-sm" type="button" data-action="close-create-user">关闭</button></div>
            <form id="create-user-form">
                <div class="modal-body"><div class="form-grid">
                    <label class="field"><span class="field-label">登录账号</span><input class="input" name="username" autocomplete="off" required /></label>
                    <label class="field"><span class="field-label">显示昵称</span><input class="input" name="displayName" autocomplete="off" /></label>
                    <label class="field"><span class="field-label">初始密码</span><input class="input" name="password" type="password" minlength="8" autocomplete="new-password" required /></label>
                    <label class="field"><span class="field-label">初始积分</span><input class="input" name="initialCredits" type="number" min="0" step="1" value="0" /></label>
                </div></div>
                <div class="modal-foot"><span class="muted">账号默认角色为普通用户</span><div class="btn-row"><button class="btn btn-sm" type="button" data-action="close-create-user">取消</button><button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>创建账号</button></div></div>
            </form>
        </section>
    </div>`;
}

function renderAdminUserManagement(view) {
    const rows = state.adminUsers.map((item) => `<tr>
        <td><strong>${escapeHtml(item.displayName || item.username)}</strong><div class="cell-sub">${escapeHtml(item.username)}</div></td>
        <td class="muted">${escapeHtml(item.id)}</td>
        <td>${escapeHtml(userRoleLabel(item.role))}</td>
        <td>${userStatusTag(item.status)}</td>
        <td class="num">${escapeHtml(formatMicrocredits(item.balance))}<div class="cell-sub">可用 ${escapeHtml(formatMicrocredits(item.available))}</div></td>
        <td class="muted">${escapeHtml(formatTime(item.createdAt))}</td>
        <td><div class="btn-row">
            <button class="btn btn-sm" data-action="toggle-user-status" data-id="${escapeHtml(item.id)}" data-status="${item.status === "active" ? "disabled" : "active"}" data-role="${escapeHtml(item.role)}" ${item.id === state.user.id ? "disabled" : ""}>${item.status === "active" ? "停用" : "启用"}</button>
            <button class="btn btn-sm" data-action="toggle-user-role" data-id="${escapeHtml(item.id)}" data-role="${item.role === "admin" ? "user" : "admin"}" data-status="${escapeHtml(item.status)}" ${item.id === state.user.id ? "disabled" : ""}>${item.role === "admin" ? "降为用户" : "设为管理员"}</button>
            <button class="btn btn-sm" data-action="reset-user-password" data-id="${escapeHtml(item.id)}">重置密码</button>
        </div></td>
    </tr>`).join("");
    view.innerHTML = `
        <div class="page-head">
            <div><h1 class="page-title">用户管理</h1><p class="page-sub">管理账号角色、启停状态和密码，所有写操作都会记录管理员审计。</p></div>
            <div class="btn-row"><button class="btn btn-primary btn-sm" data-action="open-create-user">添加用户</button><button class="btn btn-sm" data-action="refresh">刷新</button></div>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel">
            <h2 class="panel-title">账号列表（${formatPoints(state.adminUserTotal)}）</h2>
            <form id="user-filter-form" class="filter-row">
                <input class="input" name="keyword" placeholder="账号 / 昵称 / 用户 ID" value="${escapeHtml(state.adminUserKeyword)}" />
                <select class="select" name="role"><option value="">全部角色</option><option value="user" ${state.adminUserRole === "user" ? "selected" : ""}>普通用户</option><option value="admin" ${state.adminUserRole === "admin" ? "selected" : ""}>管理员</option></select>
                <select class="select" name="status"><option value="">全部状态</option><option value="active" ${state.adminUserStatus === "active" ? "selected" : ""}>正常</option><option value="disabled" ${state.adminUserStatus === "disabled" ? "selected" : ""}>已停用</option></select>
                <button class="btn btn-sm" type="submit">查询</button>
            </form>
            <div class="table-wrap"><table><thead><tr><th>用户</th><th>用户 ID</th><th>角色</th><th>状态</th><th class="num">积分</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${rows || '<tr><td class="empty" colspan="7">没有符合条件的用户</td></tr>'}</tbody></table></div>
            ${adminPager("users", state.adminUserPage, state.adminUserTotal)}
        </div>
        ${renderUserCreateModal()}`;
    document.body.classList.toggle("modal-open", state.adminUserModalOpen);
}

function projectState(item) {
    return item.kind === "canvas" ? item.moderationStatus || "active" : item.status || "active";
}

function projectStateTag(item) {
    const status = projectState(item);
    if (status === "active") return '<span class="tag tag-ok">正常</span>';
    if (status === "archived") return '<span class="tag tag-muted">已归档</span>';
    return '<span class="tag tag-warn">已下架</span>';
}

function renderAdminProjects(view) {
    const rows = state.adminProjects.map((item) => {
        const closed = item.kind === "canvas" ? item.moderationStatus === "blocked" : item.status === "archived";
        const action = closed
            ? `<button class="btn btn-sm" data-action="restore-project" data-id="${escapeHtml(item.id)}" data-kind="${escapeHtml(item.kind)}">恢复</button>`
            : `<button class="btn btn-sm btn-danger" data-action="archive-project" data-id="${escapeHtml(item.id)}" data-kind="${escapeHtml(item.kind)}">归档/下架</button>`;
        return `<tr><td><strong>${escapeHtml(item.name)}</strong><div class="cell-sub">${escapeHtml(item.id)}</div></td><td>${escapeHtml(item.displayName || item.username || item.userId)}<div class="cell-sub">${escapeHtml(item.username || item.userId)}</div></td><td>${item.kind === "canvas" ? "画布" : "项目"}</td><td>${projectStateTag(item)}</td><td class="muted">${escapeHtml(item.moderationReason || "-")}</td><td class="muted">${escapeHtml(formatTime(item.updatedAt))}</td><td>${action}</td></tr>`;
    }).join("");
    view.innerHTML = `
        <div class="page-head"><div><h1 class="page-title">项目管理</h1><p class="page-sub">统一查看域名项目和画布项目；违规内容只下架或归档，不做物理删除。</p></div><button class="btn btn-sm" data-action="refresh">刷新</button></div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel"><h2 class="panel-title">项目列表（${formatPoints(state.adminProjectTotal)}）</h2>
            <form id="project-filter-form" class="filter-row">
                <select class="select" name="kind"><option value="domain" ${state.adminProjectKind === "domain" ? "selected" : ""}>域名项目</option><option value="canvas" ${state.adminProjectKind === "canvas" ? "selected" : ""}>画布项目</option></select>
                <input class="input" name="keyword" placeholder="项目名 / 用户 / ID" value="${escapeHtml(state.adminProjectKeyword)}" />
                <select class="select" name="status"><option value="">全部状态</option><option value="active" ${state.adminProjectStatus === "active" ? "selected" : ""}>正常</option>${state.adminProjectKind === "domain" ? `<option value="archived" ${state.adminProjectStatus === "archived" ? "selected" : ""}>已归档</option>` : `<option value="blocked" ${state.adminProjectStatus === "blocked" ? "selected" : ""}>已下架</option>`}</select>
                <button class="btn btn-sm" type="submit">查询</button>
            </form>
            <div class="table-wrap"><table><thead><tr><th>项目</th><th>所属用户</th><th>类型</th><th>状态</th><th>处理原因</th><th>更新时间</th><th>操作</th></tr></thead><tbody>${rows || '<tr><td class="empty" colspan="7">没有符合条件的项目</td></tr>'}</tbody></table></div>
            ${adminPager("projects", state.adminProjectPage, state.adminProjectTotal)}
        </div>`;
}

function assetKindLabel(kind) {
    if (kind === "image") return "图片";
    if (kind === "video") return "视频";
    if (kind === "audio") return "音频";
    return kind || "其他";
}

function renderAdminAssets(view) {
    const rows = state.adminAssets.map((item) => {
        const blocked = item.moderationStatus === "blocked";
        return `<tr><td><strong>${escapeHtml(item.title || item.id)}</strong><div class="cell-sub">${escapeHtml(item.id)}</div></td><td>${escapeHtml(assetKindLabel(item.kind))}<div class="cell-sub">${escapeHtml(item.category || "-")}</div></td><td class="num">${escapeHtml(formatBytes(item.sizeBytes))}</td><td>${escapeHtml(item.displayName || item.username || item.userId)}<div class="cell-sub">${escapeHtml(item.username || item.userId)}</div></td><td>${blocked ? '<span class="tag tag-warn">已下架</span>' : '<span class="tag tag-ok">正常</span>'}<div class="cell-sub">${escapeHtml(item.moderationReason || "")}</div></td><td class="muted">${escapeHtml(formatTime(item.createdAt))}</td><td>${blocked ? `<button class="btn btn-sm" data-action="unblock-asset" data-id="${escapeHtml(item.id)}">恢复</button>` : `<button class="btn btn-sm btn-danger" data-action="block-asset" data-id="${escapeHtml(item.id)}">下架</button>`}</td></tr>`;
    }).join("");
    view.innerHTML = `
        <div class="page-head"><div><h1 class="page-title">素材库</h1><p class="page-sub">查看用户素材、存储占用和治理状态；下架后用户素材库立即隐藏。</p></div><button class="btn btn-sm" data-action="refresh">刷新</button></div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel"><h2 class="panel-title">素材列表（${formatPoints(state.adminAssetTotal)}）</h2>
            <form id="asset-filter-form" class="filter-row">
                <input class="input" name="keyword" placeholder="素材名 / 用户 / ID" value="${escapeHtml(state.adminAssetKeyword)}" />
                <select class="select" name="kind"><option value="">全部类型</option><option value="image" ${state.adminAssetKind === "image" ? "selected" : ""}>图片</option><option value="video" ${state.adminAssetKind === "video" ? "selected" : ""}>视频</option><option value="audio" ${state.adminAssetKind === "audio" ? "selected" : ""}>音频</option></select>
                <select class="select" name="status"><option value="">全部状态</option><option value="active" ${state.adminAssetStatus === "active" ? "selected" : ""}>正常</option><option value="blocked" ${state.adminAssetStatus === "blocked" ? "selected" : ""}>已下架</option></select>
                <button class="btn btn-sm" type="submit">查询</button>
            </form>
            <div class="table-wrap"><table><thead><tr><th>素材</th><th>类型</th><th class="num">大小</th><th>所属用户</th><th>治理状态</th><th>创建时间</th><th>操作</th></tr></thead><tbody>${rows || '<tr><td class="empty" colspan="7">没有符合条件的素材</td></tr>'}</tbody></table></div>
            ${adminPager("assets", state.adminAssetPage, state.adminAssetTotal)}
        </div>`;
}

function renderAdminLedger(view) {
    const rows = state.adminLedger.map((item) => {
        const user = item.displayName || item.username || item.userId;
        const model = item.model || "未关联模型";
        const token = ledgerTokenLabel(item);
        const taskDetail = item.taskId
            ? `<button class="btn btn-sm" type="button" data-action="ledger-detail" data-id="${escapeHtml(item.id)}">查看任务</button><div class="cell-sub">${escapeHtml(taskTypeLabel(item.taskType))} · ${escapeHtml(taskStatusLabel(item.taskStatus))}</div>`
            : `<span class="muted">${escapeHtml(item.reason || "无关联任务")}</span>`;
        return `<tr>
            <td><strong>${escapeHtml(user)}</strong><div class="cell-sub">${escapeHtml(item.username || item.userId)}</div></td>
            <td class="muted">${escapeHtml(formatTime(item.createdAt))}</td>
            <td>${escapeHtml(kindLabel(item.kind))}<div class="cell-sub">${escapeHtml(stageLabel(item.stage))}</div></td>
            <td><strong>${escapeHtml(model)}</strong><div class="cell-sub">${escapeHtml(item.channelName || "-")}</div></td>
            <td><span class="tag tag-muted">${escapeHtml(token)}</span></td>
            <td class="num ${item.amount >= 0 ? "amount-pos" : "amount-neg"}">${item.amount >= 0 ? "+" : ""}${escapeHtml(formatMicrocredits(item.amount))}</td>
            <td class="num">${escapeHtml(formatMicrocredits(item.balanceAfter))}</td>
            <td>${taskDetail}</td>
        </tr>`;
    }).join("");
    view.innerHTML = `
        <div class="page-head"><div><h1 class="page-title">积分明细</h1><p class="page-sub">按用户、类型、模型和 Token 能力查看积分增加、扣除、冻结及任务关联。</p></div><button class="btn btn-sm" data-action="refresh">刷新</button></div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel"><h2 class="panel-title">积分变动明细（${formatPoints(state.adminLedgerTotal)}）</h2>
            <form id="ledger-filter-form" class="filter-row">
                <input class="input" name="keyword" placeholder="用户名 / 用户 ID / 任务 / 项目" value="${escapeHtml(state.adminLedgerKeyword)}" />
                <input class="input" name="model" placeholder="模型名称" value="${escapeHtml(state.adminLedgerModel)}" />
                <select class="select" name="kind"><option value="">全部类型</option>${[['recharge', '充值'], ['grant', '赠送'], ['adjust', '人工调整'], ['consume', '任务消费'], ['freeze', '冻结'], ['unfreeze', '退回']].map(([value, label]) => `<option value="${value}" ${state.adminLedgerKind === value ? "selected" : ""}>${label}</option>`).join("")}</select>
                <select class="select" name="token"><option value="">全部 Token</option>${[['text', '文本'], ['image', '图片'], ['video', '视频'], ['audio', '语音'], ['account', '账户']].map(([value, label]) => `<option value="${value}" ${state.adminLedgerToken === value ? "selected" : ""}>${label}</option>`).join("")}</select>
                <input class="input date-input" name="start" type="date" value="${escapeHtml(state.adminLedgerStart)}" aria-label="开始日期" />
                <input class="input date-input" name="end" type="date" value="${escapeHtml(state.adminLedgerEnd)}" aria-label="结束日期" />
                <button class="btn btn-sm" type="submit">查询</button>
            </form>
            <div class="table-wrap"><table><thead><tr><th>用户名</th><th>时间</th><th>类型</th><th>模型</th><th>Token 名称</th><th class="num">积分变动</th><th class="num">变动后余额</th><th>任务详情</th></tr></thead><tbody>${rows || '<tr><td class="empty" colspan="8">没有符合条件的积分明细</td></tr>'}</tbody></table></div>
            ${adminPager("ledger", state.adminLedgerPage, state.adminLedgerTotal)}
        </div>`;
}

function ledgerTokenLabel(item) {
    if (item?.tokenName) return item.tokenName;
    if (!item?.taskId) return "账户";
    const value = `${item.capability || ""} ${item.taskType || ""}`.toLowerCase();
    if (value.includes("video")) return "视频";
    if (value.includes("image")) return "图片";
    if (value.includes("audio") || value.includes("voice") || value.includes("speech") || value.includes("music")) return "语音";
    return "文本";
}

function taskTypeLabel(value) {
    const labels = {
        canvas_text: "文本任务",
        canvas_image: "图片任务",
        canvas_video: "视频任务",
        canvas_audio: "语音任务",
        timeline_transcription: "时间线转写",
        timeline_render: "时间线渲染",
        depth_capture: "深度捕捉",
    };
    return labels[value] || value || "任务";
}

function taskStatusLabel(value) {
    const labels = {
        queued: "等待中",
        running: "生成中",
        succeeded: "成功",
        failed: "失败",
        cancelled: "已取消",
        text_replay: "文本回放",
    };
    return labels[value] || value || "未记录";
}

function openLedgerDetailModal(item) {
    closeLedgerDetailModal();
    if (!item) return;
    const rows = [
        ["用户", item.displayName || item.username || item.userId],
        ["时间", formatTime(item.createdAt)],
        ["积分类型", `${kindLabel(item.kind)} · ${stageLabel(item.stage)}`],
        ["积分变动", `${item.amount >= 0 ? "+" : ""}${formatMicrocredits(item.amount)}`],
        ["变动后余额", formatMicrocredits(item.balanceAfter)],
        ["模型", item.model || "未关联模型"],
        ["渠道", item.channelName || "-"],
        ["Token 名称", ledgerTokenLabel(item)],
        ["项目", item.projectName || "-"],
        ["任务 ID", item.taskId || "无关联任务"],
        ["任务类型", taskTypeLabel(item.taskType)],
        ["任务操作", item.taskOperation || "-"],
        ["任务状态", taskStatusLabel(item.taskStatus)],
        ["上游请求 ID", item.providerRequestId || "-"],
        ["计费状态", item.costStatus || "-"],
        ["实际消耗", item.actualCredits > 0 ? formatMicrocredits(item.actualCredits) : "-"],
        ["原因", item.reason || "-"],
        ["操作人", item.operatorId || "-"],
    ];
    document.body.insertAdjacentHTML("beforeend", `
        <div class="modal-backdrop" id="ledger-detail-modal">
            <section class="modal modal-wide" role="dialog" aria-modal="true" aria-labelledby="ledger-detail-title">
                <div class="modal-head">
                    <div>
                        <h2 class="panel-title" id="ledger-detail-title">积分明细 · 任务详情</h2>
                        <p class="modal-sub">只展示计费与任务追踪信息，不包含密钥、提示词全文或上游请求正文。</p>
                    </div>
                    <button class="btn btn-sm" type="button" data-action="close-ledger-detail">关闭</button>
                </div>
                <div class="modal-body">
                    <div class="table-wrap"><table>
                        <tbody>${rows.map(([label, value]) => `<tr><th>${escapeHtml(label)}</th><td>${escapeHtml(String(value || "-"))}</td></tr>`).join("")}</tbody>
                    </table></div>
                </div>
            </section>
        </div>`);
    document.body.classList.add("modal-open");
}

function closeLedgerDetailModal() {
    document.getElementById("ledger-detail-modal")?.remove();
    if (!document.querySelector(".modal-backdrop:not([hidden])")) document.body.classList.remove("modal-open");
}

function renderAdminSite(view) {
    const cfg = state.siteConfig || { siteName: "VisiDeep AI", creditName: "积分", currency: "CNY", footerText: "VisiDeep AI" };
    view.innerHTML = `
        <div class="page-head"><div><h1 class="page-title">站点配置</h1><p class="page-sub">维护站点品牌、备案、客服入口和积分叫法；公开配置接口会读取这里的值。</p></div><button class="btn btn-sm" data-action="refresh">刷新</button></div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel"><h2 class="panel-title">公开站点信息</h2><form id="site-config-form">
            <div class="form-grid">
                <label class="field"><span class="field-label">站点名称</span><input class="input" name="siteName" value="${escapeHtml(cfg.siteName || "")}" required /></label>
                <label class="field"><span class="field-label">Logo 地址</span><input class="input" name="logoUrl" value="${escapeHtml(cfg.logoUrl || "")}" placeholder="https://..." /></label>
                <label class="field"><span class="field-label">备案号</span><input class="input" name="filingNo" value="${escapeHtml(cfg.filingNo || "")}" /></label>
                <label class="field"><span class="field-label">客服 / 支持链接</span><input class="input" name="supportUrl" value="${escapeHtml(cfg.supportUrl || "")}" placeholder="https://..." /></label>
                <label class="field"><span class="field-label">积分叫法</span><input class="input" name="creditName" value="${escapeHtml(cfg.creditName || "积分")}" required /></label>
                <label class="field"><span class="field-label">货币单位</span><input class="input" name="currency" value="${escapeHtml(cfg.currency || "CNY")}" /></label>
                <label class="field field-wide"><span class="field-label">页脚说明</span><textarea class="textarea" name="footerText" rows="3">${escapeHtml(cfg.footerText || "")}</textarea></label>
                <label class="field field-wide"><span class="field-label">全站公告</span><textarea class="textarea" name="announcement" rows="3" placeholder="留空表示不展示">${escapeHtml(cfg.announcement || "")}</textarea></label>
            </div>
            <div class="btn-row settings-actions"><label class="muted"><input type="checkbox" name="maintenanceMode" ${cfg.maintenanceMode ? "checked" : ""} /> 开启维护模式</label><button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>保存站点配置</button></div>
        </form></div>`;
}

function renderAdminAudit(view) {
    const actions = [['user.create', '创建用户'], ['user.update', '更新用户'], ['user.reset_password', '重置密码'], ['project.archive', '归档项目'], ['project.restore', '恢复项目'], ['canvas.block', '下架画布'], ['canvas.restore', '恢复画布'], ['asset.block', '下架素材'], ['asset.restore', '恢复素材'], ['site.update', '更新站点配置']];
    const rows = state.adminAudit.map((item) => `<tr><td class="muted">${escapeHtml(formatTime(item.createdAt))}</td><td>${escapeHtml(item.displayName || item.username || item.actorUserId)}<div class="cell-sub">${escapeHtml(item.username || item.actorUserId)}</div></td><td>${escapeHtml(actions.find(([value]) => value === item.action)?.[1] || item.action)}<div class="cell-sub">${escapeHtml(item.action)}</div></td><td>${escapeHtml(item.targetType || "-")}<div class="cell-sub">${escapeHtml(item.targetId || "-")}</div></td><td>${escapeHtml(item.summary || "-")}</td></tr>`).join("");
    view.innerHTML = `
        <div class="page-head"><div><h1 class="page-title">审计日志</h1><p class="page-sub">管理员创建账号、改角色、重置密码、治理内容和修改站点配置都会留痕。</p></div><button class="btn btn-sm" data-action="refresh">刷新</button></div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel"><h2 class="panel-title">日志明细（${formatPoints(state.adminAuditTotal)}）</h2>
            <form id="audit-filter-form" class="filter-row">
                <input class="input" name="keyword" placeholder="操作人 / 用户 ID / 对象 ID" value="${escapeHtml(state.adminAuditKeyword)}" />
                <select class="select" name="action"><option value="">全部动作</option>${actions.map(([value, label]) => `<option value="${value}" ${state.adminAuditAction === value ? "selected" : ""}>${label}</option>`).join("")}</select>
                <input class="input date-input" name="start" type="date" value="${escapeHtml(state.adminAuditStart)}" aria-label="开始日期" />
                <input class="input date-input" name="end" type="date" value="${escapeHtml(state.adminAuditEnd)}" aria-label="结束日期" />
                <button class="btn btn-sm" type="submit">查询</button>
            </form>
            <div class="table-wrap"><table><thead><tr><th>时间</th><th>操作人</th><th>动作</th><th>对象</th><th>摘要</th></tr></thead><tbody>${rows || '<tr><td class="empty" colspan="5">没有符合条件的审计日志</td></tr>'}</tbody></table></div>
            ${adminPager("audit", state.adminAuditPage, state.adminAuditTotal)}
        </div>`;
}

function renderAdminUsers(view) {
    view.innerHTML = `
        <div class="page-head">
            <div>
                <h1 class="page-title">积分账户</h1>
                <p class="page-sub">人工加减积分会写入不可变流水，必须填写原因，可追溯到操作人。</p>
            </div>
            <button class="btn btn-sm" data-action="refresh">刷新</button>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel">
            <h2 class="panel-title">调整积分</h2>
            <form id="adjust-form" class="form-grid">
                <label class="field"><span class="field-label">用户 ID</span><input class="input" name="userId" placeholder="从下方列表复制" required /></label>
                <label class="field"><span class="field-label">变动数量（支持小数，负数表示扣减）</span><input class="input" name="amount" type="number" step="0.000001" required /></label>
                <label class="field"><span class="field-label">原因</span><input class="input" name="reason" placeholder="例如：活动奖励 / 纠错" required /></label>
                <div class="field" style="display:flex;align-items:flex-end"><button class="btn btn-primary" type="submit">提交调整</button></div>
            </form>
        </div>
        <div class="panel">
            <h2 class="panel-title">积分账户</h2>
            ${renderAccounts()}
        </div>`;
}

function renderAccounts() {
    if (!state.accounts.length) return `<div class="empty">还没有任何积分账户</div>`;
    return `
        <div class="table-wrap">
            <table>
                <thead><tr><th>用户</th><th>用户 ID</th><th class="num">余额</th><th class="num">冻结</th><th class="num">可用</th><th>状态</th><th></th></tr></thead>
                <tbody>
                    ${state.accounts
                        .map(
                            (account) => `<tr>
                                <td>${escapeHtml(account.displayName || account.username || "-")}</td>
                                <td class="muted">${escapeHtml(account.userId)}</td>
                                <td class="num">${escapeHtml(formatMicrocredits(account.balance))}</td>
                                <td class="num">${escapeHtml(formatMicrocredits(account.frozen))}</td>
                                <td class="num">${escapeHtml(formatMicrocredits(account.available))}</td>
                                <td>${escapeHtml(account.status || "-")}</td>
                                <td><button class="btn btn-sm" data-action="use-user" data-user="${escapeHtml(account.userId)}">填入</button></td>
                            </tr>`,
                        )
                        .join("")}
                </tbody>
            </table>
        </div>`;
}

function renderAdminStorage(view) {
    view.innerHTML = `
        <div class="page-head">
            <div>
                <h1 class="page-title">对象存储</h1>
                <p class="page-sub">新资源写入默认存储；历史资源按各自记录的位置读取。密钥加密落库，列表只回显掩码。</p>
            </div>
            <button class="btn btn-sm" data-action="refresh">刷新</button>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel">
            <h2 class="panel-title">添加存储配置</h2>
            <form id="storage-form">
                <div class="form-grid">
                    <label class="field"><span class="field-label">配置名称</span><input class="input" name="name" required /></label>
                    <label class="field"><span class="field-label">类型</span>
                        <select class="select" name="provider">
                            <option value="s3">S3 兼容</option>
                            <option value="oss">阿里云 OSS</option>
                            <option value="cos">腾讯云 COS</option>
                            <option value="qiniu">七牛</option>
                            <option value="minio">MinIO</option>
                        </select>
                    </label>
                    <label class="field"><span class="field-label">Endpoint</span><input class="input" name="endpoint" placeholder="https://s3.example.com" required /></label>
                    <label class="field"><span class="field-label">Region</span><input class="input" name="region" /></label>
                    <label class="field"><span class="field-label">Bucket</span><input class="input" name="bucket" required /></label>
                    <label class="field"><span class="field-label">AccessKey ID</span><input class="input" name="accessKeyId" required /></label>
                    <label class="field"><span class="field-label">AccessKey Secret</span><input class="input" name="secretAccessKey" type="password" required /></label>
                    <label class="field"><span class="field-label">CDN / 自定义域名（可选）</span><input class="input" name="cdnBaseUrl" /></label>
                    <label class="field"><span class="field-label">路径前缀（可选）</span><input class="input" name="prefix" /></label>
                </div>
                <div class="btn-row">
                    <label class="muted"><input type="checkbox" name="pathStyle" /> 使用 path-style 寻址</label>
                    <button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>保存配置</button>
                </div>
            </form>
        </div>
        <div class="panel">
            <h2 class="panel-title">已有配置</h2>
            ${renderStorage()}
        </div>`;
}

function renderStorage() {
    if (!state.storage.length) return `<div class="empty">还没有存储配置，请先添加一条并测试连通性</div>`;
    return `
        <div class="table-wrap">
            <table>
                <thead><tr><th>名称</th><th>类型</th><th>Bucket</th><th>Endpoint</th><th>密钥</th><th>连通性</th><th>默认</th><th></th></tr></thead>
                <tbody>
                    ${state.storage
                        .map((item) => {
                            const tested = item.lastTestError
                                ? `<span class="tag tag-warn" title="${escapeHtml(item.lastTestError)}">失败</span>`
                                : item.lastTestedAt
                                  ? `<span class="tag tag-ok">通过</span>`
                                  : `<span class="tag tag-muted">未测试</span>`;
                            return `<tr>
                                <td>${escapeHtml(item.name)}</td>
                                <td class="muted">${escapeHtml(item.provider)}</td>
                                <td>${escapeHtml(item.bucket)}</td>
                                <td class="muted">${escapeHtml(item.endpoint)}</td>
                                <td class="muted">${escapeHtml(item.secretAccessKeyMasked || "-")}</td>
                                <td>${tested}</td>
                                <td>${item.isDefault ? '<span class="tag tag-ok">默认</span>' : item.enabled ? '<span class="tag tag-muted">启用</span>' : '<span class="tag tag-muted">停用</span>'}</td>
                                <td>
                                    <div class="btn-row">
                                        <button class="btn btn-sm" data-action="test-storage" data-id="${escapeHtml(item.id)}">测试</button>
                                        <button class="btn btn-sm" data-action="default-storage" data-id="${escapeHtml(item.id)}">设为默认</button>
                                        <button class="btn btn-sm btn-danger" data-action="delete-storage" data-id="${escapeHtml(item.id)}">删除</button>
                                    </div>
                                </td>
                            </tr>`;
                        })
                        .join("")}
                </tbody>
            </table>
        </div>`;
}

/* ---------------- 交互 ---------------- */

const CREATION_SCENES = [['short_drama', '短剧'], ['ecommerce', '电商'], ['general', '通用']];

function creationCatalogKind(value) {
    return value === "prompt" || value === "style" ? value : "workflow";
}

function creationCatalogCopy(kind) {
    if (kind === "prompt") return { title: "提示词库", sub: "运营改提示词，用户端按分组直接使用，不需要重新发版。" };
    if (kind === "style") return { title: "画风预设", sub: "画风卡片与提示词前缀在这里维护，用户点选后自动套用。" };
    return { title: "工作流模板", sub: "短剧链式、电商出图等固定流程在这里维护，用户端选模板即可开始。" };
}

function creationSceneLabel(value) {
    const found = CREATION_SCENES.find(([key]) => key === value);
    return found ? found[1] : value || "通用";
}

function creationCanvasTypeLabel(value) {
    return value === "batch" ? "批量创作表" : "节点画布";
}

function renderCreationTable(kind) {
    if (!state.creationItems.length) return `<div class="empty">还没有配置，先在下方表单新增一条。</div>`;
    const rows = state.creationItems.map((item) => {
        let cells = "";
        if (kind === "prompt") {
            cells = `<td>${escapeHtml(item.group || "-")}</td><td>${escapeHtml(item.name)}</td><td>${escapeHtml(creationSceneLabel(item.scene))}</td><td class="muted">${escapeHtml(item.variables || "-")}</td><td class="muted">${escapeHtml(formatTime(item.updatedAt))}</td>`;
        } else if (kind === "style") {
            cells = `<td>${escapeHtml(item.name)}</td><td>${escapeHtml(creationSceneLabel(item.scene))}</td><td class="muted">${escapeHtml((item.content || "").slice(0, 60))}</td><td>${item.coverUrl ? `<a class="link" href="${escapeHtml(item.coverUrl)}" target="_blank" rel="noreferrer">查看封面</a>` : "-"}</td>`;
        } else {
            cells = `<td>${escapeHtml(item.name)}</td><td class="muted">${escapeHtml(item.itemKey || "-")}</td><td>${escapeHtml(creationSceneLabel(item.scene))}</td><td>${escapeHtml(creationCanvasTypeLabel(item.canvasType))}</td><td class="muted">${escapeHtml(item.defaultModel || "-")}</td>`;
        }
        return `<tr>${cells}<td>${item.enabled ? '<span class="tag tag-ok">启用</span>' : '<span class="tag tag-muted">停用</span>'}</td><td><div class="btn-row"><button class="btn btn-sm" data-action="edit-creation" data-id="${escapeHtml(item.id)}">编辑</button><button class="btn btn-sm btn-danger" data-action="delete-creation" data-id="${escapeHtml(item.id)}">删除</button></div></td></tr>`;
    }).join("");
    const headers = kind === "prompt" ? ["分组", "名称", "场景", "变量", "更新时间", "状态", "操作"] : kind === "style" ? ["画风", "场景", "提示词前缀", "封面", "状态", "操作"] : ["模板名称", "标识", "场景", "画布类型", "默认模型", "状态", "操作"];
    return `<div class="table-wrap"><table><thead><tr>${headers.map((header) => `<th>${escapeHtml(header)}</th>`).join("")}</tr></thead><tbody>${rows}</tbody></table></div>`;
}

function creationFormFields(kind) {
    const scenes = CREATION_SCENES.map(([value, label]) => `<option value="${value}">${label}</option>`).join("");
    const common = `
        <label class="field"><span class="field-label">名称</span><input class="input" name="name" required /></label>
        <label class="field"><span class="field-label">适用场景</span><select class="select" name="scene">${scenes}</select></label>
        <label class="field"><span class="field-label">标识（留空自动生成）</span><input class="input" name="itemKey" /></label>
        <label class="field"><span class="field-label">排序（小的在前）</span><input class="input" name="sortOrder" type="number" step="1" value="0" /></label>`;
    if (kind === "prompt") return `${common}
        <label class="field"><span class="field-label">分组</span><input class="input" name="group" placeholder="画布预设 / 生成模板 / 节点默认" /></label>
        <label class="field" style="grid-column:1/-1"><span class="field-label">提示词正文</span><textarea class="textarea" name="content" rows="5" required placeholder="支持 {{变量}} 占位"></textarea></label>
        <label class="field" style="grid-column:1/-1"><span class="field-label">变量说明</span><input class="input" name="variables" placeholder="逗号分隔，例如：主体, 场景, 光线" /></label>`;
    if (kind === "style") return `${common}
        <label class="field" style="grid-column:1/-1"><span class="field-label">提示词前缀</span><textarea class="textarea" name="content" rows="4" required placeholder="选这个画风时自动加在提示词前面"></textarea></label>
        <label class="field" style="grid-column:1/-1"><span class="field-label">封面图地址</span><input class="input" name="coverUrl" placeholder="https://..." /></label>`;
    return `${common}
        <label class="field"><span class="field-label">画布类型</span><select class="select" name="canvasType"><option value="node">节点画布</option><option value="batch">批量创作表</option></select></label>
        <label class="field"><span class="field-label">默认模型（可选）</span><input class="input" name="defaultModel" placeholder="选择上架的模型标识" /></label>
        <label class="field" style="grid-column:1/-1"><span class="field-label">模板说明</span><textarea class="textarea" name="description" rows="3" placeholder="给用户看的一句话说明"></textarea></label>
        <label class="field" style="grid-column:1/-1"><span class="field-label">流程定义 JSON（可选）</span><textarea class="textarea" name="payloadJson" rows="5" placeholder='{"nodes":[],"edges":[]}'></textarea></label>`;
}

function renderAdminCreationCatalog(view, kind) {
    state.creationKind = kind;
    const copy = creationCatalogCopy(kind);
    view.innerHTML = `
        <div class="page-head"><div><h1 class="page-title">${escapeHtml(copy.title)}</h1><p class="page-sub">${escapeHtml(copy.sub)}</p></div><div class="btn-row"><button class="btn btn-sm" data-action="new-creation" data-kind="${kind}">新增</button><button class="btn btn-sm" data-action="refresh">刷新</button></div></div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel"><h2 class="panel-title">已配置（${state.creationTotal}）</h2><form id="creation-filter-form" class="filter-row"><input type="hidden" name="kind" value="${kind}" /><input class="input" name="keyword" placeholder="名称 / 标识 / 说明" value="${escapeHtml(state.creationKeyword)}" /><select class="select" name="status"><option value="">全部状态</option><option value="enabled" ${state.creationStatus === "enabled" ? "selected" : ""}>启用</option><option value="disabled" ${state.creationStatus === "disabled" ? "selected" : ""}>停用</option></select><button class="btn btn-sm" type="submit">查询</button></form>${renderCreationTable(kind)}</div>
        <div class="panel"><h2 class="panel-title" id="creation-form-title">新增${escapeHtml(copy.title)}</h2><form id="creation-form"><input type="hidden" name="id" /><input type="hidden" name="kind" value="${kind}" /><div class="form-grid">${creationFormFields(kind)}</div><div class="btn-row"><label class="muted"><input type="checkbox" name="enabled" checked /> 启用</label><button class="btn btn-sm" type="button" data-action="reset-creation">清空表单</button><button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>保存</button></div></form></div>`;
}

function fillCreationForm(item) {
    const form = document.getElementById("creation-form");
    if (!form) return;
    const setValue = (name, value) => {
        const control = form.elements.namedItem(name);
        if (control) control.value = value;
    };
    setValue("id", item ? item.id : "");
    setValue("name", item ? item.name || "" : "");
    setValue("itemKey", item ? item.itemKey || "" : "");
    setValue("scene", item ? item.scene || "general" : "general");
    setValue("sortOrder", item ? item.sortOrder || 0 : 0);
    setValue("group", item ? item.group || "" : "");
    setValue("content", item ? item.content || "" : "");
    setValue("variables", item ? item.variables || "" : "");
    setValue("canvasType", item ? item.canvasType || "node" : "node");
    setValue("defaultModel", item ? item.defaultModel || "" : "");
    setValue("description", item ? item.description || "" : "");
    setValue("coverUrl", item ? item.coverUrl || "" : "");
    const enabled = form.elements.namedItem("enabled");
    if (enabled) enabled.checked = item ? item.enabled !== false : true;
    const payloadControl = form.elements.namedItem("payloadJson");
    if (payloadControl) {
        let payload = "";
        try { payload = item && item.payloadJson ? JSON.stringify(JSON.parse(item.payloadJson), null, 2) : ""; } catch { payload = item?.payloadJson || ""; }
        payloadControl.value = payload === "null" ? "" : payload;
    }
    const title = document.getElementById("creation-form-title");
    if (title) title.textContent = item ? `编辑：${item.name || item.id}` : title.textContent.replace(/^编辑：/u, "新增");
}

/* ---------------- 模型渠道 ---------------- */

const CHANNEL_CAPABILITIES = [['text', '文本'], ['image', '图片'], ['video', '视频'], ['audio', '音频']];

// 协议是上游调用形状，决定请求怎么发。列表顺序按能力分组，方便选。
const CHANNEL_PROTOCOLS = [
    ['chat-completion', 'chat-completion（文本）'],
    ['openai-response', 'openai-response（文本）'],
    ['claude-api', 'claude-api（文本）'],
    ['openai-image', 'openai-image（图片）'],
    ['toapis-image', 'toapis-image（图片）'],
    ['grok-image', 'grok-image（图片）'],
    ['volcengine-ark-image', 'volcengine-ark-image（图片）'],
    ['volcengine-ark-agent-plan-image', 'volcengine-ark-agent-plan-image（图片）'],
    ['volcengine-jimeng-image', 'volcengine-jimeng-image（图片）'],
    ['gemini-image', 'gemini-image（图片）'],
    ['openai-audio', 'openai-audio（音频）'],
    ['async-audio', 'async-audio（音频）'],
    ['newapi', 'newapi（视频）'],
    ['newapi-channel-1', 'newapi-channel-1（视频）'],
    ['newapi-channel-2', 'newapi-channel-2（视频）'],
    ['toapis-video', 'toapis-video（视频）'],
    ['xai-video', 'xai-video（视频）'],
    ['volcengine-ark-video', 'volcengine-ark-video（视频）'],
    ['volcengine-ark-agent-plan-video', 'volcengine-ark-agent-plan-video（视频）'],
    ['volcengine-jimeng-video', 'volcengine-jimeng-video（视频）'],
    ['gemini-veo', 'gemini-veo（视频）'],
    ['novita-video', 'novita-video（视频）'],
    ['minimax-video', 'minimax-video（视频）'],
    ['agnes-video', 'agnes-video（视频）'],
    ['runninghub-workflow-image', 'runninghub-workflow-image（图片）'],
    ['runninghub-workflow-video', 'runninghub-workflow-video（视频）'],
    ['runninghub-workflow-audio', 'runninghub-workflow-audio（音频）'],
];

const IMAGE_RATIO_OPTIONS = ['1:1', '3:2', '2:3', '4:3', '3:4', '16:9', '9:16', '21:9'];
const VIDEO_RATIO_OPTIONS = ['1:1', '4:3', '3:4', '16:9', '9:16', '21:9', 'adaptive'];
const IMAGE_RESOLUTION_OPTIONS = ['0.5k', '1k', '2k', '3k', '4k'];
const VIDEO_RESOLUTION_OPTIONS = ['480p', '720p', '1080p', '2k', '4k'];

function pricingResolutionOptions(capability = "") {
    if (capability === "image") return IMAGE_RESOLUTION_OPTIONS;
    if (capability === "video") return VIDEO_RESOLUTION_OPTIONS;
    return ["1k", "2k", "4k"];
}

function defaultChannelModelCapabilityConfig(capability, protocol, modelKey) {
    if (capability === 'image') {
        const seedream5 = /^doubao-seedream-5-0(?:-|$)/i.test(String(modelKey || ''));
        return {
            version: 1,
            image: {
                size: {
                    parameter: 'aspect_ratio',
                    values: IMAGE_RATIO_OPTIONS.filter((value) => value !== '21:9' || protocol === 'toapis-image' || protocol === 'openai-image'),
                    default: '1:1',
                    allowCustom: false,
                },
                quality: {
                    supported: true,
                    values: seedream5 ? ['2k', '3k'] : ['0.5k', '1k', '2k', '4k'],
                    default: seedream5 ? '2k' : '1k',
                },
                references: {
                    promptMaxChars: 32000,
                    maxImages: seedream5 ? 10 : 14,
                    maxImageBytes: 10 * 1024 * 1024,
                    maskSupported: false,
                },
                transparentBackground: { supported: false, default: false },
                responseFormat: { supported: false },
                outputFormat: { supported: false },
                maxOutputs: seedream5 ? 15 : 4,
            },
        };
    }
    if (capability === 'video') {
        return {
            version: 1,
            video: {
                references: {
                    promptMaxChars: 8000,
                    minImages: 0,
                    maxImages: 9,
                    maxImageBytes: 30 * 1024 * 1024,
                    maxVideos: 3,
                    maxVideoBytes: 50 * 1024 * 1024,
                    maxVideoDurationSeconds: 15,
                    minVideoDurationSeconds: 2,
                    maxAudios: 3,
                    maxAudioBytes: 15 * 1024 * 1024,
                    maxAudioDurationSeconds: 15,
                    minAudioDurationSeconds: 2,
                },
                duration: { selection: 'range', min: 4, max: 15, step: 1, default: 5 },
                ratios: VIDEO_RATIO_OPTIONS,
                defaultRatio: '16:9',
                resolutions: protocol === 'toapis-video' ? VIDEO_RESOLUTION_OPTIONS : ['480p', '720p', '1080p'],
                defaultResolution: '720p',
                generateAudio: { supported: true, default: true },
                watermark: { supported: true, default: false },
                operations: ['text_to_video', 'image_to_video', 'reference_to_video'],
                defaultOperation: 'text_to_video',
            },
        };
    }
    return undefined;
}

function capabilityLabel(value) {
    const found = CHANNEL_CAPABILITIES.find(([key]) => key === value);
    return found ? found[1] : value || '-';
}

function selectedChannel() {
    return state.channels.find((item) => item.id === state.selectedChannelId) || null;
}


function activeChannelTierRulesForModel(channelId, modelId) {
    const model = state.channelModels.find((item) => item.id === modelId)
        || state.pricingChannelModels.find((item) => item.id === modelId);
    const now = Date.now();
    return channelTierRulesForModel(channelId, modelId).filter((rule) => {
        if (rule.enabled === false) return false;
        if (model?.capability && rule.capability !== model.capability) return false;
        const compatibleModes = model?.capability === "image"
            ? ["fixed_request", "per_image"]
            : model?.capability === "video" || model?.capability === "audio"
              ? ["fixed_request", "per_second"]
              : ["fixed_request"];
        if (!compatibleModes.includes(rule.billingMode)) return false;
        const startsAt = rule.effectiveFrom ? Date.parse(rule.effectiveFrom) : 0;
        const endsAt = rule.effectiveTo ? Date.parse(rule.effectiveTo) : 0;
        return (!startsAt || startsAt <= now) && (!endsAt || endsAt > now);
    });
}

function channelModelCanvasVisible(item) {
    return Boolean(selectedChannel()?.enabled && item.enabled);
}

function renderChannelModelSummary(channel) {
    const total = state.channelModels.length;
    const enabled = state.channelModels.filter((item) => item.enabled).length;
    const visible = state.channelModels.filter(channelModelCanvasVisible).length;
    const priced = state.channelModels.filter((item) => activeChannelTierRulesForModel(channel.id, item.id).length).length;
    return `<div class="metric-row model-summary">
        <div class="metric"><div class="metric-label">渠道模型</div><div class="metric-value">${total}</div></div>
        <div class="metric"><div class="metric-label">模型启用</div><div class="metric-value">${enabled}</div></div>
        <div class="metric"><div class="metric-label">画布可见</div><div class="metric-value">${visible}</div></div>
        <div class="metric"><div class="metric-label">已设有效价</div><div class="metric-value">${priced}</div></div>
    </div>`;
}



function renderAdminChannels(view) {
    const channel = selectedChannel();
    const channelOptions = state.channels.map((item) => `<option value="${escapeHtml(item.id)}" ${item.id === state.selectedChannelId ? "selected" : ""}>${escapeHtml(item.name || item.id)}</option>`).join("");
    view.innerHTML = `
        <div class="page-head">
            <div>
                <h1 class="page-title">模型渠道</h1>
                <p class="page-sub">管理上游渠道和画布可用模型。筛选、上下架和定价都在同一工作区完成。</p>
            </div>
            <div class="btn-row">
                <button class="btn btn-sm btn-primary" data-action="new-channel">新增渠道</button>
                <button class="btn btn-sm" data-action="refresh" ${state.busy ? "disabled" : ""}>刷新</button>
            </div>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="channel-workspace" id="channel-workspace">
            <aside class="panel channel-sidebar">
                <div class="channel-sidebar-head">
                    <div><h2 class="panel-title">渠道</h2><p class="page-sub">共 ${state.channelTotal} 个</p></div>
                    <button class="btn btn-sm" data-action="new-channel">新增</button>
                </div>
                <form id="channel-filter-form" class="channel-filter-form">
                    <input class="input" name="keyword" placeholder="搜索渠道 / 别名 / Base URL" value="${escapeHtml(state.channelKeyword)}" />
                    <div class="channel-filter-row">
                        <select class="select" name="status">
                            <option value="">全部状态</option>
                            <option value="enabled" ${state.channelStatus === "enabled" ? "selected" : ""}>启用</option>
                            <option value="disabled" ${state.channelStatus === "disabled" ? "selected" : ""}>停用</option>
                        </select>
                        <button class="btn btn-sm" type="submit">查询</button>
                    </div>
                </form>
                <label class="field channel-mobile-picker"><span class="field-label">当前渠道</span><select class="select" id="channel-mobile-select" ${state.channels.length ? "" : "disabled"}>${channelOptions || '<option value="">暂无渠道</option>'}</select></label>
                ${renderChannelList()}
            </aside>
            <section class="panel channel-main">
                ${channel ? renderChannelModelsPanel(channel) : '<div class="empty">还没有渠道。先新增渠道，再读取上游模型。</div>'}
            </section>
        </div>
        ${renderChannelFormModal()}`;
}

function renderChannelModelsPanel(channel) {
    return `
        <div class="channel-main-head">
            <div class="channel-main-title">
                <div class="btn-row">
                    <h2 class="panel-title">${escapeHtml(channel.name || channel.id)}</h2>
                    ${channel.enabled ? '<span class="tag tag-ok">渠道启用</span>' : '<span class="tag tag-muted">渠道停用</span>'}${channel.hasApiKey ? '<span class="tag tag-ok">密钥已配置</span>' : '<span class="tag tag-warn">缺少密钥</span>'}
                </div>
                <p class="page-sub">${escapeHtml(channel.publicAlias || "未设置对外别名")} · ${escapeHtml(channel.baseUrl || "未设置 Base URL")}</p>
            </div>
            <div class="btn-row">
                <button class="btn btn-sm" data-action="edit-channel" data-id="${escapeHtml(channel.id)}">编辑渠道</button>
                <button class="btn btn-sm" data-action="duplicate-channel" data-id="${escapeHtml(channel.id)}">复制</button>
                <button class="btn btn-sm btn-danger" data-action="delete-channel" data-id="${escapeHtml(channel.id)}">删除</button>
            </div>
        </div>
        ${renderChannelModelSummary(channel)}
        <div class="channel-action-bar">
            <div class="btn-row">
                <button class="btn btn-sm btn-primary" data-action="new-channel-model" ${state.busy ? "disabled" : ""}>新增模型</button>
                <button class="btn btn-sm" data-action="fetch-channel-models" data-id="${escapeHtml(channel.id)}" ${state.busy ? "disabled" : ""}>从上游拉取目录</button>
                <button class="btn btn-sm" data-action="preview-channel-catalog" data-id="${escapeHtml(channel.id)}" ${state.busy ? "disabled" : ""}>读取上游模型/报价</button>
            </div>
            <span class="muted">画布可见 = 渠道启用 + 模型启用；提交生成还需要有效价格。</span>
        </div>
        <form id="channel-model-filter-form" class="channel-model-filter-form">
            <input class="input" name="modelKeyword" placeholder="搜索模型名称 / 标识 / 上游 ID" value="${escapeHtml(state.channelModelKeyword)}" />
            <select class="select" name="modelCapability">
                <option value="">全部能力</option>
                ${CHANNEL_CAPABILITIES.map(([value, label]) => `<option value="${value}" ${state.channelModelCapability === value ? "selected" : ""}>${label}</option>`).join("")}
            </select>
            <select class="select" name="modelCanvasStatus">
                <option value="">全部状态</option>
                <option value="visible" ${state.channelModelCanvasStatus === "visible" ? "selected" : ""}>画布可见</option>
                <option value="hidden" ${state.channelModelCanvasStatus === "hidden" ? "selected" : ""}>画布隐藏</option>
                <option value="unpriced" ${state.channelModelCanvasStatus === "unpriced" ? "selected" : ""}>未设价</option>
            </select>
            <button class="btn btn-sm" type="submit">筛选</button>
        </form>
        <div id="channel-model-results">${renderChannelModelResults()}</div>
        <div class="modal-backdrop" id="channel-model-modal" hidden>
            <section class="modal channel-model-modal" role="dialog" aria-modal="true" aria-labelledby="channel-model-form-title">
                <div class="modal-head">
                    <div>
                        <h2 class="panel-title" id="channel-model-form-title">新增渠道模型</h2>
                        <p class="modal-sub">修改模型展示、上游映射和启用状态。</p>
                    </div>
                    <button class="btn btn-sm" type="button" data-action="close-channel-model">关闭</button>
                </div>
                <form id="channel-model-form">
                    <input type="hidden" name="id" />
                    <div class="modal-body">
                        <div class="form-grid">
                            <label class="field"><span class="field-label">模型标识</span><input class="input" name="modelKey" required placeholder="例如：seedance-2-5-480p" /></label>
                            <label class="field"><span class="field-label">上游模型 ID（留空跟随标识）</span><input class="input" name="providerModelKey" placeholder="例如：doubao-seedance-2-5" /></label>
                            <label class="field"><span class="field-label">展示名</span><input class="input" name="displayName" placeholder="用户端看到的模型名" /></label>
                            <label class="field"><span class="field-label">能力</span><select class="select" name="capability">${CHANNEL_CAPABILITIES.map(([value, label]) => `<option value="${value}">${label}</option>`).join("")}</select></label>
                            <label class="field"><span class="field-label">协议</span><select class="select" name="protocol">${CHANNEL_PROTOCOLS.map(([value, label]) => `<option value="${value}">${label}</option>`).join("")}</select></label>
                        </div>
                        <div id="channel-model-image-params" hidden>
                            <div class="form-grid">
                                <div class="field" style="grid-column:1/-1">
                                    <span class="field-label">支持的画面比例</span>
                                    <div class="btn-row">${IMAGE_RATIO_OPTIONS.map((value) => `<label class="muted"><input type="checkbox" name="imageRatios" value="${value}" /> ${value}</label>`).join("")}</div>
                                </div>
                                <div class="field" style="grid-column:1/-1">
                                    <span class="field-label">支持的分辨率</span>
                                    <div class="btn-row">${IMAGE_RESOLUTION_OPTIONS.map((value) => `<label class="muted"><input type="checkbox" name="imageResolutions" value="${value}" /> ${value}</label>`).join("")}</div>
                                </div>
                                <label class="field"><span class="field-label">默认比例</span><select class="select" name="imageDefaultRatio">${IMAGE_RATIO_OPTIONS.map((value) => `<option value="${value}">${value}</option>`).join("")}</select></label>
                                <label class="field"><span class="field-label">默认分辨率</span><select class="select" name="imageDefaultResolution">${IMAGE_RESOLUTION_OPTIONS.map((value) => `<option value="${value}">${value}</option>`).join("")}</select></label>
                            </div>
                        </div>
                        <div id="channel-model-video-params" hidden>
                            <div class="form-grid">
                                <div class="field" style="grid-column:1/-1">
                                    <span class="field-label">支持的画面比例</span>
                                    <div class="btn-row">${VIDEO_RATIO_OPTIONS.map((value) => `<label class="muted"><input type="checkbox" name="videoRatios" value="${value}" /> ${value}</label>`).join("")}</div>
                                </div>
                                <div class="field" style="grid-column:1/-1">
                                    <span class="field-label">支持的分辨率</span>
                                    <div class="btn-row">${VIDEO_RESOLUTION_OPTIONS.map((value) => `<label class="muted"><input type="checkbox" name="videoResolutions" value="${value}" /> ${value}</label>`).join("")}</div>
                                </div>
                                <label class="field"><span class="field-label">默认比例</span><select class="select" name="videoDefaultRatio">${VIDEO_RATIO_OPTIONS.map((value) => `<option value="${value}">${value}</option>`).join("")}</select></label>
                                <label class="field"><span class="field-label">默认分辨率</span><select class="select" name="videoDefaultResolution">${VIDEO_RESOLUTION_OPTIONS.map((value) => `<option value="${value}">${value}</option>`).join("")}</select></label>
                                <label class="field"><span class="field-label">最短秒数</span><input class="input" name="videoMinSeconds" type="number" min="1" max="60" step="1" value="4" /></label>
                                <label class="field"><span class="field-label">最长秒数</span><input class="input" name="videoMaxSeconds" type="number" min="1" max="60" step="1" value="15" /></label>
                                <label class="field"><span class="field-label">默认秒数</span><input class="input" name="videoDefaultSeconds" type="number" min="1" max="60" step="1" value="5" /></label>
                                <div class="field"><span class="field-label">生成选项</span><div class="btn-row"><label class="muted"><input type="checkbox" name="videoGenerateAudio" /> 支持生成音频</label><label class="muted"><input type="checkbox" name="videoWatermark" /> 支持水印开关</label></div></div>
                            </div>
                        </div>
                    </div>
                    <div class="modal-foot">
                        <label class="muted"><input type="checkbox" name="enabled" checked /> 启用</label>
                        <div class="btn-row">
                            <button class="btn btn-sm" type="button" data-action="reset-channel-model">清空表单</button>
                            <button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>保存渠道模型</button>
                        </div>
                    </div>
                </form>
            </section>
        </div>`;
}

function renderChannelFormModal() {
    return `
        <div class="modal-backdrop" id="channel-form-modal" hidden>
            <section class="modal channel-modal" role="dialog" aria-modal="true" aria-labelledby="channel-form-title">
                <div class="modal-head">
                    <div>
                        <h2 class="panel-title" id="channel-form-title">新增渠道</h2>
                        <p class="modal-sub">密钥只加密保存，页面不会回显明文。</p>
                    </div>
                    <button class="btn btn-sm" type="button" data-action="close-channel-form">关闭</button>
                </div>
                <form id="channel-form">
                    <input type="hidden" name="id" />
                    <div class="modal-body">
                        <div class="form-grid">
                            <label class="field"><span class="field-label">渠道名称</span><input class="input" name="name" required placeholder="例如：方舟视频主链路" /></label>
                            <label class="field"><span class="field-label">对外别名（可选）</span><input class="input" name="publicAlias" placeholder="用户端看到的名字" /></label>
                            <label class="field"><span class="field-label">Base URL</span><input class="input" name="baseUrl" required placeholder="https://ark.cn-beijing.volces.com/api/v3" /></label>
                            <label class="field"><span class="field-label">API Key（留空表示不修改）</span><input class="input" name="apiKey" type="password" autocomplete="new-password" placeholder="保存后只显示是否已配置" /></label>
                            <label class="field"><span class="field-label">Secret Key / 密钥（留空表示不修改）</span><input class="input" name="secretKey" type="password" autocomplete="new-password" /></label>
                            <label class="field"><span class="field-label">最大并发数（1-999）</span><input class="input" name="concurrencyLimit" type="number" min="1" max="999" step="1" value="6" /></label>
                            <label class="field"><span class="field-label">排序（小的在前）</span><input class="input" name="sortOrder" type="number" step="1" value="0" /></label>
                            <label class="field" style="grid-column:1/-1"><span class="field-label">自定义请求头 JSON（可选）</span><textarea class="textarea" name="headers" rows="2" placeholder="[{\"name\":\"X-Tenant\",\"value\":\"tenant-a\"}]"></textarea></label>
                        </div>
                    </div>
                    <div class="modal-foot">
                        <div class="btn-row">
                            <label class="muted"><input type="checkbox" name="useGlobalConcurrency" /> 使用全局并发</label>
                            <label class="muted"><input type="checkbox" name="enabled" checked /> 启用</label>
                        </div>
                        <div class="btn-row">
                            <button class="btn btn-sm" type="button" data-action="reset-channel">清空表单</button>
                            <button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>保存渠道</button>
                        </div>
                    </div>
                </form>
            </section>
        </div>`;
}

function fillChannelForm(item) {
    const form = document.getElementById("channel-form");
    if (!form) return;
    form.id.value = item ? item.id : "";
    form.name.value = item ? item.name || "" : "";
    form.publicAlias.value = item ? item.publicAlias || "" : "";
    form.baseUrl.value = item ? item.baseUrl || "" : "";
    form.apiKey.value = "";
    form.secretKey.value = "";
    form.concurrencyLimit.value = item ? item.concurrencyLimit || 6 : 6;
    form.useGlobalConcurrency.checked = item ? Number(item.concurrencyLimit || 0) === 0 : false;
    form.sortOrder.value = item ? item.sortOrder || 0 : 0;
    form.headers.value = item && item.headers && item.headers.length ? JSON.stringify(item.headers) : "";
    form.enabled.checked = item ? item.enabled !== false : true;
    const title = document.getElementById("channel-form-title");
    if (title) title.textContent = item ? `编辑渠道：${item.name || item.id}` : "新增渠道";
}

function openChannelFormModal(item = null) {
    fillChannelForm(item);
    const modal = document.getElementById("channel-form-modal");
    if (!modal) return;
    modal.hidden = false;
    document.body.classList.add("modal-open");
    requestAnimationFrame(() => document.querySelector('#channel-form [name="name"]')?.focus());
}

function closeChannelFormModal() {
    const modal = document.getElementById("channel-form-modal");
    if (modal) modal.hidden = true;
    if (!document.querySelector(".modal-backdrop:not([hidden])")) document.body.classList.remove("modal-open");
}

function setNamedCheckboxes(form, name, values) {
    const selected = new Set((values || []).map(String));
    form.querySelectorAll(`input[name="${name}"]`).forEach((input) => {
        input.checked = selected.has(input.value);
    });
}

function checkedNamedValues(form, name) {
    return Array.from(form.querySelectorAll(`input[name="${name}"]:checked`)).map((input) => input.value);
}

function syncChannelModelParameterVisibility(form) {
    if (!form) return;
    const capability = form.capability.value;
    const imageSection = document.getElementById("channel-model-image-params");
    const videoSection = document.getElementById("channel-model-video-params");
    if (imageSection) imageSection.hidden = capability !== "image";
    if (videoSection) videoSection.hidden = capability !== "video";
    if (capability === "image" && !checkedNamedValues(form, "imageRatios").length) {
        setNamedCheckboxes(form, "imageRatios", ["1:1", "4:3", "3:4", "16:9", "9:16", "21:9"]);
    }
    if (capability === "image" && !checkedNamedValues(form, "imageResolutions").length) {
        setNamedCheckboxes(form, "imageResolutions", ["1k", "2k", "4k"]);
    }
    if (capability === "video" && !checkedNamedValues(form, "videoRatios").length) {
        setNamedCheckboxes(form, "videoRatios", VIDEO_RATIO_OPTIONS);
    }
    if (capability === "video" && !checkedNamedValues(form, "videoResolutions").length) {
        setNamedCheckboxes(form, "videoResolutions", ["480p", "720p", "1080p", "4k"]);
    }
}

function channelModelCapabilityConfigFromForm(form, existing) {
    const capability = form.capability.value;
    const modelKey = form.modelKey.value.trim();
    const protocol = form.protocol.value;
    if (capability === "image") {
        const defaults = defaultChannelModelCapabilityConfig(capability, protocol, modelKey);
        const current = existing?.image || defaults.image;
        return {
            version: 1,
            image: {
                ...current,
                size: {
                    ...defaults.image.size,
                    ...current.size,
                    values: checkedNamedValues(form, "imageRatios"),
                    default: form.imageDefaultRatio.value,
                },
                quality: {
                    ...defaults.image.quality,
                    ...current.quality,
                    values: checkedNamedValues(form, "imageResolutions"),
                    default: form.imageDefaultResolution.value,
                },
                references: current.references || defaults.image.references,
                transparentBackground: current.transparentBackground || defaults.image.transparentBackground,
                responseFormat: current.responseFormat || defaults.image.responseFormat,
                outputFormat: current.outputFormat || defaults.image.outputFormat,
                maxOutputs: current.maxOutputs || defaults.image.maxOutputs,
            },
        };
    }
    if (capability === "video") {
        const defaults = defaultChannelModelCapabilityConfig(capability, protocol, modelKey);
        const current = existing?.video || defaults.video;
        const min = Math.max(1, Math.round(Number(form.videoMinSeconds.value || defaults.video.duration.min)));
        const max = Math.max(min, Math.round(Number(form.videoMaxSeconds.value || defaults.video.duration.max)));
        const fallback = Math.min(max, Math.max(min, Math.round(Number(form.videoDefaultSeconds.value || defaults.video.duration.default))));
        return {
            version: 1,
            video: {
                ...current,
                references: current.references || defaults.video.references,
                duration: { selection: "range", min, max, step: 1, default: fallback },
                ratios: checkedNamedValues(form, "videoRatios"),
                defaultRatio: form.videoDefaultRatio.value,
                resolutions: checkedNamedValues(form, "videoResolutions"),
                defaultResolution: form.videoDefaultResolution.value,
                generateAudio: { supported: form.videoGenerateAudio.checked, default: form.videoGenerateAudio.checked },
                watermark: { supported: form.videoWatermark.checked, default: false },
                operations: current.operations || defaults.video.operations,
                defaultOperation: current.defaultOperation || defaults.video.defaultOperation,
            },
        };
    }
    return undefined;
}

function fillChannelModelForm(item) {
    const form = document.getElementById("channel-model-form");
    if (!form) return;
    form.id.value = item ? item.id : "";
    form.modelKey.value = item ? item.modelKey || "" : "";
    form.providerModelKey.value = item ? item.providerModelKey || "" : "";
    form.displayName.value = item ? item.displayName || "" : "";
    form.capability.value = item ? item.capability || "video" : "video";
    const toapis = /toapis/i.test(selectedChannel()?.baseUrl || "");
    const fallbackProtocol = form.capability.value === "image"
        ? (toapis ? "toapis-image" : "openai-image")
        : (toapis ? "toapis-video" : "volcengine-ark-video");
    form.protocol.value = item ? item.protocol || fallbackProtocol : fallbackProtocol;
    form.enabled.checked = item ? item.enabled !== false : true;
    const config = item?.capabilityConfig || defaultChannelModelCapabilityConfig(form.capability.value, form.protocol.value, form.modelKey.value);
    const image = config?.image;
    const video = config?.video;
    setNamedCheckboxes(form, "imageRatios", image?.size?.values || ["1:1", "4:3", "3:4", "16:9", "9:16", "21:9"]);
    setNamedCheckboxes(form, "imageResolutions", image?.quality?.values || ["1k", "2k", "4k"]);
    form.imageDefaultRatio.value = image?.size?.default || "1:1";
    form.imageDefaultResolution.value = image?.quality?.default || "1k";
    setNamedCheckboxes(form, "videoRatios", video?.ratios || VIDEO_RATIO_OPTIONS);
    setNamedCheckboxes(form, "videoResolutions", video?.resolutions || ["480p", "720p", "1080p", "4k"]);
    form.videoDefaultRatio.value = video?.defaultRatio || "16:9";
    form.videoDefaultResolution.value = video?.defaultResolution || "720p";
    form.videoMinSeconds.value = video?.duration?.min || 4;
    form.videoMaxSeconds.value = video?.duration?.max || 15;
    form.videoDefaultSeconds.value = video?.duration?.default || 5;
    form.videoGenerateAudio.checked = video?.generateAudio?.supported !== false;
    form.videoWatermark.checked = video?.watermark?.supported === true;
    syncChannelModelParameterVisibility(form);
    const title = document.getElementById("channel-model-form-title");
    if (title) title.textContent = item ? `编辑渠道模型：${item.modelKey}` : "新增渠道模型";
}

function openChannelModelModal(item = null) {
    fillChannelModelForm(item);
    const modal = document.getElementById("channel-model-modal");
    if (!modal) return;
    modal.hidden = false;
    document.body.classList.add("modal-open");
    requestAnimationFrame(() => document.querySelector('#channel-model-form [name="modelKey"]')?.focus());
}

function closeChannelModelModal() {
    const modal = document.getElementById("channel-model-modal");
    if (modal) modal.hidden = true;
    document.body.classList.remove("modal-open");
}

async function withBusy(task) {
    state.busy = true;
    state.error = "";
    state.notice = "";
    try {
        await task();
    } catch (error) {
        state.error = error instanceof Error ? error.message : "操作失败";
    } finally {
        state.busy = false;
        document.body.classList.remove("modal-open");
        render();
    }
}

async function copyTextToClipboard(value) {
    if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(value);
        return;
    }
    const input = document.createElement("textarea");
    input.value = value;
    input.style.position = "fixed";
    input.style.opacity = "0";
    document.body.appendChild(input);
    input.select();
    const copied = document.execCommand("copy");
    input.remove();
    if (!copied) throw new Error("复制失败，请手动选择邀请码");
}

document.addEventListener("click", (event) => {
    if (event.target instanceof HTMLElement && event.target.id === "user-create-modal") {
        state.adminUserModalOpen = false;
        document.body.classList.remove("modal-open");
        render();
        return;
    }
    if (event.target instanceof HTMLElement && event.target.id === "channel-catalog-modal") {
        closeChannelCatalogModal();
        return;
    }
    if (event.target instanceof HTMLElement && event.target.id === "pricing-editor-modal") {
        closePricingEditorModal();
        return;
    }
    if (event.target instanceof HTMLElement && event.target.id === "channel-model-modal") {
        closeChannelModelModal();
        return;
    }
    if (event.target instanceof HTMLElement && event.target.id === "channel-form-modal") {
        closeChannelFormModal();
        return;
    }
    if (event.target instanceof HTMLElement && event.target.id === "ledger-detail-modal") {
        closeLedgerDetailModal();
        return;
    }
    const target = event.target.closest("[data-action]");
    if (!target) return;
    const action = target.dataset.action;
    if (action === "close-channel-form") {
        closeChannelFormModal();
    } else if (action === "close-pricing-editor") {
        closePricingEditorModal();
    } else if (action === "close-ledger-detail") {
        closeLedgerDetailModal();
    } else if (action === "logout") {
        event.preventDefault();
        withBusy(async () => {
            await logout();
            navigate(HOME_PATH);
        });
    } else if (action === "auth-mode") {
        state.authMode = target.dataset.mode;
        clearFlash();
        render();
    } else if (action === "toggle-password") {
        const input = document.getElementById("auth-password");
        if (!input) return;
        const show = input.type === "password";
        input.type = show ? "text" : "password";
        target.innerHTML = icon(show ? "eye-off" : "eye");
        target.setAttribute("aria-label", show ? "隐藏密码" : "显示密码");
    } else if (action === "refresh") {
        withBusy(() => reloadCurrent());
    } else if (action === "overview-days") {
        state.adminOverviewDays = Number(target.dataset.days || 7);
        withBusy(() => loadAdminOverview());
    } else if (action === "open-create-user") {
        state.adminUserModalOpen = true;
        render();
        requestAnimationFrame(() => document.querySelector('#create-user-form [name="username"]')?.focus());
    } else if (action === "close-create-user") {
        state.adminUserModalOpen = false;
        document.body.classList.remove("modal-open");
        render();
    } else if (action === "toggle-user-status") {
        withBusy(async () => {
            await updateAdminUser(target.dataset.id, {
                role: target.dataset.role,
                status: target.dataset.status,
            });
            await loadAdminUsers();
            state.notice = target.dataset.status === "active" ? "用户已启用" : "用户已停用";
        });
    } else if (action === "toggle-user-role") {
        const nextRole = target.dataset.role;
        if (!window.confirm(`确认将该用户改为${nextRole === "admin" ? "管理员" : "普通用户"}？`)) return;
        withBusy(async () => {
            await updateAdminUser(target.dataset.id, {
                role: nextRole,
                status: target.dataset.status,
            });
            await loadAdminUsers();
            state.notice = nextRole === "admin" ? "已设为管理员" : "已改为普通用户";
        });
    } else if (action === "reset-user-password") {
        const password = window.prompt("请输入新密码（至少 8 位）");
        if (password === null) return;
        if (password.length < 8) {
            withBusy(async () => { throw new Error("新密码至少 8 位"); });
            return;
        }
        withBusy(async () => {
            await resetAdminUserPassword(target.dataset.id, password);
            state.notice = "密码已重置，请通知用户使用新密码登录";
        });
    } else if (action === "admin-page") {
        const page = Math.max(1, Number(target.dataset.page || 1));
        withBusy(async () => {
            if (target.dataset.entity === "users") { state.adminUserPage = page; await loadAdminUsers(); }
            else if (target.dataset.entity === "projects") { state.adminProjectPage = page; await loadAdminProjects(); }
            else if (target.dataset.entity === "assets") { state.adminAssetPage = page; await loadAdminAssets(); }
            else if (target.dataset.entity === "ledger") { state.adminLedgerPage = page; await loadAdminLedger(); }
            else if (target.dataset.entity === "audit") { state.adminAuditPage = page; await loadAdminAudit(); }
        });
    } else if (action === "ledger-detail") {
        openLedgerDetailModal(state.adminLedger.find((item) => item.id === target.dataset.id));
    } else if (action === "archive-project") {
        const kind = target.dataset.kind;
        const reason = window.prompt(kind === "canvas" ? "请输入下架该画布的原因（必填）" : "请输入归档该项目的原因（必填）");
        if (reason === null) return;
        if (!reason.trim()) {
            withBusy(async () => { throw new Error("归档或下架必须填写原因"); });
            return;
        }
        withBusy(async () => {
            await archiveAdminProject(target.dataset.id, kind, reason.trim());
            await loadAdminProjects();
            state.notice = kind === "canvas" ? "画布已下架" : "项目已归档";
        });
    } else if (action === "restore-project") {
        withBusy(async () => {
            await restoreAdminProject(target.dataset.id, target.dataset.kind);
            await loadAdminProjects();
            state.notice = "内容已恢复";
        });
    } else if (action === "block-asset") {
        const reason = window.prompt("请输入下架该素材的原因（必填）");
        if (reason === null) return;
        if (!reason.trim()) {
            withBusy(async () => { throw new Error("下架素材必须填写原因"); });
            return;
        }
        withBusy(async () => {
            await blockAdminAsset(target.dataset.id, reason.trim());
            await loadAdminAssets();
            state.notice = "素材已下架";
        });
    } else if (action === "unblock-asset") {
        withBusy(async () => {
            await unblockAdminAsset(target.dataset.id);
            await loadAdminAssets();
            state.notice = "素材已恢复";
        });
    } else if (action === "use-user") {
        const form = document.getElementById("adjust-form");
        if (form) form.userId.value = target.dataset.user || "";
    } else if (action === "test-storage") {
        withBusy(async () => {
            await testStorage(target.dataset.id);
            await loadStorage();
            state.notice = "连通性测试通过";
        });
    } else if (action === "default-storage") {
        withBusy(async () => {
            await setDefaultStorage(target.dataset.id);
            await loadStorage();
            state.notice = "已设为默认存储";
        });
    } else if (action === "delete-storage") {
        if (!window.confirm("删除这条存储配置？已写入该存储的历史资源仍可按记录位置读取。")) return;
        withBusy(async () => {
            await deleteStorage(target.dataset.id);
            await loadStorage();
            state.notice = "配置已删除";
        });
    } else if (action === "edit-payment") {
        const item = state.payments.find((entry) => entry.id === target.dataset.id);
        if (item) fillPaymentForm(item);
    } else if (action === "reset-payment") {
        fillPaymentForm(null);

    } else if (action === "edit-topup") {
        const item = state.adminProducts.find((entry) => entry.id === target.dataset.id);
        if (item) fillTopupForm(item);
    } else if (action === "reset-topup") {
        fillTopupForm(null);
    } else if (action === "delete-topup") {
        if (!window.confirm("删除这个充值套餐？已生成的订单不受影响。")) return;
        withBusy(async () => {
            await deleteAdminProduct(target.dataset.id);
            await loadAdminProducts();
            fillTopupForm(null);
            state.notice = "套餐已删除";
        });
    } else if (action === "mark-order-paid") {
        if (!window.confirm("确认已收到该订单的款项？确认后积分立即入账。")) return;
        withBusy(async () => {
            await markOrderPaid(target.dataset.id, "");
            await loadAdminOrders();
            state.notice = "已确认支付并入账";
        });
    } else if (action === "close-order") {
        if (!window.confirm("关闭这个未支付订单？")) return;
        withBusy(async () => {
            await closeOrder(target.dataset.id, "管理员关闭");
            await loadAdminOrders();
            state.notice = "订单已关闭";
        });
    } else if (action === "expire-orders") {
        if (!window.confirm("关闭全部已超过支付时限的订单？")) return;
        withBusy(async () => {
            const result = await expireOrders();
            await loadAdminOrders();
            state.notice = `已关闭 ${result.closed || 0} 笔超时订单`;
        });
    } else if (action === "refund-order") {
        const reason = window.prompt("请输入退款原因（必填）");
        if (reason === null || !reason.trim()) return;
        const providerRefundNo = window.prompt("支付平台退款单号（可选）", "") || "";
        if (!window.confirm("确认已在该支付平台退款，并把对应积分从用户账户冲回？")) return;
        withBusy(async () => {
            await refundOrder(target.dataset.id, reason.trim(), providerRefundNo.trim());
            await loadAdminOrders();
            state.notice = "退款已记录，积分已冲回";
        });
    } else if (action === "delete-payment") {
        if (!window.confirm("删除这个支付渠道？已下订单不受影响，但之后无法再用它收款。")) return;
        withBusy(async () => {
            await deletePayment(target.dataset.id);
            await loadPayments();
            fillPaymentForm(null);
            state.notice = "支付渠道已删除";
        });
    } else if (action === "pricing-kind") {
        withBusy(async () => {
            await loadPricingWorkspace(target.dataset.kind);
            fillPricingForm(null);
        });
    } else if (action === "edit-pricing") {
        const item = state.pricingRules.find((entry) => entry.id === target.dataset.id);
        if (item) fillPricingForm(item);
        const advanced = document.getElementById("pricing-advanced-details");
        if (advanced) advanced.open = true;
    } else if (action === "reset-pricing") {
        fillPricingForm(null);
    } else if (action === "select-all-pricing-models") {
        document.querySelectorAll("[data-pricing-batch-model]").forEach((input) => { input.checked = true; });
        updatePricingBatchSelection();
    } else if (action === "clear-pricing-models") {
        document.querySelectorAll("[data-pricing-batch-model]").forEach((input) => { input.checked = false; });
        updatePricingBatchSelection();
    } else if (action === "delete-pricing") {
        if (!window.confirm("删除这条价格规则？历史任务已生成的报价不受影响。")) return;
        withBusy(async () => {
            await deletePricing(state.pricingKind, target.dataset.id);
            await loadPricing(state.pricingKind);
            fillPricingForm(null);
            state.notice = "价格规则已删除";
        });
    } else if (action === "new-channel") {
        openChannelFormModal();
    } else if (action === "reset-channel") {
        fillChannelForm(null);
    } else if (action === "select-channel") {
        withBusy(async () => {
            state.selectedChannelId = target.dataset.id;
            state.channelModelKeyword = "";
            state.channelModelCapability = "";
            state.channelModelCanvasStatus = "";
            await loadChannelModels();
        });
    } else if (action === "edit-channel") {
        const item = state.channels.find((entry) => entry.id === target.dataset.id);
        if (item) openChannelFormModal(item);
    } else if (action === "duplicate-channel") {
        if (!window.confirm("复制这个渠道？密钥、模型和价格档会一起复制，副本默认可用性同源。")) return;
        withBusy(async () => {
            await duplicateChannel(target.dataset.id);
            await loadChannels();
            state.notice = "渠道已复制";
        });
    } else if (action === "delete-channel") {
        if (!window.confirm("删除这个渠道？历史调用日志会保留，但之后无法再调度到它。")) return;
        withBusy(async () => {
            await deleteChannel(target.dataset.id);
            state.selectedChannelId = "";
            await loadChannels();
            fillChannelForm(null);
            state.notice = "渠道已删除";
        });
    } else if (action === "preview-channel-catalog") {
        withBusy(async () => {
            const result = await previewChannelModels(target.dataset.id);
            state.channelCatalogChannelId = target.dataset.id;
            state.channelCatalog = result.catalog || [];
            await loadChannelModels();
            openChannelCatalogModal();
        });
    } else if (action === "test-channel-model") {
        const model = state.channelModels.find((entry) => entry.id === target.dataset.id);
        if (model) void runChannelModelTest(model);
    } else if (action === "toggle-channel-model-visibility") {
        withBusy(async () => {
            const model = state.channelModels.find((entry) => entry.id === target.dataset.id);
            if (!model) return;
            const enabled = target.dataset.enabled === "true";
            await saveChannelModel(state.selectedChannelId, model.id, {
                modelKey: model.modelKey,
                providerModelKey: model.providerModelKey || "",
                displayName: model.displayName || "",
                capability: model.capability,
                protocol: model.protocol,
                capabilityConfig: model.capabilityConfig || undefined,
                enabled,
            });
            await loadChannelModels();
            state.notice = enabled ? "模型已上画布" : "模型已从画布隐藏";
        });
    } else if (action === "set-channel-model-price") {
        openPricingForChannelModel(target.dataset.id);
    } else if (action === "quick-set-channel-model-price") {
        const model = state.pricingChannelModels.find((entry) => entry.id === target.dataset.id)
            || state.channelModels.find((entry) => entry.id === target.dataset.id);
        if (model) void applyQuickChannelModelPrice(model);
    } else if (action === "apply-upstream-price") {
        applyUpstreamPrice(target);
    } else if (action === "apply-rmb-conversion") {
        const form = document.getElementById(target.dataset.form || "");
        applyPricingRmbConversion(form);
    } else if (action === "close-channel-catalog") {
        closeChannelCatalogModal();
    } else if (action === "fetch-channel-models") {
        withBusy(async () => {
            const result = await fetchChannelModels(target.dataset.id);
            await loadChannelModels();
            state.notice = `已从上游读到 ${(result.models || []).length} 个模型，新建 ${result.added || 0} 个（新建的默认停用）`;
        });
    } else if (action === "edit-channel-model") {
        const model = state.channelModels.find((entry) => entry.id === target.dataset.id);
        if (model) openChannelModelModal(model);
    } else if (action === "reset-channel-model") {
        fillChannelModelForm(null);
    } else if (action === "new-channel-model") {
        openChannelModelModal();
    } else if (action === "close-channel-model") {
        closeChannelModelModal();
    } else if (action === "delete-channel-model") {
        if (!window.confirm("删除这个渠道模型？若它正被前台线路或进行中的任务使用，服务端会整批拒绝。")) return;
        withBusy(async () => {
            await deleteChannelModel(state.selectedChannelId, target.dataset.id);
            await loadChannelModels();
            closeChannelModelModal();
            state.notice = "渠道模型已删除";
        });
    } else if (action === "new-creation") {
        fillCreationForm(null);
    } else if (action === "reset-creation") {
        fillCreationForm(null);
    } else if (action === "edit-creation") {
        const item = state.creationItems.find((entry) => entry.id === target.dataset.id);
        if (item) fillCreationForm(item);
    } else if (action === "delete-creation") {
        if (!window.confirm("删除这条创作配置？已使用它的历史项目不会受影响。")) return;
        withBusy(async () => {
            await deleteCreationCatalog(target.dataset.id);
            await loadCreationCatalog(state.creationKind);
            fillCreationForm(null);
            state.notice = "创作配置已删除";
        });
    } else if (action === "copy-invite") {
        withBusy(async () => {
            await copyTextToClipboard(target.dataset.code || "");
            state.notice = "邀请码已复制";
        });
    } else if (action === "toggle-invite") {
        withBusy(async () => {
            const enabled = target.dataset.enabled === "true";
            await setInviteCodeStatus(target.dataset.id, enabled);
            await loadInviteCodes();
            state.notice = enabled ? "邀请码已启用" : "邀请码已停用";
        });
    } else if (action === "delete-invite") {
        if (!window.confirm("删除这个邀请码？已核销记录会保留用于追溯。")) return;
        withBusy(async () => {
            await deleteInviteCode(target.dataset.id);
            await loadInviteCodes();
            state.notice = "邀请码已删除";
        });
    }
});

document.addEventListener("input", (event) => {
    if (event.target?.name === "modelKeyword") {
        state.channelModelKeyword = event.target.value;
        refreshChannelModelResults();
    }
    if (event.target?.closest?.("#pricing-editor-form") && event.target.name === "resolutionPrice") updatePricingUnitPreview();
    if (event.target && ["unitPriceCredits", "billingMode", "minimumChargeCredits"].includes(event.target.name)) updatePricingUnitPreview();
    if (event.target && ["catalog-exchange-rate", "catalog-price-multiplier"].includes(event.target.id)) updateCatalogQuoteConversions();
    if (event.target?.closest?.("[data-rmb-converter]")) updatePricingRmbPreview(event.target.closest("#pricing-form, #pricing-batch-form, #pricing-editor-form"));
    if (event.target?.id === "pricing-form") applyRecommendedBillingMode(event.target);
});

document.addEventListener("change", (event) => {
    const target = event.target;
    if (target?.closest?.("#channel-model-form")) {
        const form = target.closest("#channel-model-form");
        if (target.name === "capability") {
            const toapis = /toapis/i.test(selectedChannel()?.baseUrl || "");
            if (target.value === "image") form.protocol.value = toapis ? "toapis-image" : "openai-image";
            if (target.value === "video") form.protocol.value = toapis ? "toapis-video" : "volcengine-ark-video";
            setNamedCheckboxes(form, "imageRatios", []);
            setNamedCheckboxes(form, "imageResolutions", []);
            setNamedCheckboxes(form, "videoRatios", []);
            setNamedCheckboxes(form, "videoResolutions", []);
        }
        syncChannelModelParameterVisibility(form);
        return;
    }
    if (target?.closest?.("#channel-model-filter-form")) {
        // 关键词输入已经实时刷新；失焦时不要再替换表格，否则会吞掉行内按钮的点击。
        if (target.name === "modelKeyword") return;
        if (target.name === "modelCapability") state.channelModelCapability = target.value;
        if (target.name === "modelCanvasStatus") state.channelModelCanvasStatus = target.value;
        refreshChannelModelResults();
        return;
    }
    if (target.id === "channel-mobile-select") {
        state.selectedChannelId = target.value;
        state.channelModelKeyword = "";
        state.channelModelCapability = "";
        state.channelModelCanvasStatus = "";
        withBusy(() => loadChannelModels());
        return;
    }
    if (target?.closest?.("#pricing-form") && target.name === "capability") {
        applyRecommendedBillingMode(document.getElementById("pricing-form"));
    }
    if (target?.closest?.("#pricing-editor-form")) updatePricingUnitPreview();
    if (target.id === "pricing-batch-channel") {
        state.pricingBatchChannelId = target.value;
        withBusy(() => loadPricingChannelModels(state.pricingBatchChannelId));
        return;
    }
    if (target.id === "pricing-workspace-channel") {
        state.pricingBatchChannelId = target.value;
        withBusy(() => loadPricingChannelModels(state.pricingBatchChannelId));
        return;
    }
    if (target.id === "pricing-batch-select-all") {
        document.querySelectorAll("[data-pricing-batch-model]").forEach((input) => { input.checked = target.checked; });
        updatePricingBatchSelection();
        return;
    }
    if (target.matches?.("[data-pricing-batch-model]")) updatePricingBatchSelection();
});

document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
        if (state.adminUserModalOpen) {
            state.adminUserModalOpen = false;
            document.body.classList.remove("modal-open");
            render();
            return;
        }
        closeChannelFormModal();
        closePricingEditorModal();
        closeChannelModelModal();
        closeChannelCatalogModal();
    }
});

document.addEventListener("submit", (event) => {
    const form = event.target;
    event.preventDefault();
    if (form.getAttribute("id") === "auth-form") {
        withBusy(async () => {
            const username = form.username.value.trim();
            const password = form.password.value;
            if (state.authMode === "register") await register(username, form.displayName.value.trim(), password, form.inviteCode?.value.trim() || "");
            else await login(username, password);
            if (state.authMode === "login") {
                if (form.remember?.checked) rememberAdminCredentials(username, password);
                else forgetAdminCredentials();
            }
            await reloadCurrent();
            // 登录成功就让地址栏落到概览页，刷新后不会停在空 hash 上。
            if (!window.location.hash) navigate(HOME_PATH);
        });
    } else if (form.getAttribute("id") === "adjust-form") {
        withBusy(async () => {
            const amount = creditAmountToMicrocredits(form.amount.value, { allowNegative: true });
            await adjustCredits(form.userId.value.trim(), amount, form.reason.value.trim());
            await loadAccounts();
            state.notice = "调整已入账";
        });
    } else if (form.getAttribute("id") === "create-user-form") {
        withBusy(async () => {
            await createAdminUser({
                username: form.username.value.trim(),
                displayName: form.displayName.value.trim(),
                password: form.password.value,
                initialCredits: Math.max(0, Math.round(Number(form.initialCredits.value || 0))),
            });
            state.adminUserModalOpen = false;
            document.body.classList.remove("modal-open");
            state.adminUserPage = 1;
            await loadAdminUsers();
            state.notice = "用户已创建";
        });
    } else if (form.getAttribute("id") === "user-filter-form") {
        withBusy(async () => {
            state.adminUserKeyword = form.keyword.value.trim();
            state.adminUserRole = form.role.value;
            state.adminUserStatus = form.status.value;
            state.adminUserPage = 1;
            await loadAdminUsers();
        });
    } else if (form.getAttribute("id") === "project-filter-form") {
        withBusy(async () => {
            state.adminProjectKind = form.kind.value;
            state.adminProjectKeyword = form.keyword.value.trim();
            state.adminProjectStatus = form.status.value;
            if (state.adminProjectKind === "canvas" && state.adminProjectStatus === "archived") state.adminProjectStatus = "";
            if (state.adminProjectKind === "domain" && state.adminProjectStatus === "blocked") state.adminProjectStatus = "";
            state.adminProjectPage = 1;
            await loadAdminProjects();
        });
    } else if (form.getAttribute("id") === "asset-filter-form") {
        withBusy(async () => {
            state.adminAssetKeyword = form.keyword.value.trim();
            state.adminAssetKind = form.kind.value;
            state.adminAssetStatus = form.status.value;
            state.adminAssetPage = 1;
            await loadAdminAssets();
        });
    } else if (form.getAttribute("id") === "ledger-filter-form") {
        withBusy(async () => {
            state.adminLedgerKeyword = form.keyword.value.trim();
            state.adminLedgerModel = form.model.value.trim();
            state.adminLedgerKind = form.kind.value;
            state.adminLedgerToken = form.token.value;
            state.adminLedgerStart = form.start.value;
            state.adminLedgerEnd = form.end.value;
            state.adminLedgerPage = 1;
            await loadAdminLedger();
        });
    } else if (form.getAttribute("id") === "audit-filter-form") {
        withBusy(async () => {
            state.adminAuditKeyword = form.keyword.value.trim();
            state.adminAuditAction = form.action.value;
            state.adminAuditStart = form.start.value;
            state.adminAuditEnd = form.end.value;
            state.adminAuditPage = 1;
            await loadAdminAudit();
        });
    } else if (form.getAttribute("id") === "site-config-form") {
        withBusy(async () => {
            await saveAdminSiteConfig({
                siteName: form.siteName.value.trim(),
                logoUrl: form.logoUrl.value.trim(),
                filingNo: form.filingNo.value.trim(),
                supportUrl: form.supportUrl.value.trim(),
                creditName: form.creditName.value.trim(),
                currency: form.currency.value.trim(),
                footerText: form.footerText.value.trim(),
                announcement: form.announcement.value.trim(),
                maintenanceMode: form.maintenanceMode.checked,
            });
            state.notice = "站点配置已保存";
        });
    } else if (form.getAttribute("id") === "storage-form") {
        withBusy(async () => {
            await createStorage({
                name: form.name.value.trim(),
                provider: form.provider.value,
                endpoint: form.endpoint.value.trim(),
                region: form.region.value.trim(),
                bucket: form.bucket.value.trim(),
                accessKeyId: form.accessKeyId.value.trim(),
                secretAccessKey: form.secretAccessKey.value,
                cdnBaseUrl: form.cdnBaseUrl.value.trim(),
                prefix: form.prefix.value.trim(),
                pathStyle: form.pathStyle.checked,
                enabled: true,
            });
            await loadStorage();
            state.notice = "配置已保存，请测试连通性";
        });
    } else if (form.getAttribute("id") === "payment-form") {
        withBusy(async () => {
            const paymentId = form.id.value.trim();
            await savePayment(paymentId, {
                provider: form.provider.value,
                name: form.name.value.trim(),
                appId: form.appId.value.trim(),
                mchId: form.mchId.value.trim(),
                notifyUrl: form.notifyUrl.value.trim(),
                returnUrl: form.returnUrl.value.trim(),
                endpoint: form.endpoint.value.trim(),
                payType: form.payType.value,
                apiKey: form.apiKey.value,
                privateKey: form.privateKey.value,
                platformPublicKey: form.platformPublicKey.value,
                mchSerialNo: form.mchSerialNo.value.trim(),
                enabled: form.enabled.checked,
            });
            await loadPayments();
            fillPaymentForm(null);
            state.notice = paymentId ? "支付渠道已更新" : "支付渠道已保存";
        });
    } else if (form.getAttribute("id") === "pricing-editor-form") {
        withBusy(async () => {
            const channelId = form.channelId.value.trim();
            const channelModelId = form.channelModelId.value.trim();
            const model = state.pricingChannelModels.find((item) => item.id === channelModelId)
                || state.channelModels.find((item) => item.id === channelModelId);
            const modelName = model?.displayName || model?.modelKey || channelModelId;
            const rows = Array.from(form.querySelectorAll("[data-pricing-editor-row]"));
            const operations = rows.map((row) => {
                const resolution = row.dataset.resolution || "";
                const selector = resolution ? { quality: resolution } : {};
                const existing = findChannelTierRule(channelId, channelModelId, selector);
                const enabled = row.querySelector('[name="pricingEnabled"]').checked;
                const priceText = row.querySelector('[name="resolutionPrice"]').value.trim();
                return { row, resolution, selector, existing, enabled, priceText };
            }).filter((item) => item.enabled || item.existing);
            if (!operations.length) throw new Error("请至少勾选一个价格档位");
            for (const operation of operations) {
                if (operation.enabled && (!operation.priceText || !(Number(operation.priceText) > 0))) {
                    throw new Error(`请填写“${operation.resolution || "通用价"}”的积分单价`);
                }
            }
            let saved = 0;
            const failures = [];
            for (const operation of operations) {
                const existing = operation.existing;
                const input = {
                    name: existing?.name || `${modelName} ${operation.resolution || "通用"} 渠道价`,
                    channelId,
                    channelModelId,
                    capability: existing?.capability || form.capability.value,
                    billingMode: operation.enabled ? form.billingMode.value : existing.billingMode,
                    selector: operation.selector,
                    unitPriceMicrocredits: operation.enabled
                        ? creditPriceToMicrocredits(operation.priceText)
                        : existing.unitPriceMicrocredits,
                    minimumChargeMicrocredits: operation.enabled
                        ? creditAmountToMicrocredits(form.minimumChargeCredits.value)
                        : existing.minimumChargeCredits,
                    enabled: operation.enabled,
                    sortOrder: Math.round(Number(existing?.sortOrder || 0)),
                    note: form.note.value.trim() || existing?.note || "快速定价",
                };
                if (existing?.effectiveFrom) input.effectiveFrom = existing.effectiveFrom;
                input.effectiveTo = existing?.effectiveTo || null;
                try {
                    await savePricing("channel_tier", existing?.id || "", input);
                    saved += 1;
                } catch (error) {
                    failures.push(`${operation.resolution || "通用价"}：${error instanceof Error ? error.message : "保存失败"}`);
                }
            }
            await loadPricing("channel_tier");
            if (failures.length) throw new Error(`部分价格保存失败：${failures.join("；")}`);
            closePricingEditorModal();
            state.notice = `已保存 ${saved} 个价格档位`;
        });
    } else if (form.getAttribute("id") === "pricing-form") {
        withBusy(async () => {
            const kind = state.pricingKind;
            const pricingID = form.id.value.trim();
            const input = {
                name: form.name.value.trim(),
                logicalModelId: form.logicalModelId.value.trim(),
                channelId: form.channelId.value.trim(),
                channelModelId: form.channelModelId.value.trim(),
                channelModelPriceTierId: form.channelModelPriceTierId.value.trim(),
                capability: form.capability.value,
                billingMode: form.billingMode.value,
                selector: form.selector.value.trim() ? JSON.parse(form.selector.value) : {},
                unitPriceMicrocredits: creditPriceToMicrocredits(form.unitPriceCredits.value),
                minimumChargeMicrocredits: creditAmountToMicrocredits(form.minimumChargeCredits.value),
                enabled: form.enabled.checked,
                sortOrder: Math.round(Number(form.sortOrder.value || 0)),
                note: form.note.value.trim(),
            };
            if (form.effectiveFrom.value) input.effectiveFrom = new Date(form.effectiveFrom.value).toISOString();
            input.effectiveTo = form.effectiveTo.value ? new Date(form.effectiveTo.value).toISOString() : null;
            await savePricing(kind, pricingID, input);
            await loadPricing(kind);
            fillPricingForm(null);
            state.notice = pricingID ? "价格规则已更新" : "价格规则已保存";
        });
    } else if (form.getAttribute("id") === "pricing-batch-form") {
        withBusy(async () => {
            const channelId = state.pricingBatchChannelId;
            if (!channelId) throw new Error("请先选择渠道");
            const selectedIds = Array.from(document.querySelectorAll("[data-pricing-batch-model]:checked")).map((input) => input.value);
            if (!selectedIds.length) throw new Error("请至少勾选一个渠道模型");
            const unitPriceMicrocredits = creditPriceToMicrocredits(form.unitPriceCredits.value);
            const selector = form.selector.value.trim() ? JSON.parse(form.selector.value) : {};
            const minimumChargeMicrocredits = creditAmountToMicrocredits(form.minimumChargeCredits.value);
            const skipExisting = form.skipExisting.checked;
            let saved = 0;
            let skipped = 0;
            const failures = [];
            for (const model of state.pricingChannelModels.filter((item) => selectedIds.includes(item.id))) {
                const existing = findChannelTierRule(channelId, model.id, selector);
                if (existing && skipExisting) {
                    skipped += 1;
                    continue;
                }
                const input = {
                    name: existing?.name || `${model.displayName || model.modelKey || model.id} 渠道价`,
                    channelId,
                    channelModelId: model.id,
                    capability: form.capability.value || existing?.capability || model.capability || "text",
                    billingMode: form.billingMode.value,
                    selector,
                    unitPriceMicrocredits,
                    minimumChargeMicrocredits,
                    enabled: form.enabled.checked,
                    sortOrder: existing?.sortOrder || 0,
                    note: existing?.note || "批量设置",
                };
                if (form.effectiveFrom.value) input.effectiveFrom = new Date(form.effectiveFrom.value).toISOString();
                try {
                    await savePricing("channel_tier", existing?.id || "", input);
                    saved += 1;
                } catch (error) {
                    failures.push(`${model.displayName || model.modelKey || model.id}：${error instanceof Error ? error.message : "保存失败"}`);
                }
            }
            await loadPricing("channel_tier");
            if (failures.length) throw new Error(`成功 ${saved} 个，跳过 ${skipped} 个，失败 ${failures.length} 个。${failures.slice(0, 3).join("；")}`);
            state.notice = `已为 ${saved} 个模型应用价格${skipped ? `，跳过 ${skipped} 个已有同选择器价格的模型` : ""}`;
        });
    } else if (form.getAttribute("id") === "topup-form") {
        withBusy(async () => {
            const productId = form.id.value.trim();
            await saveAdminProduct(productId, {
                name: form.name.value.trim(),
                priceFen: Math.round(Number(form.priceYuan.value) * 100),
                credits: Math.round(Number(form.credits.value)),
                bonus: Math.round(Number(form.bonus.value || 0)),
                provider: form.provider.value,
                enabled: form.enabled.checked,
                sortOrder: Math.round(Number(form.sortOrder.value || 0)),
            });
            await loadAdminProducts();
            fillTopupForm(null);
            state.notice = productId ? "套餐已更新" : "套餐已保存";
        });
    }
    else if (form.getAttribute("id") === "channel-filter-form") {
        withBusy(async () => {
            state.channelKeyword = form.keyword.value.trim();
            state.channelStatus = form.status.value;
            await loadChannels();
        });
    } else if (form.getAttribute("id") === "channel-model-filter-form") {
        state.channelModelKeyword = form.modelKeyword.value;
        state.channelModelCapability = form.modelCapability.value;
        state.channelModelCanvasStatus = form.modelCanvasStatus.value;
        refreshChannelModelResults();
    } else if (form.getAttribute("id") === "channel-form") {
        withBusy(async () => {
            const channelId = form.id.value.trim();
            const headersRaw = form.headers.value.trim();
            const input = {
                name: form.name.value.trim(),
                publicAlias: form.publicAlias.value.trim(),
                baseUrl: form.baseUrl.value.trim(),
                apiKey: form.apiKey.value,
                secretKey: form.secretKey.value,
                sortOrder: Math.round(Number(form.sortOrder.value || 0)),
                enabled: form.enabled.checked,
            };
            if (form.useGlobalConcurrency.checked) input.useGlobalConcurrency = true;
            else input.concurrencyLimit = Math.round(Number(form.concurrencyLimit.value || 0));
            if (headersRaw) input.headers = JSON.parse(headersRaw);
            const result = await saveChannel(channelId, input);
            state.selectedChannelId = (result.item && result.item.id) || channelId;
            await loadChannels();
            closeChannelFormModal();
            state.notice = channelId ? "渠道已更新" : "渠道已创建";
        });
    } else if (form.getAttribute("id") === "channel-model-form") {
        withBusy(async () => {
            if (!state.selectedChannelId) throw new Error("请先选择一个渠道");
            const modelId = form.id.value.trim();
            const existing = state.channelModels.find((item) => item.id === modelId);
            await saveChannelModel(state.selectedChannelId, modelId, {
                modelKey: form.modelKey.value.trim(),
                providerModelKey: form.providerModelKey.value.trim(),
                displayName: form.displayName.value.trim(),
                capability: form.capability.value,
                protocol: form.protocol.value,
                capabilityConfig: channelModelCapabilityConfigFromForm(form, existing?.capabilityConfig),
                enabled: form.enabled.checked,
            });
            await loadChannelModels();
            closeChannelModelModal();
            state.notice = modelId ? "渠道模型已更新" : "渠道模型已保存";
        });
    } else if (form.getAttribute("id") === "creation-filter-form") {
        withBusy(async () => {
            state.creationKind = creationCatalogKind(form.kind.value);
            state.creationKeyword = form.keyword.value.trim();
            state.creationStatus = form.status.value;
            await loadCreationCatalog(state.creationKind);
        });
    } else if (form.getAttribute("id") === "creation-form") {
        withBusy(async () => {
            const value = (name) => form.elements.namedItem(name)?.value ?? "";
            const payloadRaw = value("payloadJson").trim();
            let payload;
            if (payloadRaw) {
                try { payload = JSON.parse(payloadRaw); } catch { throw new Error("流程定义 JSON 格式错误"); }
            }
            const id = value("id").trim();
            const kind = creationCatalogKind(value("kind"));
            await saveCreationCatalog(id, {
                kind,
                itemKey: value("itemKey").trim(),
                name: value("name").trim(),
                scene: value("scene"),
                group: value("group").trim(),
                description: value("description").trim(),
                content: value("content").trim(),
                variables: value("variables").trim(),
                canvasType: value("canvasType"),
                defaultModel: value("defaultModel").trim(),
                coverUrl: value("coverUrl").trim(),
                payload,
                sortOrder: Math.round(Number(value("sortOrder") || 0)),
                enabled: form.enabled.checked,
            });
            state.creationKind = kind;
            await loadCreationCatalog(kind);
            fillCreationForm(null);
            state.notice = id ? "创作配置已更新" : "创作配置已保存";
        });
    } else if (form.getAttribute("id") === "registration-policy-form") {
        withBusy(async () => {
            await saveRegistrationPolicy({
                mode: form.mode.value,
                defaultBonusCredits: Math.max(0, Math.round(Number(form.defaultBonusCredits.value || 0))),
                inviteDefaultMaxUses: Math.max(1, Math.round(Number(form.inviteDefaultMaxUses.value || 1))),
                inviteDefaultValidDays: Math.max(0, Math.round(Number(form.inviteDefaultValidDays.value || 0))),
            });
            state.notice = "注册策略已保存";
        });
    } else if (form.getAttribute("id") === "invite-generate-form") {
        withBusy(async () => {
            const created = await generateInviteCodes({
                count: Math.max(1, Math.min(500, Math.round(Number(form.count.value || 1)))),
                batch: form.batch.value.trim(),
                prefix: form.prefix.value.trim(),
                maxUses: Math.max(1, Math.round(Number(form.maxUses.value || 1))),
                validDays: Math.max(0, Math.round(Number(form.validDays.value || 0))),
                note: form.note.value.trim(),
                autoEnabled: form.autoEnabled.checked,
            });
            await loadInviteCodes();
            state.notice = `已生成 ${created.length} 个邀请码`;
        });
    } else if (form.getAttribute("id") === "invite-filter-form") {
        withBusy(async () => {
            state.inviteKeyword = form.keyword.value.trim();
            state.inviteStatus = form.status.value;
            await loadInviteCodes();
        });
    }
});

window.addEventListener("hashchange", () => {
    clearFlash();
    withBusy(() => reloadCurrent());
});

async function reloadCurrent() {
    // 普通用户没有后台数据可拉，登录后会落到“没有权限”页。
    if (!state.user || state.user.role !== "admin") return;
    const path = currentPath();
    if (path === "/admin/overview") return loadAdminOverview();
    if (path === "/admin/users") return loadAdminUsers();
    if (path === "/admin/projects") return loadAdminProjects();
    if (path === "/admin/assets") return loadAdminAssets();
    if (path === "/admin/ledger") return loadAdminLedger();
    if (path === "/admin/site") return loadAdminSiteConfig();
    if (path === "/admin/audit") return loadAdminAudit();
    if (path === "/admin/credits") return loadAccounts();
    if (path === "/admin/registration") return Promise.all([loadRegistrationPolicy(), loadInviteCodes()]);
    if (path === "/admin/topup") return loadAdminProducts();
    if (path === "/admin/orders") return loadAdminOrders();
    if (path === "/admin/storage") return loadStorage();
    if (path === "/admin/payment") return loadPayments();
    if (path === "/admin/pricing") return loadPricingWorkspace(state.pricingKind);
    if (path === "/admin/models") return loadChannels();
    if (path === "/admin/workflows") return loadCreationCatalog("workflow");
    if (path === "/admin/prompts") return loadCreationCatalog("prompt");
    if (path === "/admin/styles") return loadCreationCatalog("style");
    return undefined;
}


function orderStatusTag(status) {
    if (status === "refunded") return '<span class="tag tag-muted">已退款</span>';
    if (status === "paid") return '<span class="tag tag-ok">已支付</span>';
    if (status === "closed") return '<span class="tag tag-muted">已关闭</span>';
    return '<span class="tag tag-warn">待支付</span>';
}

function orderStatusLabel(status) {
    if (status === "refunded") return "已退款";
    if (status === "paid") return "已支付";
    if (status === "closed") return "已关闭";
    return "待支付";
}

function renderAdminTopup(view) {
    view.innerHTML = `
        <div class="page-head">
            <div>
                <h1 class="page-title">充值商品</h1>
                <p class="page-sub">价格按元填写，用户下单时快照价格与积分，改价不影响历史订单。</p>
            </div>
            <button class="btn btn-sm" data-action="refresh">刷新</button>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel">
            <h2 class="panel-title" id="topup-form-title">新增充值商品</h2>
            <form id="topup-form">
                <input type="hidden" name="id" />
                <div class="form-grid">
                    <label class="field"><span class="field-label">套餐名称</span><input class="input" name="name" required /></label>
                    <label class="field"><span class="field-label">价格（元）</span><input class="input" name="priceYuan" type="number" step="0.01" min="0.01" required /></label>
                    <label class="field"><span class="field-label">基础积分</span><input class="input" name="credits" type="number" step="1" min="1" required /></label>
                    <label class="field"><span class="field-label">赠送积分</span><input class="input" name="bonus" type="number" step="1" min="0" value="0" /></label>
                    <label class="field"><span class="field-label">默认渠道</span>
                        <select class="select" name="provider">
                            <option value="alipay">支付宝</option>
                            <option value="wechat">微信支付</option>
                            <option value="epay">易支付</option>
                        </select>
                    </label>
                    <label class="field"><span class="field-label">排序（小的在前）</span><input class="input" name="sortOrder" type="number" step="1" value="0" /></label>
                </div>
                <div class="btn-row">
                    <label class="muted"><input type="checkbox" name="enabled" checked /> 上架</label>
                    <button class="btn btn-sm" type="button" data-action="reset-topup">清空表单</button>
                    <button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>保存套餐</button>
                </div>
            </form>
        </div>
        <div class="panel">
            <h2 class="panel-title">已有套餐</h2>
            ${renderAdminProducts()}
        </div>`;
}

function renderAdminProducts() {
    if (!state.adminProducts.length) return `<div class="empty">还没有充值套餐</div>`;
    return `
        <div class="table-wrap">
            <table>
                <thead><tr><th>套餐</th><th class="num">价格</th><th class="num">积分</th><th>渠道</th><th>状态</th><th></th></tr></thead>
                <tbody>
                    ${state.adminProducts
                        .map(
                            (item) => `<tr>
                                <td>${escapeHtml(item.name)}</td>
                                <td class="num">￥${escapeHtml(formatFenToYuan(item.priceFen))}</td>
                                <td class="num">${escapeHtml(formatPoints(item.credits + item.bonus))}</td>
                                <td class="muted">${item.provider === "wechat" ? "微信支付" : item.provider === "epay" ? "易支付" : "支付宝"}</td>
                                <td>${item.enabled ? '<span class="tag tag-ok">上架</span>' : '<span class="tag tag-muted">下架</span>'}</td>
                                <td>
                                    <div class="btn-row">
                                        <button class="btn btn-sm" data-action="edit-topup" data-id="${escapeHtml(item.id)}">编辑</button>
                                        <button class="btn btn-sm btn-danger" data-action="delete-topup" data-id="${escapeHtml(item.id)}">删除</button>
                                    </div>
                                </td>
                            </tr>`,
                        )
                        .join("")}
                </tbody>
            </table>
        </div>`;
}

function fillTopupForm(item) {
    const form = document.getElementById("topup-form");
    if (!form) return;
    form.id.value = item ? item.id : "";
    form.name.value = item ? item.name : "";
    form.priceYuan.value = item ? (item.priceFen / 100).toFixed(2) : "";
    form.credits.value = item ? item.credits : "";
    form.bonus.value = item ? item.bonus : 0;
    form.provider.value = item ? item.provider : "alipay";
    form.sortOrder.value = item ? item.sortOrder : 0;
    form.enabled.checked = item ? item.enabled !== false : true;
    const title = document.getElementById("topup-form-title");
    if (title) title.textContent = item ? `编辑套餐：${item.name}` : "新增充值商品";
}

function renderAdminOrders(view) {
    view.innerHTML = `
        <div class="page-head">
            <div>
                <h1 class="page-title">支付订单</h1>
                <p class="page-sub">确认线下收款会按订单快照入账；退款先在支付平台操作，再在这里记录并冲回积分。</p>
            </div>
            <div class="btn-row">
                <button class="btn btn-sm" data-action="expire-orders" ${state.busy ? "disabled" : ""}>关闭超时订单</button>
                <button class="btn btn-sm" data-action="refresh">刷新</button>
            </div>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel">
            <h2 class="panel-title">对账概览</h2>
            ${renderReconciliation(state.reconciliation)}
        </div>
        <div class="panel">
            <h2 class="panel-title">全部订单（最近 100 条）</h2>
            ${renderAdminOrdersTable()}
        </div>`;
}

function renderReconciliation(snapshot) {
    if (!snapshot) return `<div class="empty">暂时没有对账数据</div>`;
    const counts = snapshot.ordersByStatus || {};
    const statusSummary = ["pending_payment", "paid", "refunded", "closed"]
        .map((status) => `${orderStatusLabel(status)} ${counts[status] || 0}`)
        .join(" / ");
    return `
        <div class="metric-row">
            ${metric("已结算金额", `￥${formatFenToYuan(snapshot.settledAmountFen)}`)}
            ${metric("已结算积分", formatPoints(snapshot.settledCredits))}
            ${metric("待支付超时", formatPoints(snapshot.pendingOverdue))}
            ${metric("已支付未入账", formatPoints(snapshot.paidUnsettled))}
            ${metric("未处理回调", formatPoints(snapshot.notificationsUnprocessed))}
            ${metric("失败回调", formatPoints(snapshot.notificationsFailed))}
        </div>
        <p class="muted" style="margin:12px 0 0">订单状态：${escapeHtml(statusSummary)}</p>`;
}

function renderAdminOrdersTable() {
    if (!state.adminOrders.length) return `<div class="empty">还没有充值订单</div>`;
    return `
        <div class="table-wrap">
            <table>
                <thead><tr><th>时间</th><th>用户</th><th>套餐</th><th class="num">金额</th><th class="num">积分</th><th>渠道</th><th>状态</th><th>订单号</th><th></th></tr></thead>
                <tbody>
                    ${state.adminOrders
                        .map(
                            (item) => `<tr>
                                <td class="muted">${escapeHtml(formatTime(item.createdAt))}</td>
                                <td class="muted">${escapeHtml(item.userId)}</td>
                                <td>${escapeHtml(item.productName)}</td>
                                <td class="num">￥${escapeHtml(formatFenToYuan(item.amountFen))}</td>
                                <td class="num">${escapeHtml(formatPoints(item.credits))}</td>
                                <td class="muted">${item.provider === "wechat" ? "微信" : item.provider === "epay" ? "易支付" : "支付宝"}</td>
                                <td>${orderStatusTag(item.status)}</td>
                                <td class="muted">${escapeHtml(item.id)}</td>
                                <td>
                                    <div class="btn-row">
                                        ${item.status === "pending_payment" ? `<button class="btn btn-sm btn-primary" data-action="mark-order-paid" data-id="${escapeHtml(item.id)}">确认支付</button><button class="btn btn-sm" data-action="close-order" data-id="${escapeHtml(item.id)}">关闭</button>` : ""}
                                        ${item.status === "paid" ? `<button class="btn btn-sm btn-danger" data-action="refund-order" data-id="${escapeHtml(item.id)}">退款</button>` : ""}
                                    </div>
                                </td>
                            </tr>`,
                        )
                        .join("")}
                </tbody>
            </table>
        </div>`;
}

function renderAdminPayment(view) {
    view.innerHTML = `
        <div class="page-head">
            <div>
                <h1 class="page-title">支付渠道</h1>
                <p class="page-sub">配置支付宝 / 微信商户号。密钥加密落库，列表只回显掩码；编辑时留空表示沿用原密钥。</p>
            </div>
            <button class="btn btn-sm" data-action="refresh">刷新</button>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel">
            <h2 class="panel-title" id="payment-form-title">添加支付渠道</h2>
            <form id="payment-form">
                <input type="hidden" name="id" />
                <div class="form-grid">
                    <label class="field"><span class="field-label">渠道名称</span><input class="input" name="name" required /></label>
                    <label class="field"><span class="field-label">类型</span>
                        <select class="select" name="provider">
                            <option value="alipay">支付宝</option>
                            <option value="wechat">微信支付</option>
                            <option value="epay">易支付</option>
                        </select>
                    </label>
                    <label class="field"><span class="field-label">AppID</span><input class="input" name="appId" /></label>
                    <label class="field"><span class="field-label">商户号 MchID</span><input class="input" name="mchId" /></label>
                    <label class="field"><span class="field-label">商户证书序列号（微信）</span><input class="input" name="mchSerialNo" /></label>
                    <label class="field"><span class="field-label">易支付网关地址（易支付填写）</span><input class="input" name="endpoint" placeholder="https://pay.example.com/submit.php" /></label>
                    <label class="field"><span class="field-label">易支付付款方式</span><select class="select" name="payType"><option value="alipay">支付宝</option><option value="wxpay">微信支付</option><option value="qqpay">QQ 钱包</option></select></label>
                    <label class="field"><span class="field-label">支付完成返回地址（易支付填写）</span><input class="input" name="returnUrl" placeholder="https://你的域名/" /></label>
                    <label class="field"><span class="field-label">回调地址 NotifyURL</span><input class="input" name="notifyUrl" placeholder="https://你的域名/api/payments/epay/notify" /></label>
                    <label class="field"><span class="field-label">API Key / APIv3 Key</span><input class="input" name="apiKey" type="password" autocomplete="new-password" placeholder="留空表示不修改" /></label>
                    <label class="field"><span class="field-label">应用私钥（支付宝）</span><textarea class="textarea" name="privateKey" rows="3" placeholder="留空表示不修改"></textarea></label>
                    <label class="field"><span class="field-label">平台公钥</span><textarea class="textarea" name="platformPublicKey" rows="3" placeholder="留空表示不修改"></textarea></label>
                </div>
                <div class="btn-row">
                    <label class="muted"><input type="checkbox" name="enabled" checked /> 启用该渠道</label>
                    <button class="btn btn-sm" type="button" data-action="reset-payment">清空表单</button>
                    <button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>保存渠道</button>
                </div>
            </form>
        </div>
        <div class="panel">
            <h2 class="panel-title">已有渠道</h2>
            ${renderPayments()}
        </div>`;
}

function renderPayments() {
    if (!state.payments.length) return `<div class="empty">还没有支付渠道，请先添加一条</div>`;
    return `
        <div class="table-wrap">
            <table>
                <thead><tr><th>名称</th><th>类型</th><th>AppID</th><th>商户号</th><th>密钥</th><th>状态</th><th></th></tr></thead>
                <tbody>
                    ${state.payments
                        .map(
                            (item) => `<tr>
                                <td>${escapeHtml(item.name)}</td>
                                <td class="muted">${item.provider === "wechat" ? "微信支付" : item.provider === "epay" ? "易支付" : "支付宝"}</td>
                                <td class="muted">${escapeHtml(item.appId || "-")}</td>
                                <td class="muted">${escapeHtml(item.mchId || "-")}</td>
                                <td class="muted">${escapeHtml(item.secretMasked || "-")}</td>
                                <td>${item.enabled ? '<span class="tag tag-ok">启用</span>' : '<span class="tag tag-muted">停用</span>'}</td>
                                <td>
                                    <div class="btn-row">
                                        <button class="btn btn-sm" data-action="edit-payment" data-id="${escapeHtml(item.id)}">编辑</button>
                                        <button class="btn btn-sm btn-danger" data-action="delete-payment" data-id="${escapeHtml(item.id)}">删除</button>
                                    </div>
                                </td>
                            </tr>`,
                        )
                        .join("")}
                </tbody>
            </table>
        </div>`;
}

function fillPaymentForm(item) {
    const form = document.getElementById("payment-form");
    if (!form) return;
    form.id.value = item ? item.id : "";
    form.name.value = item ? item.name : "";
    form.provider.value = item ? item.provider : "alipay";
    form.appId.value = item ? item.appId || "" : "";
    form.mchId.value = item ? item.mchId || "" : "";
    form.mchSerialNo.value = item ? item.mchSerialNo || "" : "";
    form.notifyUrl.value = item ? item.notifyUrl || "" : "";
    form.returnUrl.value = item ? item.returnUrl || "" : "";
    form.endpoint.value = item ? item.endpoint || "" : "";
    form.payType.value = item ? item.payType || "alipay" : "alipay";
    form.apiKey.value = "";
    form.privateKey.value = "";
    form.platformPublicKey.value = "";
    form.enabled.checked = item ? item.enabled !== false : true;
    const title = document.getElementById("payment-form-title");
    if (title) title.textContent = item ? `编辑渠道：${item.name}` : "添加支付渠道";
}

function pricingKindLabel(kind) {
    if (kind === "logical_sku") return "逻辑模型 SKU";
    if (kind === "channel_tier") return "渠道模型价格档";
    return "模型默认价";
}

function billingModeLabel(mode) {
    const labels = {
        fixed_request: "每次",
        per_second: "每秒",
        per_image: "每张图",
        per_1k_tokens: "每千 Token",
        per_megabyte: "每 MB",
    };
    return labels[mode] || mode || "-";
}

function pricingSelectorText(selector) {
    const entries = Object.entries(selector || {});
    return entries.length ? entries.map(([key, value]) => `${key}=${value}`).join(", ") : "默认";
}

function activeDefaultChannelTierRule(channelId, channelModelId) {
    const rule = findChannelTierRule(channelId, channelModelId, {});
    if (!rule) return null;
    return activeChannelTierRulesForModel(channelId, channelModelId).find((item) => item.id === rule.id) || null;
}

function renderPricingWorkspaceTable(channel) {
    if (!state.pricingChannels.length) {
        return '<div class="empty">还没有模型渠道。请先到「模型渠道」新增渠道并读取模型。</div>';
    }
    if (!state.pricingChannelModels.length) {
        return '<div class="empty">这个渠道还没有模型。请先到「模型渠道」读取上游目录或手动新增模型。</div>';
    }
    return `
        <div class="table-wrap pricing-channel-model-table">
            <table id="pricing-channel-model-table">
                <thead><tr><th>模型</th><th>能力</th><th>计费单位</th><th class="num">当前积分价格</th><th>画布状态</th><th></th></tr></thead>
                <tbody>
                    ${state.pricingChannelModels.map((model) => {
                        const rule = activeDefaultChannelTierRule(channel?.id || "", model.id);
                        const anyDefault = findChannelTierRule(channel?.id || "", model.id, {});
                        const selectorRules = activeChannelTierRulesForModel(channel?.id || "", model.id).filter((item) => Object.keys(item.selector || {}).length > 0);
                        const billingMode = rule?.billingMode || anyDefault?.billingMode || recommendedBillingMode(model.capability || "text");
                        const visible = Boolean(channel?.enabled && model.enabled);
                        return `<tr>
                            <td><strong>${escapeHtml(model.displayName || model.modelKey || model.id)}</strong><div class="muted">${escapeHtml(model.modelKey || model.id)} · ${escapeHtml(upstreamPricingText(model))}</div></td>
                            <td>${escapeHtml(capabilityLabel(model.capability || "text"))}</td>
                            <td>${escapeHtml(billingModeLabel(billingMode))}</td>
                            <td class="num">${rule ? `<strong>${escapeHtml(formatMicrocredits(rule.unitPriceMicrocredits))}</strong> 积分<div class="muted">通用价 · 最低 ${escapeHtml(formatMicrocredits(rule.minimumChargeCredits))} 积分</div>` : selectorRules.length ? `<strong>${escapeHtml(selectorRules.map((item) => `${item.selector?.quality || "规格"}=${formatMicrocredits(item.unitPriceMicrocredits)}`).join(" / "))}</strong> 积分<div class="muted">按分辨率价格</div>` : anyDefault ? '<span class="tag tag-warn">规则未生效</span><div class="muted">打开高级规则检查启停与时间</div>' : '<span class="tag tag-warn">未设价</span>'}</td>
                            <td>${visible ? '<span class="tag tag-ok">画布可见</span>' : channel?.enabled === false ? '<span class="tag tag-warn">渠道停用</span>' : '<span class="tag tag-muted">画布隐藏</span>'}</td>
                            <td><div class="btn-row"><button class="btn btn-sm btn-primary" data-action="quick-set-channel-model-price" data-id="${escapeHtml(model.id)}">一键 1 积分</button><button class="btn btn-sm" data-action="set-channel-model-price" data-id="${escapeHtml(model.id)}">设置价格</button></div></td>
                        </tr>`;
                    }).join("")}
                </tbody>
            </table>
        </div>`;
}


function pricingRuleScope(rule) {
    if (rule.kind === "logical_sku") return `逻辑模型 ${rule.logicalModelId || "-"}${rule.channelModelPriceTierId ? ` / 渠道档 ${rule.channelModelPriceTierId}` : ""}`;
    if (rule.kind === "channel_tier") return `渠道 ${rule.channelId || "-"} / 模型 ${rule.channelModelId || "-"}`;
    return `逻辑模型 ${rule.logicalModelId || "-"}`;
}

function renderAdminPricing(view) {
    const kind = state.pricingKind;
    const channel = state.pricingChannels.find((item) => item.id === state.pricingBatchChannelId) || null;
    const visibleCount = state.pricingChannelModels.filter((model) => channel?.enabled && model.enabled).length;
    const pricedCount = state.pricingChannelModels.filter((model) => activeDefaultChannelTierRule(channel?.id || "", model.id)).length;
    const channelOptions = state.pricingChannels.length
        ? state.pricingChannels.map((item) => `<option value="${escapeHtml(item.id)}" ${item.id === state.pricingBatchChannelId ? "selected" : ""}>${escapeHtml(item.name || item.publicAlias || item.id)}</option>`).join("")
        : '<option value="">暂无渠道</option>';
    view.innerHTML = `
        <div class="page-head">
            <div>
                <h1 class="page-title">模型价格</h1>
                <p class="page-sub">先选渠道，再直接给渠道模型设置积分价格。画布只展示已上架且已设有效价的模型。</p>
            </div>
            <button class="btn btn-sm" data-action="refresh">刷新</button>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel pricing-workspace">
            <div class="pricing-workspace-head">
                <div>
                    <h2 class="panel-title" style="margin:0">选择渠道</h2>
                    <p class="page-sub">同一页查看模型能力、计费单位、当前价格和画布状态。</p>
                </div>
                <label class="field pricing-channel-picker"><span class="field-label">模型渠道</span><select class="select" id="pricing-workspace-channel" ${state.pricingChannels.length ? "" : "disabled"}>${channelOptions}</select></label>
            </div>
            <div class="metric-row model-summary">
                <div class="metric"><div class="metric-label">渠道模型</div><div class="metric-value">${state.pricingChannelModels.length}</div></div>
                <div class="metric"><div class="metric-label">画布可见</div><div class="metric-value">${visibleCount}</div></div>
                <div class="metric"><div class="metric-label">已设有效价</div><div class="metric-value">${pricedCount}</div></div>
            </div>
            ${renderPricingWorkspaceTable(channel)}
        </div>
        <details class="pricing-disclosure" id="pricing-batch-details">
            <summary><span>批量设置</span><span class="muted">给多个模型套用同一价格</span></summary>
            <div class="pricing-disclosure-body">${renderPricingBatchPanel()}</div>
        </details>
        <details class="pricing-disclosure" id="pricing-advanced-details">
            <summary><span>高级规则</span><span class="muted">逻辑模型 SKU、模型默认价、选择器、生效时间</span></summary>
            <div class="pricing-disclosure-body">
                <div class="btn-row" style="margin-bottom:14px">
                    ${["logical_sku", "channel_tier", "model"].map((item) => `<button class="btn btn-sm ${item === kind ? "btn-primary" : ""}" data-action="pricing-kind" data-kind="${item}">${pricingKindLabel(item)}</button>`).join("")}
                </div>
                <h2 class="panel-title" id="pricing-form-title">${pricingKindLabel(kind)}</h2>
                <form id="pricing-form">
                    <input type="hidden" name="id" />
                    <div class="form-grid">
                        <label class="field"><span class="field-label">名称（可选）</span><input class="input" name="name" /></label>
                        <label class="field"><span class="field-label">能力</span>
                            <select class="select" name="capability">
                                <option value="text">文本</option>
                                <option value="image">图片</option>
                                <option value="video">视频</option>
                                <option value="audio">音频</option>
                            </select>
                        </label>
                        <label class="field"><span class="field-label">计费方式</span>
                            <select class="select" name="billingMode">
                                <option value="per_second">每秒</option>
                                <option value="fixed_request">每次请求</option>
                                <option value="per_image">每张图</option>
                                <option value="per_1k_tokens">每千 Token</option>
                                <option value="per_megabyte">每 MB</option>
                            </select>
                        </label>
                        <label class="field"><span class="field-label">单价（积分 / 计费单位）</span><input class="input" name="unitPriceCredits" type="number" min="0.000001" step="0.000001" placeholder="例如 2.5" required /></label>
                        <div class="muted" id="pricing-unit-preview" style="align-self:end;padding-bottom:10px">折合 0 积分</div>
                        <div class="pricing-converter" data-rmb-converter>
                            <div class="pricing-converter-head"><strong>人民币成本换算积分</strong><span>可选，只用于计算，不会保存人民币价格</span></div>
                            <div class="pricing-converter-grid">
                                <label class="field"><span class="field-label">人民币成本 / 计费单位</span><input class="input" id="pricing-rmb-cost" name="rmbUnitCost" type="number" min="0" step="0.000001" placeholder="例如 0.002" /></label>
                                <label class="field"><span class="field-label">每 1 元折算积分</span><input class="input" id="pricing-exchange-rate" name="creditsPerYuan" type="number" min="0.000001" step="0.01" value="100" /></label>
                                <label class="field"><span class="field-label">平台倍率</span><input class="input" id="pricing-price-multiplier" name="priceMultiplier" type="number" min="0.000001" step="0.01" value="1" /></label>
                                <button class="btn btn-sm" type="button" data-action="apply-rmb-conversion" data-form="pricing-form">换算并填入单价</button>
                            </div>
                            <div class="muted pricing-converter-preview" id="pricing-rmb-preview">填写人民币成本后显示换算结果</div>
                        </div>
                        <label class="field"><span class="field-label">最低扣费（积分，支持小数）</span><input class="input" name="minimumChargeCredits" type="number" min="0" step="0.000001" value="1" /></label>
                        <label class="field" data-kind-field="model logical_sku"><span class="field-label">逻辑模型 ID</span><input class="input" name="logicalModelId" /></label>
                        <label class="field" data-kind-field="channel_tier"><span class="field-label">渠道 ID</span><input class="input" name="channelId" /></label>
                        <label class="field" data-kind-field="channel_tier"><span class="field-label">渠道模型 ID</span><input class="input" name="channelModelId" /></label>
                        <label class="field" data-kind-field="logical_sku"><span class="field-label">关联渠道价格档 ID（可选）</span><input class="input" name="channelModelPriceTierId" /></label>
                        <label class="field"><span class="field-label">选择器 JSON（如 {"quality":"720p"}）</span><textarea class="textarea" name="selector" rows="2" placeholder="{}"></textarea></label>
                        <label class="field"><span class="field-label">生效时间（留空立即生效）</span><input class="input" name="effectiveFrom" type="datetime-local" /></label>
                        <label class="field"><span class="field-label">失效时间（留空长期有效）</span><input class="input" name="effectiveTo" type="datetime-local" /></label>
                        <label class="field"><span class="field-label">排序（小的在前）</span><input class="input" name="sortOrder" type="number" step="1" value="0" /></label>
                        <label class="field"><span class="field-label">备注</span><input class="input" name="note" /></label>
                    </div>
                    <p class="page-sub" style="margin:0 0 12px">扣费公式：实际用量 × 单价，保留到 0.000001 积分；低于最低扣费时按最低扣费计算。文本按 Token、图片按张、视频按秒、固定请求按次。</p>
                    <div class="btn-row">
                        <label class="muted"><input type="checkbox" name="enabled" checked /> 启用</label>
                        <button class="btn btn-sm" type="button" data-action="reset-pricing">清空表单</button>
                        <button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>保存价格规则</button>
                    </div>
                </form>
                <h3 class="pricing-subtitle">${pricingKindLabel(kind)}（${state.pricingRules.length} 条）</h3>
                ${renderPricingRules()}
            </div>
        </details>`;
    applyPricingKindFields();
    if (kind !== "channel_tier") {
        const advanced = document.getElementById("pricing-advanced-details");
        if (advanced) advanced.open = true;
    }
    consumePricingPreset();
}

function canonicalPricingSelector(selector) {
    const normalized = {};
    Object.keys(selector || {}).sort().forEach((key) => { normalized[key] = selector[key]; });
    return JSON.stringify(normalized);
}

function findChannelTierRule(channelId, channelModelId, selector = {}) {
    const selectorKey = canonicalPricingSelector(selector);
    return state.pricingRules.find((rule) => rule.kind === "channel_tier"
        && rule.channelId === channelId
        && rule.channelModelId === channelModelId
        && canonicalPricingSelector(rule.selector) === selectorKey) || null;
}

function channelTierRulesForModel(channelId, channelModelId) {
    return state.pricingRules.filter((rule) => rule.kind === "channel_tier" && rule.channelId === channelId && rule.channelModelId === channelModelId);
}

function renderPricingBatchPanel() {
    if (!state.pricingChannels.length) {
        return `<div class="panel"><h2 class="panel-title">批量设置渠道模型价格</h2><div class="empty">还没有模型渠道。请先到「模型渠道」新增渠道并读取模型。</div></div>`;
    }
    const channelId = state.pricingBatchChannelId;
    const channelOptions = state.pricingChannels.map((item) => `<option value="${escapeHtml(item.id)}" ${item.id === channelId ? "selected" : ""}>${escapeHtml(item.name || item.publicAlias || item.id)}</option>`).join("");
    const channel = state.pricingChannels.find((item) => item.id === channelId);
    const visibleCount = state.pricingChannelModels.filter((model) => channel?.enabled && model.enabled).length;
    const pricedCount = state.pricingChannelModels.filter((model) => activeChannelTierRulesForModel(channelId, model.id).length).length;
    const rows = state.pricingChannelModels.map((model) => {
        const rules = channelTierRulesForModel(channelId, model.id);
        const defaultRule = rules.find((rule) => !Object.keys(rule.selector || {}).length) || null;
        return `<tr>
            <td><input type="checkbox" data-pricing-batch-model value="${escapeHtml(model.id)}" aria-label="选择 ${escapeHtml(model.displayName || model.modelKey || model.id)}" /></td>
            <td>${escapeHtml(model.displayName || model.modelKey || model.id)}<div class="muted">${escapeHtml(model.modelKey || model.id)}</div></td>
            <td>${model.capability ? escapeHtml(capabilityLabel(model.capability)) : '<span class="tag tag-warn">未设置</span>'}</td>
            <td>${channel?.enabled && model.enabled ? '<span class="tag tag-ok">画布可见</span>' : '<span class="tag tag-muted">画布隐藏</span>'}</td>
            <td>${defaultRule ? escapeHtml(formatMicrocredits(defaultRule.unitPriceMicrocredits)) : '<span class="muted">未设置</span>'}</td>
            <td class="muted">${escapeHtml(upstreamPricingText(model))}</td>
            <td>${rules.length ? `<span class="tag tag-ok">${rules.length} 条</span>` : '<span class="tag tag-muted">无规则</span>'}</td>
            <td>${model.enabled ? '<span class="tag tag-ok">启用</span>' : '<span class="tag tag-muted">停用</span>'}</td>
            <td><div class="btn-row"><button class="btn btn-sm" data-action="quick-set-channel-model-price" data-id="${escapeHtml(model.id)}">一键 1 积分</button><button class="btn btn-sm" data-action="set-channel-model-price" data-id="${escapeHtml(model.id)}">设价</button></div></td>
        </tr>`;
    }).join("");
    return `
        <div class="panel">
            <div class="page-head" style="margin-bottom:12px">
                <div>
                    <h2 class="panel-title" style="margin:0">批量设置渠道模型价格</h2>
                    <p class="page-sub">选择渠道和模型，一次填写统一价格。已有同选择器规则会更新，没有规则时自动新建。</p>
                    <p class="page-sub">选择渠道和模型后一次套用价格；表格里显示「能力未设置」时，请在下方统一选择本次要使用的模型能力。</p>
                </div>
                <label class="field" style="min-width:260px;margin:0"><span class="field-label">渠道</span><select class="select" id="pricing-batch-channel">${channelOptions}</select></label>
            </div>
            <div class="metric-row model-summary">
                <div class="metric"><div class="metric-label">渠道模型</div><div class="metric-value">${state.pricingChannelModels.length}</div></div>
                <div class="metric"><div class="metric-label">画布可见</div><div class="metric-value">${visibleCount}</div></div>
                <div class="metric"><div class="metric-label">已设有效价</div><div class="metric-value">${pricedCount}</div></div>
            </div>
            <form id="pricing-batch-form">
                <div class="form-grid">
                    <label class="field"><span class="field-label">模型能力</span>
                        <select class="select" name="capability">
                            <option value="">按模型自身能力（缺失时按文本）</option>
                            <option value="text">文本</option>
                            <option value="image">图片</option>
                            <option value="video">视频</option>
                            <option value="audio">音频</option>
                        </select>
                    </label>
                    <label class="field"><span class="field-label">计费方式</span>
                        <select class="select" name="billingMode">
                            <option value="per_second">每秒</option>
                            <option value="fixed_request">每次请求</option>
                            <option value="per_image">每张图</option>
                            <option value="per_1k_tokens">每千 Token</option>
                            <option value="per_megabyte">每 MB</option>
                        </select>
                    </label>
                    <label class="field"><span class="field-label">单价（积分 / 计费单位）</span><input class="input" name="unitPriceCredits" type="number" min="0.000001" step="0.000001" placeholder="例如 2.5" required /></label>
                    <div class="muted" id="pricing-batch-unit-preview" style="align-self:end;padding-bottom:10px">填写后显示扣费说明</div>
                    <div class="pricing-converter" data-rmb-converter>
                        <div class="pricing-converter-head"><strong>人民币成本换算积分</strong><span>本批次共用，只用于计算积分单价</span></div>
                        <div class="pricing-converter-grid">
                            <label class="field"><span class="field-label">人民币成本 / 计费单位</span><input class="input" id="batch-rmb-cost" name="rmbUnitCost" type="number" min="0" step="0.000001" placeholder="例如 0.002" /></label>
                            <label class="field"><span class="field-label">每 1 元折算积分</span><input class="input" id="batch-exchange-rate" name="creditsPerYuan" type="number" min="0.000001" step="0.01" value="100" /></label>
                            <label class="field"><span class="field-label">平台倍率</span><input class="input" id="batch-price-multiplier" name="priceMultiplier" type="number" min="0.000001" step="0.01" value="1" /></label>
                            <button class="btn btn-sm" type="button" data-action="apply-rmb-conversion" data-form="pricing-batch-form">换算并填入单价</button>
                        </div>
                        <div class="muted pricing-converter-preview" id="batch-rmb-preview">填写人民币成本后显示换算结果</div>
                    </div>
                    <label class="field"><span class="field-label">最低扣费（积分，支持小数）</span><input class="input" name="minimumChargeCredits" type="number" min="0" step="0.000001" value="1" /></label>
                    <label class="field"><span class="field-label">生效时间（留空立即生效）</span><input class="input" name="effectiveFrom" type="datetime-local" /></label>
                    <label class="field" style="grid-column:1/-1"><span class="field-label">选择器 JSON（所有选中模型共用，如 {"quality":"720p"}）</span><textarea class="textarea" name="selector" rows="2" placeholder="{}"></textarea></label>
                </div>
                <div class="btn-row pricing-batch-actions">
                    <label class="muted"><input type="checkbox" name="enabled" checked /> 启用</label>
                    <label class="muted"><input type="checkbox" name="skipExisting" /> 跳过已有同选择器价格的模型</label>
                    <span class="muted" id="pricing-batch-count">已选 0 个模型</span>
                    <button class="btn btn-sm" type="button" data-action="select-all-pricing-models">全选模型</button>
                    <button class="btn btn-sm" type="button" data-action="clear-pricing-models">取消全选</button>
                    <button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>应用价格到选中模型</button>
                </div>
                ${state.pricingChannelModels.length ? `<div class="table-wrap pricing-batch-table"><table>
                    <thead><tr><th><input type="checkbox" id="pricing-batch-select-all" aria-label="全选模型" /></th><th>模型</th><th>能力</th><th>画布</th><th class="num">默认价（积分）</th><th>上游参考</th><th>已有规则</th><th>状态</th><th></th></tr></thead>
                    <tbody>${rows}</tbody>
                </table></div>` : '<div class="empty">这个渠道还没有模型，请先在上游读取或新增模型。</div>'}
            </form>
        </div>`;
}

function updatePricingBatchSelection() {
    const selected = document.querySelectorAll("[data-pricing-batch-model]:checked").length;
    const count = document.getElementById("pricing-batch-count");
    if (count) count.textContent = `已选 ${selected} 个模型`;
    const selectAll = document.getElementById("pricing-batch-select-all");
    const total = document.querySelectorAll("[data-pricing-batch-model]").length;
    if (selectAll) selectAll.checked = total > 0 && selected === total;
}
function applyPricingKindFields() {
    const kind = state.pricingKind;
    document.querySelectorAll("[data-kind-field]").forEach((field) => {
        const kinds = (field.dataset.kindField || "").split(/\s+/u);
        field.hidden = !kinds.includes(kind);
    });
}

function renderPricingRules() {
    if (!state.pricingRules.length) return `<div class="empty">当前分类还没有价格规则</div>`;
    return `
        <div class="table-wrap">
            <table>
                <thead><tr><th>名称</th><th>适用范围</th><th>选择器</th><th>计费</th><th class="num">单价（积分）</th><th class="num">最低扣费</th><th>生效区间</th><th>状态</th><th></th></tr></thead>
                <tbody>
                    ${state.pricingRules.map((rule) => `<tr>
                        <td>${escapeHtml(rule.name || pricingKindLabel(rule.kind))}</td>
                        <td class="muted">${escapeHtml(pricingRuleScope(rule))}</td>
                        <td class="muted">${escapeHtml(pricingSelectorText(rule.selector))}</td>
                        <td>${escapeHtml(billingModeLabel(rule.billingMode))}</td>
                        <td class="num">${escapeHtml(formatMicrocredits(rule.unitPriceMicrocredits))}</td>
                        <td class="num">${escapeHtml(formatMicrocredits(rule.minimumChargeCredits))}</td>
                        <td class="muted">${escapeHtml(formatTime(rule.effectiveFrom))}${rule.effectiveTo ? ` 至 ${escapeHtml(formatTime(rule.effectiveTo))}` : " 起长期"}</td>
                        <td>${rule.enabled ? '<span class="tag tag-ok">启用</span>' : '<span class="tag tag-muted">停用</span>'}</td>
                        <td><div class="btn-row"><button class="btn btn-sm" data-action="edit-pricing" data-id="${escapeHtml(rule.id)}">编辑</button><button class="btn btn-sm btn-danger" data-action="delete-pricing" data-id="${escapeHtml(rule.id)}">删除</button></div></td>
                    </tr>`).join("")}
                </tbody>
            </table>
        </div>`;
}

function pricingEditorDefaultBillingMode(capability) {
    if (capability === "image") return "per_image";
    if (capability === "video" || capability === "audio") return "per_second";
    return "fixed_request";
}

function pricingEditorBillingModes(capability, current) {
    const modes = capability === "image"
        ? ["per_image", "fixed_request"]
        : capability === "video"
          ? ["per_second", "fixed_request"]
          : capability === "audio"
            ? ["per_second", "per_megabyte", "fixed_request"]
            : ["fixed_request", "per_1k_tokens"];
    const selected = String(current || "").trim();
    if (selected && !modes.includes(selected)) modes.push(selected);
    return modes;
}

function pricingEditorChargeText(form) {
    const mode = form?.billingMode?.value || "fixed_request";
    const minimumInput = Number(form?.minimumChargeCredits?.value || 0);
    const minimum = Number.isFinite(minimumInput) && minimumInput > 0 ? Math.round(minimumInput * 1_000_000) : 0;
    const unit = pricingBillingUnit(mode);
    const rows = Array.from(form?.querySelectorAll?.("[data-pricing-editor-row]") || [])
        .filter((row) => row.querySelector('[name="pricingEnabled"]')?.checked);
    const priced = rows.map((row) => {
        const value = Number(row.querySelector('[name="resolutionPrice"]')?.value || 0);
        return { label: row.dataset.pricingLabel || "通用价", value };
    }).filter((row) => Number.isFinite(row.value) && row.value > 0);
    if (!priced.length) return "勾选需要的档位，每行填写积分单价，可一次保存多档。";
    const summary = priced.map((row) => `${row.label}=${formatMicrocredits(Math.round(row.value * 1_000_000))}`).join("，");
    const floor = minimum > 0 ? `；每档单次最低扣 ${formatMicrocredits(minimum)} 积分` : "";
    return `将保存 ${priced.length} 档：${summary} 积分/${unit}${floor}。`;
}

function pricingEditorRows(channelId, model, capability, preset) {
    const values = [{ value: "", label: "通用价（兜底）" }].concat(pricingResolutionOptions(capability).map((value) => ({ value, label: value })));
    const presetQuality = String(preset?.selector?.quality || "");
    return values.map((row) => {
        const selector = row.value ? { quality: row.value } : {};
        const existing = findChannelTierRule(channelId, model.id, selector);
        const presetSelected = Boolean(preset && presetQuality === row.value);
        const price = existing ? formatCreditInput(existing.unitPriceMicrocredits) : presetSelected ? String(preset.unitPriceCredits || "") : "";
        return {
            ...row,
            existing,
            checked: Boolean(existing || presetSelected),
            price,
        };
    });
}

function openPricingEditorModal(channelModelId, preset = null) {
    const channelId = preset?.channelId || state.pricingBatchChannelId;
    const model = state.pricingChannelModels.find((item) => item.id === channelModelId)
        || state.channelModels.find((item) => item.id === channelModelId);
    const channel = state.pricingChannels.find((item) => item.id === channelId) || selectedChannel();
    if (!model || !channelId) return;
    const existingRules = channelTierRulesForModel(channelId, model.id);
    const existing = existingRules.find((item) => !Object.keys(item.selector || {}).length) || existingRules[0] || null;
    const capability = preset?.capability || existing?.capability || model.capability || "text";
    const billingMode = preset?.billingMode || existing?.billingMode || pricingEditorDefaultBillingMode(capability);
    const billingModes = pricingEditorBillingModes(capability, billingMode);
    const minimum = existing?.minimumChargeCredits ? formatCreditInput(existing.minimumChargeCredits) : "1";
    const modelName = model.displayName || model.modelKey || model.id;
    const rows = pricingEditorRows(channelId, model, capability, preset);
    closePricingEditorModal();
    document.body.insertAdjacentHTML("beforeend", `
        <div class="modal-backdrop" id="pricing-editor-modal">
            <section class="modal pricing-editor-modal" role="dialog" aria-modal="true" aria-labelledby="pricing-editor-title">
                <div class="modal-head">
                    <div>
                        <h2 class="panel-title" id="pricing-editor-title">${existing ? "修改价格" : "设置价格"}</h2>
                        <p class="modal-sub">${escapeHtml(modelName)} · ${escapeHtml(channel?.name || channel?.publicAlias || channelId)}</p>
                    </div>
                    <button class="btn btn-sm" type="button" data-action="close-pricing-editor">关闭</button>
                </div>
                <form id="pricing-editor-form">
                    <div class="modal-body">
                        <input type="hidden" name="id" value="${escapeHtml(existing?.id || "")}" />
                        <input type="hidden" name="channelId" value="${escapeHtml(channelId)}" />
                        <input type="hidden" name="channelModelId" value="${escapeHtml(model.id)}" />
                        <input type="hidden" name="capability" value="${escapeHtml(capability)}" />
                        <input type="hidden" name="note" value="${escapeHtml(preset?.note || existing?.note || "快速定价")}" />
                        <div class="pricing-editor-overview">
                            <div class="pricing-editor-model">
                                <span class="pricing-editor-capability">${escapeHtml(capabilityLabel(capability))}模型</span>
                                <strong>${escapeHtml(modelName)}</strong>
                                <small>${escapeHtml(channel?.name || channel?.publicAlias || channelId)}</small>
                            </div>
                            <span class="tag ${channel?.enabled && model.enabled ? "tag-ok" : "tag-muted"}">${channel?.enabled && model.enabled ? "画布可见" : "画布隐藏"}</span>
                        </div>
                        <div class="pricing-editor-primary">
                            <label class="field">
                                <span class="field-label">计费方式</span>
                                <select class="select" id="pricing-editor-billing-mode" name="billingMode">
                                    ${billingModes.map((mode) => `<option value="${escapeHtml(mode)}" ${mode === billingMode ? "selected" : ""}>${escapeHtml(billingModeLabel(mode))}</option>`).join("")}
                                </select>
                            </label>
                            <label class="field">
                                <span class="field-label">最低扣费（积分，支持小数）</span>
                                <input class="input" name="minimumChargeCredits" type="number" min="0" step="0.000001" value="${escapeHtml(String(minimum))}" />
                            </label>
                        </div>
                        <div class="pricing-resolution-picker">
                            <div class="pricing-resolution-picker-head">
                                <strong>按分辨率设置积分</strong>
                                <span>勾选多个档位，填写价格后一次保存</span>
                            </div>
                            <div class="table-wrap">
                                <table>
                                    <thead><tr><th>启用</th><th>价格档位</th><th>积分单价</th></tr></thead>
                                    <tbody>
                                        ${rows.map((row) => `<tr data-pricing-editor-row data-pricing-label="${escapeHtml(row.label)}" data-resolution="${escapeHtml(row.value)}">
                                            <td><input type="checkbox" name="pricingEnabled" ${row.checked ? "checked" : ""} aria-label="启用 ${escapeHtml(row.label)}" /></td>
                                            <td><strong>${escapeHtml(row.label)}</strong></td>
                                            <td><input class="input" name="resolutionPrice" type="number" min="0.000001" step="0.000001" value="${escapeHtml(row.price)}" placeholder="例如 2" /></td>
                                        </tr>`).join("")}
                                    </tbody>
                                </table>
                            </div>
                        </div>
                        <details class="pricing-editor-converter" data-rmb-converter>
                            <summary><span>用人民币报价换算积分</span><small>换算后填入所有已勾选档位</small></summary>
                            <div class="pricing-converter-grid">
                                <label class="field"><span class="field-label">上游人民币报价</span><input class="input" name="rmbUnitCost" type="number" inputmode="decimal" min="0" step="any" placeholder="${billingMode === "per_1k_tokens" ? "例如 2（元/百万 Token）" : "例如 0.02"}" /></label>
                                <label class="field"><span class="field-label">报价单位</span><select class="select" name="rmbQuoteUnit"><option value="per_unit" ${billingMode !== "per_1k_tokens" ? "selected" : ""}>按当前计费单位</option><option value="per_million_tokens" ${billingMode === "per_1k_tokens" ? "selected" : ""}>元 / 百万 Token</option><option value="per_1k_tokens">元 / 千 Token</option></select></label>
                                <label class="field"><span class="field-label">1 元折算积分（默认 100）</span><input class="input" name="creditsPerYuan" type="number" inputmode="decimal" min="0" step="any" value="100" /></label>
                                <button class="btn btn-sm" type="button" data-action="apply-rmb-conversion" data-form="pricing-editor-form">换算并填入已勾选档位</button>
                            </div>
                            <div class="muted pricing-converter-preview" id="pricing-editor-rmb-preview">填写上游报价后显示换算结果</div>
                        </details>
                        <div class="pricing-editor-charge-summary" id="pricing-editor-charge-summary">勾选需要的档位，每行填写积分单价，可一次保存多档。</div>
                        <p class="pricing-editor-footnote">取消勾选已有档位会将其停用；未勾选且没有价格的档位不会创建规则。</p>
                    </div>
                    <div class="modal-foot">
                        <span class="muted">生效时间和复杂条件可到页面下方“高级规则”维护。</span>
                        <div class="btn-row">
                            <button class="btn btn-sm" type="button" data-action="close-pricing-editor">取消</button>
                            <button class="btn btn-primary" type="submit" ${state.busy ? "disabled" : ""}>保存价格</button>
                        </div>
                    </div>
                </form>
            </section>
        </div>`);
    document.body.classList.add("modal-open");
    updatePricingUnitPreview();
    requestAnimationFrame(() => document.querySelector('#pricing-editor-form [name="resolutionPrice"]')?.focus());
}

function closePricingEditorModal() {
    document.getElementById("pricing-editor-modal")?.remove();
    if (!document.querySelector(".modal-backdrop:not([hidden])")) document.body.classList.remove("modal-open");
}

function fillPricingForm(item) {
    const form = document.getElementById("pricing-form");
    if (!form) return;
    form.id.value = item ? item.id : "";
    form.name.value = item ? item.name || "" : "";
    form.capability.value = item ? item.capability || "video" : "video";
    form.billingMode.value = item ? item.billingMode || recommendedBillingMode(form.capability.value) : recommendedBillingMode(form.capability.value);
    form.unitPriceCredits.value = item ? formatCreditInput(item.unitPriceMicrocredits) : "";
    form.minimumChargeCredits.value = item?.minimumChargeCredits ? formatCreditInput(item.minimumChargeCredits) : "1";
    form.logicalModelId.value = item ? item.logicalModelId || "" : "";
    form.channelId.value = item ? item.channelId || "" : "";
    form.channelModelId.value = item ? item.channelModelId || "" : "";
    form.channelModelPriceTierId.value = item ? item.channelModelPriceTierId || "" : "";
    form.selector.value = item && item.selector ? JSON.stringify(item.selector) : "";
    form.effectiveFrom.value = item && item.effectiveFrom ? toDateTimeLocal(item.effectiveFrom) : "";
    form.effectiveTo.value = item && item.effectiveTo ? toDateTimeLocal(item.effectiveTo) : "";
    form.sortOrder.value = item ? item.sortOrder || 0 : 0;
    form.note.value = item ? item.note || "" : "";
    form.enabled.checked = item ? item.enabled !== false : true;
    updatePricingUnitPreview();
    const title = document.getElementById("pricing-form-title");
    if (title) title.textContent = item ? `编辑${pricingKindLabel(state.pricingKind)}：${item.name || item.id}` : `新增${pricingKindLabel(state.pricingKind)}`;
}

function updatePricingUnitPreview() {
    for (const [formID, previewID] of [["pricing-form", "pricing-unit-preview"], ["pricing-batch-form", "pricing-batch-unit-preview"], ["pricing-editor-form", "pricing-editor-charge-summary"]]) {
        const form = document.getElementById(formID);
        const preview = document.getElementById(previewID);
        if (!form || !preview) continue;
        const credits = Number(form.unitPriceCredits?.value);
        preview.textContent = formID === "pricing-editor-form"
            ? pricingEditorChargeText(form)
            : Number.isFinite(credits) && credits > 0
              ? `每${pricingBillingUnit(form.billingMode?.value)}扣 ${formatMicrocredits(Math.round(credits * 1_000_000))} 积分`
              : "填写积分单价后显示扣费说明";
        updatePricingRmbPreview(form);
    }
}

function pricingRmbConversion(form) {
    const rmbCost = Number(form?.elements?.namedItem("rmbUnitCost")?.value || 0);
    const creditsPerYuan = Number(form?.elements?.namedItem("creditsPerYuan")?.value || 100);
    const multiplier = Number(form?.elements?.namedItem("priceMultiplier")?.value || 1);
    const billingMode = form?.elements?.namedItem("billingMode")?.value || "fixed_request";
    const quoteUnit = form?.elements?.namedItem("rmbQuoteUnit")?.value || "per_unit";
    const normalizedCost = normalizeRmbCost(rmbCost, billingMode, quoteUnit);
    if (!Number.isFinite(creditsPerYuan) || creditsPerYuan <= 0 || !Number.isFinite(multiplier) || multiplier <= 0 || normalizedCost <= 0) return null;
    const credits = creditsFromCurrency(normalizedCost, { creditsPerYuan, multiplier });
    return { rmbCost, normalizedCost, creditsPerYuan, multiplier, billingMode, quoteUnit, credits };
}

function updatePricingRmbPreview(form) {
    if (!form) return;
    const preview = form.querySelector("[id$='rmb-preview']");
    if (!preview) return;
    const conversion = pricingRmbConversion(form);
    if (!conversion) {
        preview.textContent = "填写上游人民币报价后显示换算结果";
        return;
    }
    const tokenNote = conversion.billingMode === "per_1k_tokens" && conversion.quoteUnit === "per_million_tokens"
        ? ` 已自动折算为 ${formatMicrocredits(Math.round(conversion.normalizedCost * 1_000_000))} 元 / 千 Token。`
        : "";
    preview.textContent = `换算结果：${formatMicrocredits(Math.round(conversion.credits * 1_000_000))} 积分 / ${pricingBillingUnit(form.elements.namedItem("billingMode")?.value)}。${tokenNote}`;
}

function applyPricingRmbConversion(form) {
    const conversion = pricingRmbConversion(form);
    if (!form || !conversion?.credits) throw new Error("请填写有效的人民币报价");
    const converted = formatCreditInput(Math.round(conversion.credits * 1_000_000));
    if (form.id === "pricing-editor-form") {
        const rows = Array.from(form.querySelectorAll("[data-pricing-editor-row]"))
            .filter((row) => row.querySelector('[name="pricingEnabled"]')?.checked);
        if (!rows.length) throw new Error("请先勾选至少一个价格档位");
        rows.forEach((row) => { row.querySelector('[name="resolutionPrice"]').value = converted; });
    } else {
        form.unitPriceCredits.value = converted;
    }
    updatePricingUnitPreview();
    updatePricingRmbPreview(form);
}

function pricingBillingUnit(mode) {
    switch (mode) {
        case "fixed_request": return "次请求";
        case "per_second": return "秒";
        case "per_image": return "张图";
        case "per_1k_tokens": return "千 Token";
        case "per_megabyte": return "MB";
        default: return "计费单位";
    }
}

function creditAmountToMicrocredits(value, { allowNegative = false } = {}) {
    const credits = Number(value);
    if (!Number.isFinite(credits)) throw new Error("请输入有效的积分数");
    if (!allowNegative && credits < 0) throw new Error("积分不能为负数");
    const microcredits = Math.round(credits * 1_000_000);
    if (!Number.isSafeInteger(microcredits) || (!allowNegative && microcredits < 0)) {
        throw new Error("积分数量超出可设置范围");
    }
    return microcredits;
}

function creditPriceToMicrocredits(value) {
    const microcredits = creditAmountToMicrocredits(value);
    if (microcredits <= 0) throw new Error("请输入大于 0 的积分单价");
    return microcredits;
}

function formatCreditInput(microcredits) {
    const value = Number(microcredits) / 1_000_000;
    return Number.isFinite(value) ? value.toFixed(6).replace(/0+$/u, "").replace(/\.$/u, "") : "";
}

function toDateTimeLocal(value) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "";
    const pad = (number) => String(number).padStart(2, "0");
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

async function boot() {
    clearFlash();
    try {
        const authenticated = await loadSession();
        if (authenticated) {
            await reloadCurrent().catch((error) => {
                state.error = error instanceof Error ? error.message : "加载失败";
            });
        }
    } catch (error) {
        state.error = error instanceof Error ? error.message : "无法连接后端";
    }
    render();
}

boot();

function recommendedBillingMode(capability) {
    if (capability === "image") return "fixed_request";
    if (capability === "video" || capability === "audio") return "per_second";
    if (capability === "text") return "per_1k_tokens";
    return "fixed_request";
}

function applyRecommendedBillingMode(form) {
    if (!form?.billingMode || !form?.capability) return;
    form.billingMode.value = recommendedBillingMode(form.capability.value);
    updatePricingUnitPreview();
}

function parsePricingObject(value) {
    if (!value) return null;
    if (typeof value === "object") return value;
    try { return JSON.parse(value); } catch { return null; }
}

function upstreamPricingObject(item) {
    return parsePricingObject(item?.upstreamPricing)
        || parsePricingObject(item?.videoPricing)
        || parsePricingObject(item?.upstreamPricingJson)
        || null;
}

function upstreamPricingRows(item) {
    const pricing = upstreamPricingObject(item);
    if (!pricing || !pricing.rates || typeof pricing.rates !== "object") return [];
    const currency = String(pricing.currency || "CNY").toUpperCase();
    const mode = String(pricing.mode || "").toLowerCase();
    const rows = [];
    for (const [quality, rawValue] of Object.entries(pricing.rates)) {
        const detail = rawValue && typeof rawValue === "object" ? rawValue : { output: rawValue };
        const output = Number(detail.output);
        if (!Number.isFinite(output) || output <= 0) continue;
        const tokens = mode === "tokens";
        rows.push({
            quality,
            currency,
            sourceAmount: output,
            billingMode: tokens ? "per_1k_tokens" : "per_second",
            billableAmount: tokens ? output / 1000 : output,
            sourceLabel: tokens ? "百万 Token" : "秒",
        });
    }
    return rows;
}

function upstreamPricingText(item) {
    const rows = upstreamPricingRows(item);
    if (!rows.length) return "上游未返回价格";
    return rows.slice(0, 2).map((row) => {
        const symbol = row.currency === "CNY" ? "¥" : `${row.currency} `;
        return `${row.quality}: ${symbol}${row.sourceAmount}/${row.sourceLabel}`;
    }).join("；") + (rows.length > 2 ? ` 等 ${rows.length} 档` : "");
}

function normalizeCatalogKey(value) {
    return String(value || "").trim().replace(/^models\//u, "");
}

function channelModelForCatalogItem(item) {
    const key = normalizeCatalogKey(item?.id);
    return state.channelModels.find((model) => normalizeCatalogKey(model.modelKey) === key || normalizeCatalogKey(model.providerModelKey) === key) || null;
}

function inferCatalogCapability(item, model = null) {
    const explicit = String(item?.modelType || model?.capability || "").toLowerCase();
    if (["text", "image", "video", "audio"].includes(explicit)) return explicit;
    const value = `${item?.id || ""} ${(item?.supportedEndpointTypes || []).join(" ")}`.toLowerCase();
    if (/video|seedance|veo|t2v|i2v/u.test(value)) return "video";
    if (/image|flux|banana/u.test(value)) return "image";
    if (/audio|speech|tts|music/u.test(value)) return "audio";
    return "text";
}

function catalogConversionValues() {
    const creditsPerYuan = Number(document.getElementById("catalog-exchange-rate")?.value || 100);
    const multiplier = Number(document.getElementById("catalog-price-multiplier")?.value || 1);
    return {
        creditsPerYuan: Number.isFinite(creditsPerYuan) && creditsPerYuan > 0 ? creditsPerYuan : 100,
        multiplier: Number.isFinite(multiplier) && multiplier > 0 ? multiplier : 1,
    };
}

function creditsFromCurrency(amount, conversion) {
    const value = Number(amount) * conversion.creditsPerYuan * conversion.multiplier;
    if (!Number.isFinite(value) || value <= 0) return 0;
    return Math.max(0.000001, Math.round(value * 1_000_000) / 1_000_000);
}

function renderCatalogQuoteRows(item) {
    const model = channelModelForCatalogItem(item);
    const capability = inferCatalogCapability(item, model);
    const rows = upstreamPricingRows(item);
    if (!rows.length) {
        return `<tr><td colspan="5" class="muted">上游没有返回可用价格，请回到渠道模型表点「设价」手工填写积分单价。</td></tr>`;
    }
    return rows.map((row) => {
        const symbol = row.currency === "CNY" ? "¥" : `${row.currency} `;
        return `<tr>
            <td>${escapeHtml(item.displayName || item.id)}<div class="muted">${escapeHtml(item.id)}</div></td>
            <td>${escapeHtml(capabilityLabel(capability))}</td>
            <td>${escapeHtml(row.quality)}</td>
            <td>${escapeHtml(symbol)}${escapeHtml(String(row.sourceAmount))} / ${escapeHtml(row.sourceLabel)}<div class="muted" data-quote-credits data-rmb="${escapeHtml(String(row.billableAmount))}">填写兑换比例后换算</div></td>
            <td><button class="btn btn-sm ${model ? "btn-primary" : ""}" data-action="apply-upstream-price" data-model-id="${escapeHtml(model?.id || "")}" data-capability="${escapeHtml(capability)}" data-billing-mode="${escapeHtml(row.billingMode)}" data-rmb="${escapeHtml(String(row.billableAmount))}" data-quality="${escapeHtml(row.quality)}" ${model ? "" : "disabled"}>${model ? "用此价设价" : "先拉取目录"}</button></td>
        </tr>`;
    }).join("");
}

function openChannelCatalogModal() {
    closeChannelCatalogModal();
    const channel = state.channels.find((item) => item.id === state.channelCatalogChannelId);
    const body = state.channelCatalog.length
        ? state.channelCatalog.map(renderCatalogQuoteRows).join("")
        : `<tr><td colspan="5" class="empty">上游没有返回模型目录</td></tr>`;
    document.body.insertAdjacentHTML("beforeend", `
        <div class="modal-backdrop" id="channel-catalog-modal">
            <section class="modal modal-wide" role="dialog" aria-modal="true" aria-labelledby="channel-catalog-title">
                <div class="modal-head">
                    <div>
                        <h2 class="panel-title" id="channel-catalog-title">${escapeHtml(channel?.name || "上游")} 报价</h2>
                        <p class="modal-sub">这里读取的是上游人民币成本，不会直接当积分。设置“每元兑换多少积分”和平台倍率后再写入价格规则。</p>
                    </div>
                    <button class="btn btn-sm" type="button" data-action="close-channel-catalog">关闭</button>
                </div>
                <div class="modal-body">
                    <div class="form-grid" style="margin-bottom:16px">
                        <label class="field"><span class="field-label">每 1 元成本兑换积分</span><input class="input" id="catalog-exchange-rate" type="number" min="0.000001" step="0.01" value="100" /></label>
                        <label class="field"><span class="field-label">平台倍率</span><input class="input" id="catalog-price-multiplier" type="number" min="0.000001" step="0.01" value="1" /></label>
                    </div>
                    <div class="table-wrap"><table>
                        <thead><tr><th>模型</th><th>能力</th><th>上游档位</th><th>上游成本与换算</th><th></th></tr></thead>
                        <tbody>${body}</tbody>
                    </table></div>
                    <p class="page-sub" style="margin:14px 0 0">公式：人民币单价 × 每元兑换积分 × 平台倍率。Token 报价会先从“每百万 Token”换算成“每千 Token”。</p>
                </div>
            </section>
        </div>`);
    document.body.classList.add("modal-open");
    updateCatalogQuoteConversions();
}

function closeChannelCatalogModal() {
    document.getElementById("channel-catalog-modal")?.remove();
    if (!document.getElementById("channel-model-modal") || document.getElementById("channel-model-modal").hidden) {
        document.body.classList.remove("modal-open");
    }
}

function updateCatalogQuoteConversions() {
    const conversion = catalogConversionValues();
    document.querySelectorAll("[data-quote-credits][data-rmb]").forEach((node) => {
        const credits = creditsFromCurrency(node.dataset.rmb, conversion);
        node.textContent = `约 ${formatMicrocredits(Math.round(credits * 1_000_000))} 积分`;
    });
}

function applyUpstreamPrice(target) {
    const model = state.channelModels.find((item) => item.id === target.dataset.modelId);
    if (!model) return;
    const conversion = catalogConversionValues();
    const credits = creditsFromCurrency(target.dataset.rmb, conversion);
    state.pricingKind = "channel_tier";
    state.pricingPreset = {
        channelId: state.selectedChannelId,
        channelModelId: model.id,
        capability: target.dataset.capability,
        billingMode: target.dataset.billingMode,
        selector: target.dataset.quality ? { quality: target.dataset.quality } : {},
        unitPriceCredits: credits,
        name: `${model.displayName || model.modelKey || model.id} ${target.dataset.quality || ""} 渠道价`,
        note: `按上游人民币报价换算：1 元 = ${conversion.creditsPerYuan} 积分，倍率 ${conversion.multiplier}`,
    };
    closeChannelCatalogModal();
    goToPricingPage();
}

function openPricingForChannelModel(modelId) {
    const model = state.pricingChannelModels.find((item) => item.id === modelId)
        || state.channelModels.find((item) => item.id === modelId);
    if (!model) return;
    state.pricingKind = "channel_tier";
    state.pricingPreset = {
        channelId: currentPath() === "/admin/pricing" ? state.pricingBatchChannelId : state.selectedChannelId || state.pricingBatchChannelId,
        channelModelId: model.id,
        capability: model.capability || "video",
        billingMode: pricingEditorDefaultBillingMode(model.capability || "video"),
        selector: {},
        unitPriceCredits: "",
        name: `${model.displayName || model.modelKey || model.id} 渠道价`,
        note: "",
    };
    goToPricingPage();
}

async function applyQuickChannelModelPrice(model) {
    if (!model) return;
    const channelId = currentPath() === "/admin/pricing"
        ? state.pricingBatchChannelId
        : state.selectedChannelId || state.pricingBatchChannelId;
    if (!channelId) return;
    const capability = model.capability || "text";
    const billingMode = pricingEditorDefaultBillingMode(capability);
    const modeLabel = billingModeLabel(billingMode);
    const modelName = model.displayName || model.modelKey || model.id;
    if (!window.confirm(`为“${modelName}”创建通用价？\n计费：${modeLabel}\n单价：1 积分\n最低扣费：1 积分\n将覆盖该模型原有的通用价规则。`)) return;
    await withBusy(async () => {
        const existing = findChannelTierRule(channelId, model.id, {});
        await savePricing("channel_tier", existing?.id || "", {
            name: existing?.name || `${modelName} 一键通用价`,
            channelId,
            channelModelId: model.id,
            capability,
            billingMode,
            selector: {},
            unitPriceMicrocredits: 1_000_000,
            minimumChargeMicrocredits: 1_000_000,
            enabled: true,
            sortOrder: existing?.sortOrder || 0,
            effectiveFrom: new Date().toISOString(),
            effectiveTo: null,
            note: "一键通用价：1 积分",
        });
        await loadPricing("channel_tier");
        state.notice = `${modelName} 已设置通用价：${modeLabel}，1 积分`;
    });
}

function goToPricingPage() {
    if (currentPath() === "/admin/pricing") {
        withBusy(() => loadPricingWorkspace("channel_tier"));
        return;
    }
    navigate("/admin/pricing");
}

function consumePricingPreset() {
    const preset = state.pricingPreset;
    if (!preset || state.pricingKind !== "channel_tier") return;
    state.pricingPreset = null;
    openPricingEditorModal(preset.channelModelId, preset);
}
function renderChannelList() {
    if (!state.channels.length) return '<div class="empty">还没有渠道，点上方「新增渠道」开始。</div>';
    return `<div class="channel-list" id="channel-list">
        ${state.channels.map((item) => {
            const active = item.id === state.selectedChannelId;
            const modelCount = (item.models || []).length;
            return `<button class="channel-list-item ${active ? "is-active" : ""}" type="button" data-action="select-channel" data-id="${escapeHtml(item.id)}" ${active ? 'aria-current="true"' : ""}>
                <span class="channel-list-main">
                    <span class="channel-list-name">${escapeHtml(item.name || item.id)}</span>
                    <span class="channel-list-alias">${escapeHtml(item.publicAlias || item.baseUrl || "未设置对外别名")}</span>
                </span>
                <span class="channel-list-meta">
                    <span>${modelCount} 模型</span>
                    ${item.enabled ? '<span class="tag tag-ok">启用</span>' : '<span class="tag tag-muted">停用</span>'}${item.hasApiKey ? '' : '<span class="tag tag-warn">无密钥</span>'}
                </span>
            </button>`;
        }).join("")}</div>`;
}
function filterChannelModels() {
    const keyword = state.channelModelKeyword.trim().toLowerCase();
    return state.channelModels.filter((item) => {
        if (state.channelModelCapability && item.capability !== state.channelModelCapability) return false;
        if (state.channelModelCanvasStatus === "visible" && !channelModelCanvasVisible(item)) return false;
        if (state.channelModelCanvasStatus === "hidden" && channelModelCanvasVisible(item)) return false;
        if (state.channelModelCanvasStatus === "unpriced" && activeChannelTierRulesForModel(state.selectedChannelId, item.id).length) return false;
        if (!keyword) return true;
        return [item.displayName, item.modelKey, item.providerModelKey, item.protocol]
            .some((value) => String(value || "").toLowerCase().includes(keyword));
    });
}

function renderChannelModelsTable(models) {
    if (!state.channelModels.length) return '<div class="empty">这个渠道还没有模型。先新增一个，或从上游拉取目录。</div>';
    if (!models.length) return '<div class="empty">没有符合当前筛选条件的模型。</div>';
    const channelDisabled = selectedChannel()?.enabled === false;
    return `
        <div class="table-wrap channel-model-table" id="channel-model-list">
            <table>
                <thead><tr><th>模型</th><th>能力</th><th>画布</th><th>价格</th><th>上游参考</th><th></th></tr></thead>
                <tbody>
                    ${models.map((item) => {
                        const rules = activeChannelTierRulesForModel(state.selectedChannelId, item.id);
                        const visible = channelModelCanvasVisible(item);
                        return `<tr>
                            <td><strong>${escapeHtml(item.displayName || item.modelKey || item.id)}</strong><div class="muted">${escapeHtml(item.modelKey || item.id)}${item.providerModelKey && item.providerModelKey !== item.modelKey ? ` → ${escapeHtml(item.providerModelKey)}` : ""}</div></td>
                            <td>${escapeHtml(capabilityLabel(item.capability || "text"))}</td>
                            <td>${visible ? '<span class="tag tag-ok">画布可见</span>' : channelDisabled ? '<span class="tag tag-warn">渠道停用</span>' : '<span class="tag tag-muted">画布隐藏</span>'}</td>
                            <td>${rules.length ? `<span class="tag tag-ok">已设价</span><div class="muted">${rules.length} 条生效规则</div>` : '<span class="tag tag-warn">未设价</span>'}</td>
                            <td class="muted">${escapeHtml(upstreamPricingText(item))}</td>
                            <td><div class="btn-row channel-model-actions">
                                <button class="btn btn-sm" data-action="test-channel-model" data-id="${escapeHtml(item.id)}" ${state.busy ? "disabled" : ""}>${state.channelModelTestingId === item.id ? "测试中…" : "测试"}</button>
                                <button class="btn btn-sm" data-action="toggle-channel-model-visibility" data-id="${escapeHtml(item.id)}" data-enabled="${item.enabled ? "false" : "true"}">${item.enabled ? "下画布" : "上画布"}</button>
                                <button class="btn btn-sm btn-primary" data-action="quick-set-channel-model-price" data-id="${escapeHtml(item.id)}">一键 1 积分</button>
                                <button class="btn btn-sm" data-action="set-channel-model-price" data-id="${escapeHtml(item.id)}">去定价</button>
                                <button class="btn btn-sm" data-action="edit-channel-model" data-id="${escapeHtml(item.id)}">编辑</button>
                                <button class="btn btn-sm btn-danger" data-action="delete-channel-model" data-id="${escapeHtml(item.id)}">删除</button>
                            </div></td>
                        </tr>`;
                    }).join("")}
                </tbody>
            </table>
        </div>`;
}

function renderChannelModelResults() {
    const models = filterChannelModels();
    return `<div class="channel-model-result-head"><span id="channel-model-filter-count">显示 ${models.length} / ${state.channelModels.length} 个模型</span><span class="muted">筛选只改变当前列表，不会修改模型状态</span></div>${renderChannelModelsTable(models)}`;
}

function refreshChannelModelResults() {
    const results = document.getElementById("channel-model-results");
    if (results) results.innerHTML = renderChannelModelResults();
}

async function runChannelModelTest(model) {
    if (!model || state.busy) return;
    const label = model.displayName || model.modelKey || model.id;
    const confirmed = window.confirm(`测试“${label}”会向当前上游发起真实生成请求，可能产生服务商费用。确认开始测试？`);
    if (!confirmed) return;
    state.busy = true;
    state.channelModelTestingId = model.id;
    state.error = "";
    state.notice = "";
    render();
    try {
        const result = await testAdminChannelModel(state.selectedChannelId, model);
        const duration = Number(result?.durationMs || 0);
        const durationText = duration >= 1000 ? `${(duration / 1000).toFixed(1)} 秒` : `${Math.max(1, duration)} ms`;
        state.notice = `模型“${label}”测试通过，耗时 ${durationText}。`;
    } catch (error) {
        state.error = `模型“${label}”测试失败：${error instanceof Error ? error.message : "未知错误"}`;
    } finally {
        state.busy = false;
        state.channelModelTestingId = "";
        render();
    }
}
