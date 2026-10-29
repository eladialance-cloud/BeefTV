import { mkdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const baseURL = process.env.MANJU_LAYOUT_BASE_URL || "http://127.0.0.1:3000";
const projectID = process.env.MANJU_LAYOUT_PROJECT_ID || "aa45590eb1863fa6849dbd5dfc9f2788";
const username = process.env.MANJU_LAYOUT_USERNAME || "visideep-admin";
const password = process.env.MANJU_LAYOUT_PASSWORD || "VisiDeep-Local-2026!";
const scriptDir = dirname(fileURLToPath(import.meta.url));
const outputDir = resolve(scriptDir, "../../build/visual");

function fail(message) {
    throw new Error(message);
}

async function measure(page, selector, label) {
    const result = await page.evaluate((targetSelector) => {
        const body = document.querySelector(".manju-page-body");
        const target = document.querySelector(targetSelector);
        if (!(body instanceof HTMLElement) || !(target instanceof HTMLElement)) return null;
        const bodyRect = body.getBoundingClientRect();
        const targetRect = target.getBoundingClientRect();
        const bodyStyle = getComputedStyle(body);
        const innerWidth = body.clientWidth - Number.parseFloat(bodyStyle.paddingLeft) - Number.parseFloat(bodyStyle.paddingRight);
        return {
            bodyWidth: Math.round(bodyRect.width),
            innerWidth: Math.round(innerWidth),
            contentWidth: Math.round(targetRect.width),
            leftGap: Math.round(targetRect.left - bodyRect.left),
            rightGap: Math.round(bodyRect.right - targetRect.right),
            scrollWidth: document.documentElement.scrollWidth,
            clientWidth: document.documentElement.clientWidth,
        };
    }, selector);
    if (!result) fail(`${label}: missing ${selector}`);
    if (result.scrollWidth > result.clientWidth + 1) fail(`${label}: horizontal overflow ${JSON.stringify(result)}`);
    if (result.contentWidth < result.innerWidth - 2) fail(`${label}: content does not fill the workspace ${JSON.stringify(result)}`);
    return result;
}

await mkdir(outputDir, { recursive: true });
const browser = await chromium.launch({ channel: "msedge", headless: true });
const context = await browser.newContext({ viewport: { width: 1920, height: 1080 }, deviceScaleFactor: 1 });

try {
    const page = await context.newPage();
    await page.goto(`${baseURL}/login?next=%2Fmanju`, { waitUntil: "domcontentloaded" });
    await page.locator('input[autocomplete="username"]').fill(username);
    await page.locator('input[autocomplete="current-password"]').fill(password);
    await page.locator('button[type="submit"]').click();
    await page.waitForURL((url) => !url.pathname.startsWith("/login"), { timeout: 20_000 });

    const checks = {};
    for (const width of [1920, 1440]) {
        await page.setViewportSize({ width, height: 1000 });
        await page.goto(`${baseURL}/manju`, { waitUntil: "domcontentloaded" });
        await page.locator(".manju-project-grid, .manju-empty-state").first().waitFor({ timeout: 20_000 });
        const librarySelector = (await page.locator(".manju-project-grid").count()) ? ".manju-project-grid" : ".manju-empty-state";
        checks[`library-${width}`] = await measure(page, librarySelector, `library ${width}`);
        await page.screenshot({ path: resolve(outputDir, `manju-layout-library-${width}.png`), fullPage: true });

        await page.goto(`${baseURL}/manju/${projectID}?tab=overview`, { waitUntil: "domcontentloaded" });
        await page.locator(".manju-overview").waitFor({ timeout: 20_000 });
        checks[`overview-${width}`] = await measure(page, ".manju-overview", `overview ${width}`);
        await page.screenshot({ path: resolve(outputDir, `manju-layout-overview-${width}.png`), fullPage: true });

        await page.goto(`${baseURL}/manju/${projectID}?tab=p0`, { waitUntil: "domcontentloaded" });
        await page.locator(".manju-p0-layout").waitFor({ timeout: 20_000 });
        checks[`p0-${width}`] = await measure(page, ".manju-p0-layout", `p0 ${width}`);
        await page.screenshot({ path: resolve(outputDir, `manju-layout-p0-${width}.png`), fullPage: true });
    }

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`${baseURL}/manju`, { waitUntil: "domcontentloaded" });
    await page.locator(".manju-project-grid, .manju-empty-state").first().waitFor({ timeout: 20_000 });
    const mobileLibrarySelector = (await page.locator(".manju-project-grid").count()) ? ".manju-project-grid" : ".manju-empty-state";
    checks["library-390"] = await measure(page, mobileLibrarySelector, "library 390");
    await page.screenshot({ path: resolve(outputDir, "manju-layout-library-390.png"), fullPage: true });

    await page.goto(`${baseURL}/manju/${projectID}?tab=p0`, { waitUntil: "domcontentloaded" });
    await page.locator(".manju-p0-layout").waitFor({ timeout: 20_000 });
    checks["p0-390"] = await measure(page, ".manju-p0-layout", "p0 390");
    await page.screenshot({ path: resolve(outputDir, "manju-layout-p0-390.png"), fullPage: true });

    console.log(JSON.stringify({ checks, outputDir }, null, 2));
} finally {
    await browser.close();
}
