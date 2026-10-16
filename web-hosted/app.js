// BeefTV 托管工作台：登录、钱包流水，以及管理后台的用户积分与对象存储配置。
//
// 这是给网页版（server profile）用的独立前端，刻意不放进 web/src：
// web/src 的本地桌面边界测试明确禁止托管 UI 入口出现在桌面产物里。
// 本文件零依赖，直接以静态站点方式部署即可（见 README.md）。

import { buildAdjustIdempotencyKey, escapeHtml, formatFenToYuan, formatPoints, formatTime, kindLabel, resolveApiBase, stageLabel } from "./format.js";

const API_BASE = (() => {
    const injected = typeof window !== "undefined" && typeof window.__BEEFTV_API_BASE__ === "string" ? window.__BEEFTV_API_BASE__ : "";
    return resolveApiBase({ injected, stored: localStorage.getItem("beeftv.apiBase") || "" });
})();

const ROUTES = [
    { path: "/wallet", label: "我的钱包", section: "用户", icon: "wallet" },
    { path: "/topup", label: "充值", section: "用户", icon: "card" },
    { path: "/admin/users", label: "用户积分", section: "管理后台", icon: "users", admin: true },
    { path: "/admin/topup", label: "充值商品", section: "管理后台", icon: "card", admin: true },
    { path: "/admin/orders", label: "支付订单", section: "管理后台", icon: "list", admin: true },
    { path: "/admin/storage", label: "对象存储", section: "管理后台", icon: "database", admin: true },
    { path: "/admin/payment", label: "支付渠道", section: "管理后台", icon: "card", admin: true },
];

const state = {
    user: null,
    account: null,
    ledger: [],
    accounts: [],
    storage: [],
    payments: [],
    topupProducts: [],
    topupOrders: [],
    adminProducts: [],
    adminOrders: [],
    busy: false,
    error: "",
    notice: "",
};

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

async function register(username, displayName, password) {
    const data = await api("/register", { method: "POST", body: { username, displayName, password } });
    state.user = data.user;
}

async function logout() {
    try {
        await api("/logout", { method: "POST" });
    } finally {
        state.user = null;
        state.account = null;
        state.ledger = [];
        state.accounts = [];
        state.storage = [];
        state.payments = [];
        state.topupProducts = [];
        state.topupOrders = [];
        state.adminProducts = [];
        state.adminOrders = [];
    }
}

/* ---------------- 数据 ---------------- */

async function loadWallet() {
    const [wallet, ledger] = await Promise.all([api("/credits/wallet"), api("/credits/ledger?limit=50")]);
    state.account = wallet.account;
    state.ledger = ledger.items || [];
}

async function loadAccounts() {
    const data = await api("/admin/credits/accounts");
    state.accounts = data.items || [];
}

