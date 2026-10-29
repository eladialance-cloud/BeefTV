import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";

const baseURL = process.env.MANJU_P3_BASE_URL || "http://127.0.0.1:3000";
const projectID = process.env.MANJU_P3_PROJECT_ID || "41f6979a3d99cf8a37672f280e432510";
const cookieFile = process.env.MANJU_P3_COOKIE_FILE || "D:/画布/_manual/admin-cookie-current.txt";
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
    const apiRuns = await context.request.get(`${baseURL}/api/manju/projects/${projectID}/production-runs`);
    if (!apiRuns.ok()) fail(`P3-P5 API failed with ${apiRuns.status()}`);

    const page = await context.newPage();
    await page.goto(`${baseURL}/manju/${projectID}?tab=stages`, { waitUntil: "domcontentloaded" });
    await page.locator(".manju-script-workspace").waitFor({ timeout: 20_000 });
    const panel = page.locator(".manju-production-workspace");
    await panel.waitFor({ timeout: 20_000 });
    await page.getByRole("heading", { name: "分镜、视频与质检交付", exact: true }).waitFor({ timeout: 20_000 });

    const text = await panel.innerText();
    for (const required of ["故事板表合同", "P3 交付清单", "素材接线单", "人工看片"]) {
        if (!text.includes(required)) fail(`production workspace is missing ${required}`);
    }
    for (const forbidden of ["未读取生成视频", "P5 文本质检", "仍是文字投喂稿质检", "机检已完成", "交付包已生成"]) {
        if (text.includes(forbidden)) fail(`production workspace contains stale or fake copy: ${forbidden}`);
    }

    const runsPayload = await apiRuns.json();
    const hasAnyRun = runsPayload.data.items.length > 0;
    let productionDetail = null;
    if (hasAnyRun) {
        const runId = runsPayload.data.items[0].id;
        const detailResponse = await context.request.get(`${baseURL}/api/manju/projects/${projectID}/production-runs/${runId}`);
        if (!detailResponse.ok()) fail(`production detail API failed with ${detailResponse.status()}`);
        productionDetail = (await detailResponse.json()).data;
        if (!productionDetail?.stages?.some((stage) => stage.key === "qc_handoff")) {
            fail("production detail is missing qc_handoff stage");
        }
        if (!text.includes("视频交付与人工看片边界")) fail("P5 workspace is missing the real-video/manual-review boundary");
        if (productionDetail?.artifacts?.some((artifact) => artifact.kind === "FEED_PACKAGE.md") && !text.includes("P4 视频")) {
            fail("P4 artifact is present but the real video batch is not rendered");
        }
        if (productionDetail?.videos?.length && !text.includes("待主理人看片")) {
            fail("P5 videos are present but manual review status is missing");
        }
    }
    const hasGate = await panel.getByText("P3 还不能开始", { exact: true }).count() > 0;
    const stageRunButtons = await panel.getByRole("button", { name: /执行 P3|封装 P4|生成 P5/ }).count();
    if (hasGate && stageRunButtons > 0) fail("P3-P5 execute controls were exposed before P2 was locked");
    if (!hasAnyRun && hasGate) {
        const createButtons = await panel.getByRole("button", { name: "建立 P3-P5 生产链" }).count();
        if (createButtons > 0) fail("production run create button was exposed before P2 was locked");
    }

    const desktopMetrics = await assertNoHorizontalOverflow(page, "desktop P3-P5");
    await page.screenshot({ path: resolve(outputDir, "manju-p3-p5-desktop.png"), fullPage: true });

    const mobile = await context.newPage();
    await mobile.setViewportSize({ width: 390, height: 844 });
    await mobile.goto(`${baseURL}/manju/${projectID}?tab=stages`, { waitUntil: "domcontentloaded" });
    await mobile.locator(".manju-production-workspace").waitFor({ timeout: 20_000 });
    const mobileMetrics = await assertNoHorizontalOverflow(mobile, "mobile P3-P5");
    await mobile.screenshot({ path: resolve(outputDir, "manju-p3-p5-mobile.png"), fullPage: true });

    console.log(JSON.stringify({
        projectID,
        productionRouteStatus: apiRuns.status(),
        hasAnyRun,
        productionRunId: hasAnyRun ? runsPayload.data.items[0].id : null,
        productionVideoCount: productionDetail?.videos?.length || 0,
        hasGate,
        stageRunButtons,
        textPreview: text.slice(0, 240),
        desktopMetrics,
        mobileMetrics,
        screenshots: {
            desktop: resolve(outputDir, "manju-p3-p5-desktop.png"),
            mobile: resolve(outputDir, "manju-p3-p5-mobile.png"),
        },
    }, null, 2));
} finally {
    await browser.close();
}
