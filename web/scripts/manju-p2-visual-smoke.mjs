import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";

const baseURL = process.env.MANJU_P2_BASE_URL || "http://127.0.0.1:3000";
const projectID = process.env.MANJU_P2_PROJECT_ID || "41f6979a3d99cf8a37672f280e432510";
const cookieFile = process.env.MANJU_P2_COOKIE_FILE || "D:/画布/_manual/admin-cookie-current.txt";
const outputDir = resolve(import.meta.dir, "../../build/visual");

function fail(message) {
    throw new Error(message);
}

async function sessionCookie() {
    const text = await Bun.file(cookieFile).text();
    const row = text.split(/\r?\n/).find((line) => line.includes("\tbeeftv_session\t"));
    const value = row?.split("\t")[6]?.trim();
    if (!value) fail(`cookie file does not contain beeftv_session: ${cookieFile}`);
    return value;
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
await context.addCookies([{
    name: "beeftv_session",
    value: await sessionCookie(),
    domain: "127.0.0.1",
    path: "/",
    httpOnly: true,
    sameSite: "Lax",
}]);

try {
    const page = await context.newPage();
    await page.goto(`${baseURL}/manju`, { waitUntil: "domcontentloaded" });
    const libraryPanel = page.locator(".manju-library-intro");
    await libraryPanel.waitFor({ timeout: 20_000 });
    const libraryCopy = await libraryPanel.innerText();
    if (libraryCopy.includes("P2-P5 暂未执行")) {
        fail("project library still claims P2-P5 are all unimplemented");
    }
    const assetResponse = await context.request.get(`${baseURL}/api/manju/projects/${projectID}/asset-runs`);
    if (!assetResponse.ok()) fail(`P2 API failed with ${assetResponse.status()}`);
    await page.goto(`${baseURL}/manju/${projectID}?tab=stages`, { waitUntil: "domcontentloaded" });

    await page.locator(".manju-script-workspace").waitFor({ timeout: 20_000 });
    await page.getByRole("heading", { name: "P2 数字资产", exact: true }).waitFor({ timeout: 20_000 });
    const p2Panel = page.locator(".manju-asset-workspace");
    await p2Panel.waitFor({ timeout: 20_000 });


    const bodyText = await p2Panel.innerText();
    const hasCompletedP1 = await p2Panel.getByText("尚未建立 P2 数字资产任务", { exact: true }).count() === 0;
    if (!hasCompletedP1) {
        await p2Panel.getByText("P1 剧本全部阶段完成并锁定后，P2 才会开放。未锁定草稿和小说原文不会被 P2 读取。", { exact: true }).waitFor();
        if (await p2Panel.locator('input[type="file"]').count()) fail("P2 gate exposed a file input before a run existed");
    }

    const desktopMetrics = await assertNoHorizontalOverflow(page, "desktop P2");
    await page.screenshot({ path: resolve(outputDir, "manju-p2-desktop.png"), fullPage: true });

    const mobile = await context.newPage();
    await mobile.setViewportSize({ width: 390, height: 844 });
    await mobile.goto(`${baseURL}/manju/${projectID}?tab=stages`, { waitUntil: "domcontentloaded" });
    await mobile.locator(".manju-asset-workspace").waitFor({ timeout: 20_000 });
    const mobileMetrics = await assertNoHorizontalOverflow(mobile, "mobile P2");
    await mobile.screenshot({ path: resolve(outputDir, "manju-p2-mobile.png"), fullPage: true });

    console.log(JSON.stringify({
        projectID,
        p2RouteStatus: assetResponse.status(),
        hasCompletedP1,
        bodyPreview: bodyText.slice(0, 160),
        desktopMetrics,
        mobileMetrics,
        screenshots: {
            desktop: resolve(outputDir, "manju-p2-desktop.png"),
            mobile: resolve(outputDir, "manju-p2-mobile.png"),
        },
    }, null, 2));
} finally {
    await browser.close();
}
