import { afterAll, beforeAll, expect, test } from "bun:test";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { chromium, type Browser } from "playwright";

import { createWeddingProductionState } from "@/lib/wedding/production-state";

let browser: Browser;
let server: ReturnType<typeof Bun.serve>;
let project: Record<string, unknown>;

beforeAll(async () => {
    const build = await Bun.build({
        entrypoints: [import.meta.dir + "/fixtures/wedding-workbench-harness.tsx"],
        target: "browser",
        define: {
            "import.meta.env.DEV": "false",
            "import.meta.env.PROD": "true",
            "import.meta.env.MODE": '"production"',
            "import.meta.env.VITE_CANVAS_LOCAL_MODE": '"true"',
            "import.meta.env.VITE_CANVAS_BACKEND_URL": '"/api"',
            "process.env.NODE_ENV": '"production"',
        },
        plugins: [{
            name: "source",
            setup(builder) {
                builder.onResolve({ filter: /\?url$/ }, () => ({
                    path: "virtual-url",
                    namespace: "test-url",
                }));
                builder.onResolve({ filter: /^@\/components\/model-logo$/ }, () => ({
                    path: "model-logo",
                    namespace: "test-icons",
                }));
                builder.onResolve({ filter: /^@excalidraw\/excalidraw\/index\.css$/ }, () => ({
                    path: "virtual-css",
                    namespace: "test-css",
                }));
                builder.onResolve({ filter: /^@\// }, (args) => ({
                    path: Bun.resolveSync("../src/" + args.path.slice(2), import.meta.dir),
                }));
                builder.onLoad({ filter: /.*/, namespace: "test-url" }, () => ({
                    contents: 'export default "data:application/octet-stream;base64,";',
                    loader: "js",
                }));
                builder.onLoad({ filter: /.*/, namespace: "test-css" }, () => ({
                    contents: "export default '';",
                    loader: "js",
                }));
                builder.onLoad({ filter: /.*/, namespace: "test-icons" }, () => ({
                    contents: "export function ModelLogo() { return null; }",
                    loader: "js",
                }));
                builder.onLoad({ filter: /\.(svg|png|jpe?g|gif|webp|woff2?)$/ }, () => ({
                    contents: "export default ''",
                    loader: "js",
                }));
                builder.onLoad({ filter: /stores[\\/]canvas[\\/]use-canvas-store\.ts$/ }, (args) => ({
                    loader: "ts",
                    contents: readFileSync(args.path, "utf8"),
                }));
            },
        }],
    });
    if (!build.success) throw new Error(build.logs.join("\n"));
    const script = await build.outputs.find((output) => output.path.endsWith(".js"))!.text();
    const css = await build.outputs.find((output) => output.path.endsWith(".css"))?.text() || "";
    const productionCssDir = `${import.meta.dir}/../dist/static`;
    const productionCss = existsSync(productionCssDir)
        ? readdirSync(productionCssDir)
            .filter((file) => file.endsWith(".css"))
            .map((file) => readFileSync(`${productionCssDir}/${file}`, "utf8"))
            .join("\n")
        : "";
    const production = createWeddingProductionState({
        template: { id: "template-1", key: "cross-era-wedding-volc", version: "1.0.0" },
        source: {
            groomPhotoNodeId: "groom",
            bridePhotoNodeId: "bride",
            maleName: "林屿",
            femaleName: "周棠",
            story: "杭州西湖相识",
            consentConfirmed: true,
            ratio: "16:9",
            resolution: "720p",
        },
        dynasties: [{
            id: "tang",
            name: "唐",
            order: 0,
            portraitMaleNodeId: "male",
            portraitFemaleNodeId: "female",
            sceneNodeId: "scene",
            videoNodeId: "video",
        }],
        now: "2026-10-25T00:00:00.000Z",
    });
    production.activeStage = "videos";
    project = {
        id: "c1",
        revision: 1,
        title: "林屿 × 周棠 · 跨时空婚礼电影",
        createdAt: "2026-10-25T00:00:00.000Z",
        updatedAt: "2026-10-25T00:00:00.000Z",
        nodes: [],
        connections: [],
        chatSessions: [],
        activeChatId: null,
        backgroundMode: "grid",
        showImageInfo: false,
        viewport: { x: 0, y: 0, k: 1 },
        directorScenes: [],
        weddingProduction: production,
    };
    const ok = (data: unknown) => Response.json({ code: 0, data, msg: "ok" });
    server = Bun.serve({
        port: 0,
        async fetch(request) {
            const path = new URL(request.url).pathname;
            if (path === "/harness.js") {
                return new Response(script, { headers: { "Content-Type": "application/javascript" } });
            }
            if (path === "/harness.css") {
                return new Response(css, { headers: { "Content-Type": "text/css" } });
            }
            if (path === "/production.css") {
                return new Response(productionCss, { headers: { "Content-Type": "text/css" } });
            }
            if (path === "/api/canvas-projects/c1" && request.method === "GET") {
                return ok({ project });
            }
            if (path === "/api/creation-templates/template-1/prompt-preview" && request.method === "POST") {
                return ok({
                    defaultPrompt: "DEFAULT TANG VIDEO PROMPT",
                    nodeRef: "dynastyVideo",
                    dynastyId: "tang",
                    settings: { duration: 8, ratio: "16:9", resolution: "720p" },
                });
            }
            if (path === "/api/ops/canvas.document.commit" && request.method === "POST") {
                const body = await request.json() as {
                    params: { expectedRevision: number; document: Record<string, unknown> };
                };
                const revision = body.params.expectedRevision + 1;
                project = { ...body.params.document, id: "c1", revision };
                return ok({
                    revision,
                    result: {
                        canvasId: "c1",
                        revision,
                        title: project.title,
                        updatedAt: project.updatedAt,
                    },
                });
            }
            if (path.startsWith("/api/")) return ok({});
            return new Response(
                '<!doctype html><meta name="viewport" content="width=device-width"><style>html,body,#root{min-height:100%;margin:0}body{overflow:hidden}</style><link rel="stylesheet" href="/production.css"><link rel="stylesheet" href="/harness.css"><div id="root"></div><script type="module" src="/harness.js"></script>',
                { headers: { "Content-Type": "text/html" } },
            );
        },
    });
    const executablePath = [
        process.env.CHROME_PATH,
        "C:/Program Files/Google/Chrome/Application/chrome.exe",
        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    ].find((path): path is string => Boolean(path && existsSync(path)));
    browser = await chromium.launch({ executablePath, headless: true });
}, 60000);

