import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";

const baseURL = process.env.MANJU_E2E_BASE_URL || "http://127.0.0.1:13000";
const adaptationProjectID = process.env.MANJU_E2E_PROJECT_ID || process.env.MANJU_E2E_ADAPTATION_PROJECT_ID;
const branchProjectIDs = {
    original: process.env.MANJU_E2E_ORIGINAL_PROJECT_ID,
    benchmark: process.env.MANJU_E2E_BENCHMARK_PROJECT_ID,
    polish: process.env.MANJU_E2E_POLISH_PROJECT_ID,
};
const username = process.env.MANJU_E2E_USERNAME || "e2e_admin_1018";
const password = process.env.MANJU_E2E_PASSWORD || "VisiDeep-E2E-1018!";

if (!adaptationProjectID) throw new Error("MANJU_E2E_PROJECT_ID or MANJU_E2E_ADAPTATION_PROJECT_ID is required");

const outputDir = resolve(import.meta.dir, "../../build/visual");
await mkdir(outputDir, { recursive: true });

function fail(message) {
    throw new Error(message);
}

async function assertText(scope, text) {
    await scope.getByText(text, { exact: true }).first().waitFor({ timeout: 20_000 });
}

async function assertNoHorizontalOverflow(page, label) {
    const metrics = await page.evaluate(() => ({
        scrollWidth: document.documentElement.scrollWidth,
        clientWidth: document.documentElement.clientWidth,
    }));
    if (metrics.scrollWidth > metrics.clientWidth + 1) {
        fail(`${label} horizontal overflow: ${JSON.stringify(metrics)}`);
    }
    return metrics;
}

const browser = await chromium.launch({ channel: "msedge", headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 1 });
const page = await context.newPage();

async function login() {
    await page.goto(`${baseURL}/login`, { waitUntil: "domcontentloaded" });
    await page.locator('input[autocomplete="username"]').fill(username);
    await page.locator('input[autocomplete="current-password"]').fill(password);
    await page.locator('button[type="submit"]').click();
    await page.waitForURL(url => !url.pathname.startsWith("/login"), { timeout: 20_000 });
}

async function openProject(projectID) {
    await page.goto(`${baseURL}/manju/${projectID}?tab=stages`, { waitUntil: "domcontentloaded" });
    await page.waitForSelector(".manju-script-workspace", { timeout: 20_000 });
    await page.locator(".manju-script-stage").first().waitFor({ timeout: 20_000 });
}

