import { afterAll, beforeAll, expect, test } from "bun:test";
import { existsSync, readFileSync } from "node:fs";
import { chromium, type Browser, type Page } from "playwright";

let browser: Browser;
let server: ReturnType<typeof Bun.serve>;

beforeAll(async () => {
    const build = await Bun.build({
        entrypoints: [import.meta.dir + "/fixtures/manju-source-routing-harness.tsx"],
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

async function chooseOption(page: Page, label: string, option: string) {
    await page.getByRole("combobox", { name: label, exact: true }).click();
    await page
        .locator(".ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option-content")
        .filter({ hasText: option })
        .first()
        .click();
}

test("trend run creation requires and submits explicit source routing", async () => {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    try {
        await page.goto(server.url.toString());
        const createButton = page.getByRole("button", { name: "开始 P1", exact: true });
        await createButton.waitFor();
        expect(await createButton.isDisabled()).toBe(true);

        await chooseOption(page, "趋势落点", "选定方向后按原创执行");
        expect(await createButton.isEnabled()).toBe(true);
        await createButton.click();
        await page.waitForFunction(() => window.__manjuScriptRunSubmissions.length === 1);

        const submission = await page.evaluate(() => window.__manjuScriptRunSubmissions[0]);
        expect(submission.mode).toBe("STEPWISE");
        expect(submission.inputKind).toBe("novel");
        expect(submission.trendResolution).toBe("original");
    } finally {
        await page.close();
    }
}, 30_000);