afterAll(async () => {
    await browser?.close();
    server?.stop(true);
});

test("producer can load, edit, save and restore the dynasty video prompt", async () => {
    const page = await browser.newPage({ viewport: { width: 1440, height: 960 } });
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(new URL("/canvas/c1?canvasView=wedding", server.url).toString());

    await page.getByTestId("wedding-canvas-overlay").waitFor();
    const closeAgent = page.getByRole("button", { name: "关闭 Agent" });
    if (await closeAgent.count()) await closeAgent.first().click();
    await page.getByRole("heading", { name: "视频导演", exact: true }).waitFor();
    await page.getByLabel("唐最终英文视频提示词").waitFor();
    expect(await page.getByLabel("唐最终英文视频提示词").inputValue()).toBe("DEFAULT TANG VIDEO PROMPT");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.screenshot({
        path: "D:/画布/BeefTV/build/wedding-workbench-1440.png",
        fullPage: true,
    });

    await page.getByLabel("唐最终英文视频提示词").fill("PRODUCER EDITED PROMPT");
    await page.getByRole("button", { name: "保存提示词" }).click();
    await page.getByRole("status").filter({ hasText: "提示词已保存" }).waitFor();

    await page.reload();
    await page.getByLabel("唐最终英文视频提示词").waitFor();
    expect(await page.getByLabel("唐最终英文视频提示词").inputValue()).toBe("PRODUCER EDITED PROMPT");
    expect(errors).toEqual([]);

    await page.getByRole("button", { name: "恢复默认" }).click();
    await page.getByRole("status").filter({ hasText: "已恢复朝代默认提示词" }).waitFor();
    expect(await page.getByLabel("唐最终英文视频提示词").inputValue()).toBe("DEFAULT TANG VIDEO PROMPT");

    await page.setViewportSize({ width: 390, height: 900 });
    await page.waitForTimeout(100);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.screenshot({
        path: "D:/画布/BeefTV/build/wedding-workbench-390.png",
        fullPage: true,
    });
    await page.setViewportSize({ width: 1440, height: 960 });
    await page.getByRole("button", { name: "返回节点图" }).first().click();
    await page.getByTestId("wedding-canvas-overlay").waitFor({ state: "hidden" });
    await page.close();
}, 30000);

