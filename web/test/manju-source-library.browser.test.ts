import { afterAll, beforeAll, expect, test } from "bun:test";
import { existsSync, readFileSync } from "node:fs";
import { chromium, type Browser } from "playwright";

let browser: Browser;
let server: ReturnType<typeof Bun.serve>;

beforeAll(async () => {
    const build = await Bun.build({
        entrypoints: [import.meta.dir + "/fixtures/manju-source-library-harness.tsx"],
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
            return new Response('<meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/manju.css"><div id="root"></div><script type="module" src="/app.js"></script>', {
                headers: { "Content-Type": "text/html" },
            });
        },
    });
    const executablePath = [process.env.CHROME_PATH, "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"].find((path): path is string => Boolean(path && existsSync(path)));
    browser = await chromium.launch({ executablePath, headless: true });
}, 60_000);

afterAll(async () => {
    await browser?.close();
    server?.stop(true);
}, 30_000);

for (const width of [1440, 390]) {
    test(`原著库章节选择在 ${width}px 内完整显示`, async () => {
        const page = await browser.newPage({ viewport: { width, height: 900 } });
        const errors: string[] = [];
        page.on("pageerror", (error) => errors.push(error.message));
        page.on("console", (item) => {
            if (item.type() === "error") errors.push(item.text());
        });
        try {
            await page.goto(server.url.toString());
            await page.waitForTimeout(500);
            if (errors.length) throw new Error(errors.join("\n"));
            await page.getByText("原著与本次范围", { exact: true }).waitFor();
            await page.getByText("第一章 雨夜", { exact: true }).waitFor();
            await page.getByText("第二章 旧照片", { exact: true }).waitFor();
            await page.getByText("第一章 雨夜", { exact: true }).click();
            await page.getByText("已选择 1 章 · 12,000 字", { exact: true }).waitFor();
            expect(await page.getByRole("button", { name: "确认本次范围" }).isEnabled()).toBe(true);
            await page.getByRole("button", { name: "确认本次范围" }).click();
            await page.getByText("已自动填入第 2 步确认的范围", { exact: false }).waitFor();
            expect(await page.getByRole("button", { name: "创建改编单元" }).isEnabled()).toBe(true);
            const layout = await page.evaluate(() => ({
                scrollWidth: document.documentElement.scrollWidth,
                innerWidth,
                offenders: Array.from(document.querySelectorAll<HTMLElement>("body *"))
                    .map((element) => ({ tag: element.tagName, className: element.className, right: element.getBoundingClientRect().right, width: element.getBoundingClientRect().width }))
                    .filter((item) => item.right > innerWidth + 1)
                    .slice(0, 8),
            }));
            if (layout.scrollWidth > layout.innerWidth + 1) throw new Error(JSON.stringify(layout));
            await page.screenshot({
                path: import.meta.dir + `/../../build/manju-source-library-${width}.png`,
                fullPage: true,
                animations: "disabled",
            });
        } finally {
            await page.close();
        }
    }, 30_000);
}

test("原著库变化后禁用旧全局阶段继承", async () => {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    try {
        await page.goto(server.url.toString() + "?stale");
        await page.getByText("第一章 雨夜", { exact: true }).click();
        await page.getByRole("button", { name: "确认本次范围" }).click();
        await page.getByText("来源库已变化，必须重新生成 IP 与故事圣经", { exact: true }).waitFor();
        const reuse = page.getByRole("checkbox", { name: /来源库已变化/ });
        expect(await reuse.isDisabled()).toBe(true);
    } finally {
        await page.close();
    }
}, 30_000);