try {
    await login();

    await openProject(adaptationProjectID);
    const startButton = page.getByRole("button", { name: "开始 P1", exact: true });
    if (await startButton.isVisible().catch(() => false)) {
        await startButton.click();
    }

    const sourceForm = page.locator(".manju-script-source-form");
    if (await sourceForm.count()) {
        const sourceInputs = sourceForm.locator("input");
        await sourceInputs.nth(0).fill("临时验收小说");
        await sourceInputs.nth(1).fill("第 1 章第 1-2 节");
        await sourceInputs.nth(2).fill("仅用于本地自动化验收，不进入现有数据库");
        await sourceForm.locator("textarea").fill("第一章 雨夜\n林澈推开旧宅的门，发现桌上放着一封没有署名的信。");
        await page.getByRole("button", { name: "登记来源", exact: true }).click();
    }

    const confirmButton = page.getByRole("button", { name: "确认并进入下一阶段", exact: true });
    await page.waitForSelector(".manju-script-source-summary", { timeout: 20_000 });
    await page.screenshot({ path: resolve(outputDir, "manju-p1-source-desktop.png"), fullPage: true });
    if (await confirmButton.isVisible().catch(() => false)) {
        await confirmButton.click();
    }
    const executeIPButton = page.getByRole("button", { name: "执行 IP 与角色开发", exact: true });
    await executeIPButton.waitFor({ timeout: 20_000 });
    await page.screenshot({ path: resolve(outputDir, "manju-p1-confirmed-desktop.png"), fullPage: true });
    const desktopMetrics = await assertNoHorizontalOverflow(page, "desktop adaptation");

    const ipStage = page.locator(".manju-script-stage").filter({ hasText: "IP 与角色开发" }).first();
    const runResponsePromise = page.waitForResponse(response =>
        response.request().method() === "POST" && response.url().includes("/stages/ip_development/run"),
        { timeout: 20_000 },
    );
    await executeIPButton.click();
    const runResponse = await runResponsePromise;
    if (!runResponse.ok()) fail(`IP stage run request failed with ${runResponse.status()}`);
    await ipStage.locator(".manju-script-running").waitFor({ timeout: 20_000 });
    await ipStage.locator(".manju-script-status.is-blocked").filter({ hasText: "执行失败" }).waitFor({ timeout: 45_000 });
    const failedStage = ipStage;
    const failureNotices = (await failedStage.locator(".manju-notice.is-error").allInnerTexts()).map(text => text.trim()).filter(Boolean);
    if (!failureNotices.length) fail("blocked stage did not expose a real failure reason");
    if (failureNotices.some(text => text.includes("本地工作区不支持系统渠道配置"))) {
        fail("hosted local kernel rejected the admin-managed system channel");
    }
    if (await failedStage.locator(".manju-script-artifact").count()) fail("blocked stage generated an artifact");
    if (await failedStage.getByRole("button", { name: /确认|提交圆桌/ }).count()) fail("blocked stage exposed a confirmation action");
    await page.screenshot({ path: resolve(outputDir, "manju-p1-blocked-desktop.png"), fullPage: true });

    const branchChecks = {};
    if (branchProjectIDs.original) {
        await openProject(branchProjectIDs.original);
        const form = page.locator(".manju-script-source-form");
        await form.waitFor({ timeout: 20_000 });
        for (const label of ["故事名", "目标篇幅", "原创声明", "故事种子"]) await assertText(form, label);
        await assertText(page, "原创故事登记");
        await page.screenshot({ path: resolve(outputDir, "manju-p1-original-desktop.png"), fullPage: true });
        branchChecks.original = await assertNoHorizontalOverflow(page, "desktop original");
    }

    if (branchProjectIDs.benchmark) {
        await openProject(branchProjectIDs.benchmark);
        const firstStage = page.locator(".manju-script-stage").first();
        await assertText(firstStage, "对标情报卡");
        const secondStage = page.locator(".manju-script-stage").nth(1);
        if (await secondStage.locator(".manju-script-source-form").count()) {
            fail("benchmark source registration is editable before the intelligence card is confirmed");
        }
        if (await page.getByText("小说改编登记", { exact: true }).count()) {
            fail("benchmark branch rendered the novel-adaptation source form");
        }
        await page.screenshot({ path: resolve(outputDir, "manju-p1-benchmark-desktop.png"), fullPage: true });
        branchChecks.benchmark = await assertNoHorizontalOverflow(page, "desktop benchmark");
    }

    if (branchProjectIDs.polish) {
        await openProject(branchProjectIDs.polish);
        const form = page.locator(".manju-script-source-form");
        await form.waitFor({ timeout: 20_000 });
        for (const label of ["剧本名", "润色范围", "授权范围", "外来剧本正文"]) await assertText(form, label);
        await assertText(page, "外来剧本登记");
        await page.screenshot({ path: resolve(outputDir, "manju-p1-polish-desktop.png"), fullPage: true });
        branchChecks.polish = await assertNoHorizontalOverflow(page, "desktop polish");
    }

    const mobile = await context.newPage();
    await mobile.setViewportSize({ width: 390, height: 844 });
    await mobile.goto(`${baseURL}/manju/${adaptationProjectID}?tab=stages`, { waitUntil: "domcontentloaded" });
    await mobile.waitForSelector(".manju-script-source-summary", { timeout: 20_000 });
    const mobileMetrics = await assertNoHorizontalOverflow(mobile, "mobile adaptation");
    await mobile.screenshot({ path: resolve(outputDir, "manju-p1-source-mobile.png"), fullPage: true });

    console.log(JSON.stringify({
        desktopMetrics,
        mobileMetrics,
        branchChecks,
        failureNotices,
        screenshots: {
            desktop: resolve(outputDir, "manju-p1-source-desktop.png"),
            blocked: resolve(outputDir, "manju-p1-blocked-desktop.png"),
            original: branchProjectIDs.original ? resolve(outputDir, "manju-p1-original-desktop.png") : null,
            benchmark: branchProjectIDs.benchmark ? resolve(outputDir, "manju-p1-benchmark-desktop.png") : null,
            polish: branchProjectIDs.polish ? resolve(outputDir, "manju-p1-polish-desktop.png") : null,
            mobile: resolve(outputDir, "manju-p1-source-mobile.png"),
        },
    }, null, 2));
} finally {
    await browser.close();
}