test("an existing cross-era canvas backfills production state and reveals the workbench toggle", async () => {
    const currentProduction = project.weddingProduction as {
        source: Record<string, unknown>;
        dynasties: Array<{ id: string; name: string }>;
    };
    const pluginBase = {
        creationTemplateId: "template-1",
        creationTemplateKey: "cross-era-wedding-volc",
        creationTemplateVersion: "1.0.0",
        creationTemplateVariables: {
            maleName: currentProduction.source.maleName,
            femaleName: currentProduction.source.femaleName,
            customDesc: currentProduction.source.story,
            dynasties: currentProduction.dynasties.map((dynasty) => dynasty.id),
            ratio: currentProduction.source.ratio,
            resolution: currentProduction.source.resolution,
            consentConfirmed: currentProduction.source.consentConfirmed,
        },
    };
    const node = (
        id: string,
        type: string,
        title: string,
        metadata: Record<string, unknown>,
        x: number,
    ) => ({
        id,
        type,
        title,
        position: { x, y: 0 },
        width: 320,
        height: 220,
        metadata,
    });
    const dynasty = currentProduction.dynasties[0]!;
    const nextProject = {
        ...project,
        revision: Number(project.revision || 1) + 1,
        nodes: [
            node("groom", "image", "新郎原始照片", {
                uploadField: "groomPhoto",
                pluginData: pluginBase,
            }, 0),
            node("bride", "image", "新娘原始照片", {
                uploadField: "bridePhoto",
                pluginData: pluginBase,
            }, 340),
            node("male-tang", "image", `${dynasty.name} · 新郎肖像`, {
                pluginData: { creationTemplateDynamicRef: "dynastyPortraitMale", creationTemplateDynastyId: dynasty.id, creationTemplateDynastyName: dynasty.name },
            }, 680),
            node("female-tang", "image", `${dynasty.name} · 新娘肖像`, {
                pluginData: { creationTemplateDynamicRef: "dynastyPortraitFemale", creationTemplateDynastyId: dynasty.id, creationTemplateDynastyName: dynasty.name },
            }, 1020),
            node("scene-tang", "image", `${dynasty.name} · 双人婚礼场景`, {
                pluginData: { creationTemplateDynamicRef: "dynastyScene", creationTemplateDynastyId: dynasty.id, creationTemplateDynastyName: dynasty.name },
            }, 1360),
            node("video-tang", "video", `${dynasty.name} · 朝代视频`, {
                pluginData: { creationTemplateDynamicRef: "dynastyVideo", creationTemplateDynastyId: dynasty.id, creationTemplateDynastyName: dynasty.name },
            }, 1700),
        ],
        weddingProduction: undefined,
    };
    project = nextProject;

    const page = await browser.newPage({ viewport: { width: 1440, height: 960 } });
    await page.goto(new URL("/canvas/c1", server.url).toString());
    const openWorkbench = page.getByRole("button", { name: "打开婚礼制作台" }).first();
    await openWorkbench.waitFor();
    await openWorkbench.click();
    await page.getByTestId("wedding-canvas-overlay").waitFor();
    await page.getByRole("heading", { name: "项目设定", exact: true }).waitFor();
    expect(project.weddingProduction).toBeDefined();
    await page.close();
}, 30000);
