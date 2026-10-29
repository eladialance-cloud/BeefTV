import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";

const baseURL = process.env.MANJU_P0_BASE_URL || "http://127.0.0.1:3000";
const projectID = process.env.MANJU_P0_PROJECT_ID || "aa45590eb1863fa6849dbd5dfc9f2788";
const username = process.env.MANJU_P0_USERNAME || "visideep-admin";
const password = process.env.MANJU_P0_PASSWORD || "VisiDeep-Local-2026!";
const outputDir = resolve(import.meta.dir, "../../build/visual");

function fail(message) {
    throw new Error(message);
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

await mkdir(outputDir, { recursive: true });
const browser = await chromium.launch({ channel: "msedge", headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 1 });

try {
    const page = await context.newPage();
    await page.goto(`${baseURL}/login?next=%2Fmanju`, { waitUntil: "domcontentloaded" });
    await page.locator('input[autocomplete="username"]').fill(username);
    await page.locator('input[autocomplete="current-password"]').fill(password);
    await page.locator('button[type="submit"]').click();
    await page.waitForURL((url) => !url.pathname.startsWith("/login"), { timeout: 20_000 });

    await page.goto(`${baseURL}/manju/${projectID}?tab=p0`, { waitUntil: "domcontentloaded" });
    const styleLock = page.locator(".manju-style-lock");
    await styleLock.waitFor({ timeout: 20_000 });
    await styleLock.getByRole("heading", { name: "风格候选卡", exact: true }).waitFor();

    const candidateCards = styleLock.locator(".manju-style-card");
    const candidateCount = await candidateCards.count();
    if (candidateCount < 1 || candidateCount > 4) {
        fail(`candidate card count must be between 1 and 4, got ${candidateCount}`);
    }

    await styleLock.getByRole("button", { name: "按你的推荐走", exact: true }).click();
    await styleLock.locator(".manju-style-selection").waitFor();

    await styleLock.getByRole("button", { name: "查看全部 74 组", exact: true }).click();
    const allCards = styleLock.locator(".manju-style-all-grid button");
    await allCards.first().waitFor();
    const allCount = await allCards.count();
    if (allCount !== 74) fail(`expected 74 full catalog cards, got ${allCount}`);
    const disabledCount = await allCards.evaluateAll((items) => items.filter((item) => item.disabled).length);
    if (disabledCount < 1) fail("base-incompatible cards must remain visible but disabled");

    await styleLock.getByRole("button", { name: "自定义", exact: true }).click();
    const customInput = styleLock.locator(".manju-style-custom textarea");
    await customInput.waitFor();
    await customInput.fill("冷青低照度、侧逆光轮廓、狭窄空间贴身手持、无粒子特效");
    await styleLock.getByRole("button", { name: "按你的推荐走", exact: true }).click();
    await styleLock.locator(".manju-style-selection").waitFor();

    const receiptPanel = page.locator(".manju-receipt-panel");
    const beforeText = await receiptPanel.innerText();
    const beforeRevision = Number(beforeText.match(/Revision\s+(\d+)/)?.[1] || 0);
    const saveResponsePromise = page.waitForResponse((response) =>
        response.request().method() === "PUT" &&
        response.url().includes(`/api/manju/projects/${projectID}/lock`),
        { timeout: 20_000 },
    );
    await page.getByRole("button", { name: "保存 P0 新 revision", exact: true }).click();
    const saveResponse = await saveResponsePromise;
    if (!saveResponse.ok()) fail(`P0 save failed with ${saveResponse.status()}: ${await saveResponse.text()}`);
    const savePayload = await saveResponse.json();
    const saved = savePayload.data.item;
    if (saved.revision <= beforeRevision) fail(`revision did not advance from ${beforeRevision}`);
    if (saved.snapshot.styleCatalogVersion !== "V7.4.26") fail("saved lock is missing style catalog version");
    if (!saved.snapshot.stylePromptParameters?.trim()) fail("saved lock is missing model-facing style parameters");
    if (!saved.snapshot.styleCardName?.trim()) fail("saved lock is missing option-layer card metadata");

    await receiptPanel.getByText(`Revision ${saved.revision}`, { exact: true }).waitFor();
    const desktopMetrics = await assertNoHorizontalOverflow(page, "desktop P0");
    await page.screenshot({ path: resolve(outputDir, "manju-p0-desktop.png"), fullPage: true });

    const mobile = await context.newPage();
    await mobile.setViewportSize({ width: 390, height: 844 });
    await mobile.goto(`${baseURL}/manju/${projectID}?tab=p0`, { waitUntil: "domcontentloaded" });
    await mobile.locator(".manju-style-lock").waitFor({ timeout: 20_000 });
    const mobileMetrics = await assertNoHorizontalOverflow(mobile, "mobile P0");
    await mobile.screenshot({ path: resolve(outputDir, "manju-p0-mobile.png"), fullPage: true });

    console.log(JSON.stringify({
        projectID,
        candidateCount,
        allCount,
        disabledCount,
        revision: saved.revision,
        styleCardId: saved.snapshot.styleCardId,
        styleCardName: saved.snapshot.styleCardName,
        styleCatalogVersion: saved.snapshot.styleCatalogVersion,
        desktopMetrics,
        mobileMetrics,
        screenshots: {
            desktop: resolve(outputDir, "manju-p0-desktop.png"),
            mobile: resolve(outputDir, "manju-p0-mobile.png"),
        },
    }, null, 2));
} finally {
    await browser.close();
}
