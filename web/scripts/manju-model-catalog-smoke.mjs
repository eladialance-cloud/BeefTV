import { mkdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const baseURL = process.env.MANJU_MODEL_BASE_URL || "http://127.0.0.1:3000";
const projectID = process.env.MANJU_MODEL_PROJECT_ID || "aa45590eb1863fa6849dbd5dfc9f2788";
const username = process.env.MANJU_MODEL_USERNAME || "visideep-admin";
const password = process.env.MANJU_MODEL_PASSWORD || "VisiDeep-Local-2026!";
const scriptDir = dirname(fileURLToPath(import.meta.url));
const outputDir = resolve(scriptDir, "../../build/visual");

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

    await page.goto(`${baseURL}/manju/${projectID}?tab=models`, { waitUntil: "domcontentloaded" });
    await page.locator(".manju-model-layout").waitFor({ timeout: 20_000 });

    const warning = page.getByText("画布后台还没有可用模型", { exact: true });
    await warning.waitFor({ timeout: 20_000 });

    const saveButton = page.locator(".manju-model-layout .manju-panel.is-side button").last();
    const state = {
        warning: await warning.textContent(),
        saveLabel: (await saveButton.textContent())?.trim(),
        saveDisabled: await saveButton.isDisabled(),
        enabledModelSelects: await page.locator(".manju-model-slot:not(:has(.ant-select-disabled)) .ant-select").count(),
    };
    await page.screenshot({ path: resolve(outputDir, "manju-models-empty-catalog-1440.png"), fullPage: true });
    console.log(JSON.stringify({ state, outputDir }, null, 2));
} finally {
    await browser.close();
}
