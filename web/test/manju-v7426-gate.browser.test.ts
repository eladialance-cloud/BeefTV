import { afterAll, beforeAll, expect, test } from "bun:test";
import { existsSync, readFileSync } from "node:fs";
import { chromium, type Browser, type Page } from "playwright";

let browser: Browser;
let server: ReturnType<typeof Bun.serve>;

beforeAll(async () => {
    const build = await Bun.build({
        entrypoints: [import.meta.dir + "/fixtures/manju-v7426-gate-harness.tsx"],
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

async function openHarness(mode: string) {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } });
    await page.goto(`${server.url.toString()}?mode=${mode}`);
    return page;
}

async function confirmationRequests(page: Page) {
    return page.evaluate(() => window.__manjuGateRequests.filter((request) =>
        request.method === "post" && request.url.endsWith("/confirm"),
    ));
}

test("V7.4.26 UI exposes no bypass control", async () => {
    const page = await openHarness("missing-p0");
    try {
        await page.getByRole("heading", { name: "立项锁" }).waitFor();
        expect(await page.getByText("跳过官方双机检").count()).toBe(0);
        expect(await page.getByText("没有图片也可以确认进入 P4").count()).toBe(0);
        expect(await page.getByRole("button", { name: "锁定 P0", exact: true }).isDisabled()).toBe(true);
        expect(await page.evaluate(() => window.__manjuGateRequests.some((request) =>
            request.method === "put" && request.url.endsWith("/lock"),
        ))).toBe(false);
    } finally {
        await page.close();
    }
}, 30_000);

test("P3 storyboard execution remains blocked until P0B is locked", async () => {
    const page = await openHarness("missing-p0b");
    try {
        await page.getByText("P0B 锁定后才能执行 P3 分镜。").waitFor();
        expect(await page.getByRole("button", { name: "执行 P3 分镜与故事板", exact: true }).isDisabled()).toBe(true);
        expect(await page.evaluate(() => window.__manjuGateRequests.some((request) =>
            request.url.endsWith("/stages/director_storyboard/run"),
        ))).toBe(false);
    } finally {
        await page.close();
    }
}, 30_000);

test("partial storyboard coverage reaches the real P3 component and server gate", async () => {
    const page = await openHarness("partial-storyboard");
    try {
        await page.getByText("当前已就绪 1 张故事板图。").waitFor();
        expect(await page.getByText("没有图片也可以确认进入 P4").count()).toBe(0);
        const confirm = page.getByRole("button", { name: "确认并锁定，开放下一阶段", exact: true });
        expect(await confirm.isEnabled()).toBe(true);

        await confirm.click();
        await page.getByText("P3 故事板图片未全部就绪，不能锁定进入 P4。").waitFor();
        const requests = await confirmationRequests(page);
        expect(requests).toHaveLength(1);
        expect(requests[0].url).toEndWith("/stages/director_storyboard/confirm");
        expect(requests[0].data).toEqual({ note: "确认 P3 分镜与故事板" });
    } finally {
        await page.close();
    }
}, 30_000);

test("P4 without complete machine-check proof has no skip path and is rejected", async () => {
    const page = await openHarness("missing-p4-proof");
    try {
        await page.getByText("未记录完整四步机检，不能作为 PASS；请重新执行官方 P4 编译链").waitFor();
        expect(await page.getByText("跳过官方双机检").count()).toBe(0);
        const confirm = page.getByRole("button", { name: "确认并锁定，开放下一阶段", exact: true });
        await confirm.click();
        await page.getByText("P4 机检凭证缺失，不能锁定进入 P5。").waitFor();
        const requests = await confirmationRequests(page);
        expect(requests).toHaveLength(1);
        expect(requests[0].url).toEndWith("/stages/feed_package/confirm");
        expect(requests[0].data).toEqual({ note: "确认 P4 投喂封装" });
    } finally {
        await page.close();
    }
}, 30_000);

test("P5 renders a board row only when the backend materialized it", async () => {
    const withBoard = await openHarness("board-delivery");
    try {
        await withBoard.getByText("最终交付物").waitFor();
        await withBoard.getByText("离线 HTML 看板（仅 board/both）").waitFor();
        await withBoard.getByText("V7.4.26-看板.html").waitFor();
    } finally {
        await withBoard.close();
    }

    const withoutBoard = await openHarness("markdown-delivery");
    try {
        await withoutBoard.getByText("最终交付物").waitFor();
        expect(await withoutBoard.getByText("离线 HTML 看板（仅 board/both）").count()).toBe(0);
        expect(await withoutBoard.getByText("V7.4.26-看板.html").count()).toBe(0);
        await withoutBoard.getByText("Markdown 源稿").waitFor();
    } finally {
        await withoutBoard.close();
    }
}, 30_000);
