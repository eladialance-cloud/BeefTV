import { afterAll, beforeAll, expect, test } from "bun:test";
import { existsSync, readFileSync } from "node:fs";
import { chromium, type Browser, type Page } from "playwright";

let browser: Browser;
let server: ReturnType<typeof Bun.serve>;

beforeAll(async () => {
    const build = await Bun.build({
        entrypoints: [import.meta.dir + "/fixtures/manju-p0-lock-harness.tsx"],
        target: "browser",
        define: {
            "import.meta.env": "{}",
            "process.env.NODE_ENV": '"production"',
        },
        plugins: [{
            name: "aliases",
            setup(builder) {
                builder.onResolve({ filter: /^@\// }, (args) => ({
                    path: Bun.resolveSync("../src/" + args.path.slice(2), import.meta.dir),
                }));
            },
        }],
    });
    if (!build.success) throw new Error(build.logs.join("\n"));
    const script = await build.outputs.find((output) => output.path.endsWith(".js"))!.text();
    const css = readFileSync(import.meta.dir + "/../src/pages/manju/manju-studio.css", "utf8");
    server = Bun.serve({
        port: 0,
        fetch(request) {
            const path = new URL(request.url).pathname;
            if (path === "/app.js") return new Response(script, { headers: { "Content-Type": "text/javascript" } });
            if (path === "/manju.css") return new Response(css, { headers: { "Content-Type": "text/css" } });
            return new Response(
                '<meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/manju.css"><div id="root"></div><script type="module" src="/app.js"></script>',
                { headers: { "Content-Type": "text/html" } },
            );
        },
    });
    const executablePath = [
        process.env.CHROME_PATH,
        "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    ].find((path): path is string => Boolean(path && existsSync(path)));
    browser = await chromium.launch({ executablePath, headless: true });
}, 60_000);

afterAll(async () => {
    await browser?.close();
    server?.stop(true);
}, 30_000);

async function renderP0Harness(query = "") {
    const page = await browser.newPage({ viewport: { width: 1280, height: 1000 } });
    await page.goto(server.url.toString() + query);
    await page.getByRole("heading", { name: "立项锁" }).waitFor();
    return page;
}

async function chooseLock(page: Page, label: string, option: string) {
    await page.getByRole("combobox", { name: label, exact: true }).click();
    await page
        .locator(".ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option-content")
        .filter({ hasText: option })
        .first()
        .click();
}

async function chooseStyleCard(page: Page, name: string) {
    await chooseLock(page, "影像基底", "实拍质感");
    await page.locator("button.manju-style-card").filter({ hasText: name }).click();
}

async function confirmSingleSegmentHardLocks(page: Page) {
    await chooseLock(page, "赛道", "国内剧");
    await chooseLock(page, "素材来源", "改编");
    await chooseLock(page, "视频模型", "Seedance 2.5");
    await chooseLock(page, "画幅", "9:16");
    await chooseLock(page, "交付形态", "Markdown");
    await chooseStyleCard(page, "徐克");
    await page.getByRole("spinbutton").fill("15");
}

test("P0 cannot be saved until every hard lock is explicitly confirmed", async () => {
    const page = await renderP0Harness();
    try {
        const lockButton = page.getByRole("button", { name: "锁定 P0", exact: true });
        expect(await lockButton.isDisabled()).toBe(true);

        await confirmSingleSegmentHardLocks(page);

        expect(await lockButton.isEnabled()).toBe(true);
        await lockButton.click();
        await page.waitForFunction(() => window.__manjuLockSubmissions.length === 1);

        const submissions = await page.evaluate(() => window.__manjuLockSubmissions);
        expect(submissions).toHaveLength(1);
        expect([...submissions[0].confirmedLocks].sort()).toEqual([
            "aspectRatio",
            "deliveryMode",
            "source",
            "styleCard",
            "track",
            "videoModel",
        ]);
    } finally {
        await page.close();
    }
}, 30_000);

test("changing style family clears the style card confirmation", async () => {
    const page = await renderP0Harness();
    try {
        const lockButton = page.getByRole("button", { name: "锁定 P0", exact: true });
        await confirmSingleSegmentHardLocks(page);
        expect(await lockButton.isEnabled()).toBe(true);

        await page.locator(".manju-style-family-row .ant-select").click();
        await page
            .locator(".ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option-content")
            .filter({ hasText: "T2" })
            .click();

        expect(await page.locator("button.manju-style-card.is-selected").count()).toBe(0);
        expect(await lockButton.isDisabled()).toBe(true);
    } finally {
        await page.close();
    }
}, 30_000);

test("historical P0 values are not treated as explicit confirmation proof", async () => {
    const page = await renderP0Harness("?legacy");
    try {
        const trackSelect = page
            .locator(".ant-select")
            .filter({ has: page.getByRole("combobox", { name: "赛道", exact: true }) });
        expect(await trackSelect.locator(".ant-select-selection-item").innerText()).toBe("国内剧");
        expect(await page.getByRole("button", { name: "锁定 P0", exact: true }).isDisabled()).toBe(true);
    } finally {
        await page.close();
    }
}, 30_000);