async function adjustCredits(userId, amount, reason) {
    return api("/admin/credits/adjust", {
        method: "POST",
        body: { userId, amount, reason, idempotencyKey: buildAdjustIdempotencyKey(userId, Date.now(), Math.random().toString(36).slice(2, 8)) },
    });
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

async function loadTopup() {
    const [products, orders] = await Promise.all([api("/topup/products"), api("/topup/orders?limit=50")]);
    state.topupProducts = products.items || [];
    state.topupOrders = orders.items || [];
}

async function createTopupOrder(productId, provider) {
    return api("/topup/orders", { method: "POST", body: { productId, provider } });
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
    const data = await api("/admin/payment-orders?limit=100");
    state.adminOrders = data.items || [];
}

async function markOrderPaid(id, providerTradeNo) {
    return api(`/admin/payment-orders/${encodeURIComponent(id)}/mark-paid`, { method: "POST", body: { providerTradeNo } });
}

async function closeOrder(id, reason) {
    return api(`/admin/payment-orders/${encodeURIComponent(id)}/close`, { method: "POST", body: { reason } });
}

/* ---------------- 小工具 ---------------- */

const ICONS = {
    wallet: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7.5A2.5 2.5 0 0 1 5.5 5H18a2 2 0 0 1 2 2v1H5.5"/><path d="M3 7.5V17a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2H5.5A2.5 2.5 0 0 1 3 7.5Z"/><circle cx="16.5" cy="13.5" r="1"/></svg>',
    users: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M16 19v-1.5a3.5 3.5 0 0 0-3.5-3.5h-5A3.5 3.5 0 0 0 4 17.5V19"/><circle cx="10" cy="8" r="3.2"/><path d="M20 19v-1.5a3.5 3.5 0 0 0-2.6-3.4M15.5 5.2a3.2 3.2 0 0 1 0 5.6"/></svg>',
    database: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="6" rx="7.5" ry="3"/><path d="M4.5 6v12c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3V6"/><path d="M4.5 12c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3"/></svg>',
    logout: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M15 12H3.5"/><path d="m7 8-4 4 4 4"/><path d="M11 4h6a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-6"/></svg>',
    card: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="5" width="19" height="14" rx="2"/><path d="M2.5 9.5h19"/><path d="M6.5 14.5h3"/></svg>',
    list: '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M8 6h12M8 12h12M8 18h12"/><circle cx="4" cy="6" r="1"/><circle cx="4" cy="12" r="1"/><circle cx="4" cy="18" r="1"/></svg>',
};

function icon(name) {
    return ICONS[name] || "";
}

function currentPath() {
    const raw = (window.location.hash || "").replace(/^#/u, "");
    return raw || "/wallet";
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
    const path = currentPath();
    const route = ROUTES.find((item) => item.path === path) || ROUTES[0];
    if (route.admin && state.user.role !== "admin") navigate("/wallet");
    app.innerHTML = `
        <div class="shell">
            <aside class="sidebar">
                <div class="sidebar-brand">
                    <span class="auth-brand-mark">B</span>
                    <span class="auth-brand-name">BeefTV</span>
                </div>
                <nav class="sidebar-nav">
                    ${ROUTES.filter((item) => !item.admin || state.user.role === "admin")
                        .map((item, index, list) => {
                            const header = item.section !== list[index - 1]?.section ? `<div class="nav-section">${escapeHtml(item.section)}</div>` : "";
                            return `${header}<a class="nav-item" href="#${item.path}" ${item.path === route.path ? 'aria-current="page"' : ""}>${icon(item.icon)}<span>${escapeHtml(item.label)}</span></a>`;
                        })
                        .join("")}
                </nav>
                <div class="sidebar-foot">
                    <div class="who">
                        <span class="who-name">${escapeHtml(state.user.displayName || state.user.username)}</span>
                        <span class="who-role">${state.user.role === "admin" ? "管理员" : "普通用户"}</span>
                    </div>
                    <button class="btn btn-sm" data-action="logout">${icon("logout")}退出登录</button>
                </div>
            </aside>
            <main class="main" id="view"></main>
        </div>`;
    const view = document.getElementById("view");
    if (route.path === "/admin/users") renderAdminUsers(view);
    else if (route.path === "/topup") renderTopup(view);
    else if (route.path === "/admin/topup") renderAdminTopup(view);
    else if (route.path === "/admin/orders") renderAdminOrders(view);
    else if (route.path === "/admin/storage") renderAdminStorage(view);
    else if (route.path === "/admin/payment") renderAdminPayment(view);
    else renderWallet(view);
}

function renderFlash() {
    if (state.error) return `<div class="banner banner-error">${escapeHtml(state.error)}</div>`;
    if (state.notice) return `<div class="banner banner-ok">${escapeHtml(state.notice)}</div>`;
    return "";
}

function renderLogin() {
    const mode = state.authMode === "register" ? "register" : "login";
    app.innerHTML = `
        <div class="auth-layout">
            <div class="auth-card">
                <div class="auth-brand">
                    <span class="auth-brand-mark">B</span>
                    <span class="auth-brand-name">BeefTV 工作台</span>
                </div>
                <div class="auth-tabs" role="tablist">
                    <button class="auth-tab" role="tab" aria-selected="${mode === "login"}" data-action="auth-mode" data-mode="login">登录</button>
                    <button class="auth-tab" role="tab" aria-selected="${mode === "register"}" data-action="auth-mode" data-mode="register">注册</button>
                </div>
                <div id="flash">${renderFlash()}</div>
                <form id="auth-form" autocomplete="on">
                    <label class="field">
                        <span class="field-label">登录名</span>
                        <input class="input" name="username" autocomplete="username" required />
                    </label>
                    ${
                        mode === "register"
                            ? `<label class="field"><span class="field-label">显示名称（可选）</span><input class="input" name="displayName" autocomplete="nickname" /></label>`
                            : ""
                    }
                    <label class="field">
                        <span class="field-label">密码</span>
                        <input class="input" name="password" type="password" autocomplete="${mode === "register" ? "new-password" : "current-password"}" required />
                    </label>
                    <button class="btn btn-primary btn-block" type="submit" ${state.busy ? "disabled" : ""}>${mode === "register" ? "注册并登录" : "登录"}</button>
                </form>
            </div>
        </div>`;
}

function metric(label, value, suffix = "") {
    return `<div class="metric"><div class="metric-label">${escapeHtml(label)}</div><div class="metric-value">${escapeHtml(value)}${suffix ? `<small>${escapeHtml(suffix)}</small>` : ""}</div></div>`;
}

function renderWallet(view) {
    const account = state.account;
    view.innerHTML = `
        <div class="page-head">
            <div>
                <h1 class="page-title">我的钱包</h1>
                <p class="page-sub">提交生成任务时先冻结积分，结束后按实际用量结算，失败或取消全额退回。</p>
            </div>
            <button class="btn btn-sm" data-action="refresh">刷新</button>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel">
            <div class="metric-row">
                ${metric("账户余额", account ? formatPoints(account.balance) : "-")}
                ${metric("冻结中", account ? formatPoints(account.frozen) : "-")}
                ${metric("可用", account ? formatPoints(account.available) : "-")}
            </div>
        </div>
        <div class="panel">
            <h2 class="panel-title">积分流水（最近 50 条）</h2>
            ${renderLedger()}
        </div>`;
}

function renderLedger() {
    if (!state.ledger.length) return `<div class="empty">还没有积分变动</div>`;
    return `
        <div class="table-wrap">
            <table>
                <thead>
                    <tr><th>时间</th><th>类型</th><th>阶段</th><th>任务</th><th class="num">变动</th><th class="num">余额</th><th>原因</th></tr>
                </thead>
                <tbody>
                    ${state.ledger
                        .map((entry) => {
                            const delta = entry.balanceAfter - entry.balanceBefore;
                            const deltaClass = delta > 0 ? "amount-pos" : delta < 0 ? "amount-neg" : "muted";
                            const frozenDelta = entry.frozenAfter - entry.frozenBefore;
                            const detail = delta !== 0 ? delta : frozenDelta;
                            return `<tr>
                                <td class="muted">${escapeHtml(formatTime(entry.createdAt))}</td>
                                <td>${escapeHtml(kindLabel(entry.kind))}</td>
                                <td class="muted">${escapeHtml(stageLabel(entry.stage))}</td>
                                <td class="muted">${escapeHtml(entry.taskId || "-")}</td>
                                <td class="num ${deltaClass}">${detail > 0 ? "+" : ""}${escapeHtml(formatPoints(detail))}</td>
                                <td class="num">${escapeHtml(formatPoints(entry.balanceAfter))}</td>
                                <td class="muted">${escapeHtml(entry.reason || "-")}</td>
                            </tr>`;
                        })
                        .join("")}
                </tbody>
            </table>
        </div>`;
}

function renderAdminUsers(view) {
    view.innerHTML = `
        <div class="page-head">
            <div>
                <h1 class="page-title">用户积分</h1>
                <p class="page-sub">人工加减积分会写入不可变流水，必须填写原因，可追溯到操作人。</p>
            </div>
            <button class="btn btn-sm" data-action="refresh">刷新</button>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel">
            <h2 class="panel-title">调整积分</h2>
            <form id="adjust-form" class="form-grid">
                <label class="field"><span class="field-label">用户 ID</span><input class="input" name="userId" placeholder="从下方列表复制" required /></label>
                <label class="field"><span class="field-label">变动数量（负数表示扣减）</span><input class="input" name="amount" type="number" step="1" required /></label>
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
                                <td class="num">${escapeHtml(formatPoints(account.balance))}</td>
                                <td class="num">${escapeHtml(formatPoints(account.frozen))}</td>
                                <td class="num">${escapeHtml(formatPoints(account.available))}</td>
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
        render();
    }
}

document.addEventListener("click", (event) => {
    const target = event.target.closest("[data-action]");
    if (!target) return;
    const action = target.dataset.action;
    if (action === "logout") {
        event.preventDefault();
        withBusy(async () => {
            await logout();
            navigate("/wallet");
        });
    } else if (action === "auth-mode") {
        state.authMode = target.dataset.mode;
        clearFlash();
        render();
    } else if (action === "refresh") {
        withBusy(() => reloadCurrent());
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
    } else if (action === "buy-topup") {
        withBusy(async () => {
            await createTopupOrder(target.dataset.id, target.dataset.provider);
            await loadTopup();
            state.notice = "订单已创建，请完成支付；暂未接入网关时由管理员确认入账";
        });
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
    } else if (action === "delete-payment") {
        if (!window.confirm("删除这个支付渠道？已下订单不受影响，但之后无法再用它收款。")) return;
        withBusy(async () => {
            await deletePayment(target.dataset.id);
            await loadPayments();
            fillPaymentForm(null);
            state.notice = "支付渠道已删除";
        });
    }
});

document.addEventListener("submit", (event) => {
    const form = event.target;
    event.preventDefault();
    if (form.id === "auth-form") {
        withBusy(async () => {
            const username = form.username.value.trim();
            const password = form.password.value;
            if (state.authMode === "register") await register(username, form.displayName.value.trim(), password);
            else await login(username, password);
            await reloadCurrent();
        });
    } else if (form.id === "adjust-form") {
        withBusy(async () => {
            await adjustCredits(form.userId.value.trim(), Number(form.amount.value), form.reason.value.trim());
            await Promise.all([loadAccounts(), loadWallet().catch(() => {})]);
            state.notice = "调整已入账";
        });
    } else if (form.id === "storage-form") {
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
    } else if (form.id === "payment-form") {
        withBusy(async () => {
            const paymentId = form.id.value.trim();
            await savePayment(paymentId, {
                provider: form.provider.value,
                name: form.name.value.trim(),
                appId: form.appId.value.trim(),
                mchId: form.mchId.value.trim(),
                notifyUrl: form.notifyUrl.value.trim(),
                apiKey: form.apiKey.value,
                privateKey: form.privateKey.value,
                platformPublicKey: form.platformPublicKey.value,
                enabled: form.enabled.checked,
            });
            await loadPayments();
            fillPaymentForm(null);
            state.notice = paymentId ? "支付渠道已更新" : "支付渠道已保存";
        });
    } else if (form.id === "topup-form") {
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
});

window.addEventListener("hashchange", () => {
    clearFlash();
    withBusy(() => reloadCurrent());
});

async function reloadCurrent() {
    if (!state.user) return;
    const path = currentPath();
    if (path === "/admin/users") return loadAccounts();
    if (path === "/topup") return loadTopup();
    if (path === "/admin/topup") return loadAdminProducts();
    if (path === "/admin/orders") return loadAdminOrders();
    if (path === "/admin/storage") return loadStorage();
    if (path === "/admin/payment") return loadPayments();
    return loadWallet();
}

function renderAdminPayment(view) {
function renderTopup(view) {
    view.innerHTML = `
        <div class="page-head">
            <div>
                <h1 class="page-title">充值</h1>
                <p class="page-sub">选择充值套餐下单，支付完成后积分自动到账。当前尚未接入网关，订单需管理员确认。</p>
            </div>
            <button class="btn btn-sm" data-action="refresh">刷新</button>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel">
            <h2 class="panel-title">充值套餐</h2>
            ${renderTopupProducts()}
        </div>
        <div class="panel">
            <h2 class="panel-title">我的充值订单</h2>
            ${renderTopupOrders()}
        </div>`;
}

function renderTopupProducts() {
    if (!state.topupProducts.length) return `<div class="empty">还没有可购买的充值套餐</div>`;
    return `
        <div class="table-wrap">
            <table>
                <thead><tr><th>套餐</th><th class="num">价格</th><th class="num">到账积分</th><th>渠道</th><th></th></tr></thead>
                <tbody>
                    ${state.topupProducts
                        .map(
                            (item) => `<tr>
                                <td>${escapeHtml(item.name)}${item.bonus > 0 ? `<span class="tag tag-muted">赠 ${escapeHtml(formatPoints(item.bonus))}</span>` : ""}</td>
                                <td class="num">￥${escapeHtml(formatFenToYuan(item.priceFen))}</td>
                                <td class="num">${escapeHtml(formatPoints(item.credits + item.bonus))}</td>
                                <td class="muted">${item.provider === "wechat" ? "微信支付" : "支付宝"}</td>
                                <td><button class="btn btn-sm btn-primary" data-action="buy-topup" data-id="${escapeHtml(item.id)}" data-provider="${escapeHtml(item.provider)}">下单</button></td>
                            </tr>`,
                        )
                        .join("")}
                </tbody>
            </table>
        </div>`;
}

function renderTopupOrders() {
    if (!state.topupOrders.length) return `<div class="empty">还没有充值订单</div>`;
    return `
        <div class="table-wrap">
            <table>
                <thead><tr><th>时间</th><th>套餐</th><th class="num">金额</th><th class="num">积分</th><th>状态</th><th>订单号</th></tr></thead>
                <tbody>
                    ${state.topupOrders
                        .map(
                            (item) => `<tr>
                                <td class="muted">${escapeHtml(formatTime(item.createdAt))}</td>
                                <td>${escapeHtml(item.productName)}</td>
                                <td class="num">￥${escapeHtml(formatFenToYuan(item.amountFen))}</td>
                                <td class="num">${escapeHtml(formatPoints(item.credits))}</td>
                                <td>${orderStatusTag(item.status)}</td>
                                <td class="muted">${escapeHtml(item.id)}</td>
                            </tr>`,
                        )
                        .join("")}
                </tbody>
            </table>
        </div>`;
}

function orderStatusTag(status) {
    if (status === "paid") return '<span class="tag tag-ok">已支付</span>';
    if (status === "closed") return '<span class="tag tag-muted">已关闭</span>';
    return '<span class="tag tag-warn">待支付</span>';
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
                                <td class="muted">${item.provider === "wechat" ? "微信支付" : "支付宝"}</td>
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
                <p class="page-sub">确认线下收款会按订单快照的积分入账，并写不可变流水；同一订单重复确认只入账一次。</p>
            </div>
            <button class="btn btn-sm" data-action="refresh">刷新</button>
        </div>
        <div id="flash">${renderFlash()}</div>
        <div class="panel">
            <h2 class="panel-title">全部订单（最近 100 条）</h2>
            ${renderAdminOrdersTable()}
        </div>`;
}

function renderAdminOrdersTable() {
    if (!state.adminOrders.length) return `<div class="empty">还没有充值订单</div>`;
    return `
        <div class="table-wrap">
            <table>
                <thead><tr><th>时间</th><th>用户</th><th>套餐</th><th class="num">金额</th><th class="num">积分</th><th>状态</th><th></th></tr></thead>
                <tbody>
                    ${state.adminOrders
                        .map(
                            (item) => `<tr>
                                <td class="muted">${escapeHtml(formatTime(item.createdAt))}</td>
                                <td class="muted">${escapeHtml(item.userId)}</td>
                                <td>${escapeHtml(item.productName)}</td>
                                <td class="num">￥${escapeHtml(formatFenToYuan(item.amountFen))}</td>
                                <td class="num">${escapeHtml(formatPoints(item.credits))}</td>
                                <td>${orderStatusTag(item.status)}</td>
                                <td>
                                    ${item.status === "pending_payment" ? `<div class="btn-row"><button class="btn btn-sm btn-primary" data-action="mark-order-paid" data-id="${escapeHtml(item.id)}">确认支付</button><button class="btn btn-sm" data-action="close-order" data-id="${escapeHtml(item.id)}">关闭</button></div>` : ""}
                                </td>
                            </tr>`,
                        )
                        .join("")}
                </tbody>
            </table>
        </div>`;
}

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
                        </select>
                    </label>
                    <label class="field"><span class="field-label">AppID</span><input class="input" name="appId" /></label>
                    <label class="field"><span class="field-label">商户号 MchID</span><input class="input" name="mchId" /></label>
                    <label class="field"><span class="field-label">回调地址 NotifyURL</span><input class="input" name="notifyUrl" placeholder="https://你的域名/api/payments/notify" /></label>
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
                                <td class="muted">${item.provider === "wechat" ? "微信支付" : "支付宝"}</td>
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
    form.notifyUrl.value = item ? item.notifyUrl || "" : "";
    form.apiKey.value = "";
    form.privateKey.value = "";
    form.platformPublicKey.value = "";
    form.enabled.checked = item ? item.enabled !== false : true;
    const title = document.getElementById("payment-form-title");
    if (title) title.textContent = item ? `编辑渠道：${item.name}` : "添加支付渠道";
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
