import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { chromium } from "playwright";

const baseURL = process.env.MANJU_CANVAS_SMOKE_BASE_URL || "http://127.0.0.1:3000";
const outputDir = resolve(import.meta.dir, "../../build/visual");
const executablePath = process.env.MANJU_CANVAS_SMOKE_BROWSER || "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe";
const credentials = [
    { username: process.env.MANJU_CANVAS_SMOKE_USERNAME || "visideep-admin", password: process.env.MANJU_CANVAS_SMOKE_PASSWORD || "VisiDeep-Local-2026!" },
    { username: "e2e_admin_1018", password: "VisiDeep-E2E-1018!" },
];

await mkdir(outputDir, { recursive: true });
const browser = await chromium.launch({ executablePath, headless: true });

try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 1 });
    const page = await context.newPage();
    const cookieFile = Bun.file("D:/画布/_manual/admin-cookie-current.txt");
    if (await cookieFile.exists()) {
        const row = (await cookieFile.text()).split(/\r?\n/).find((line) => line.includes("\tbeeftv_session\t"));
        const value = row?.split("\t")[6]?.trim();
        if (value) await context.addCookies([{ name: "beeftv_session", value, domain: "127.0.0.1", path: "/", httpOnly: true, sameSite: "Lax" }]);
    }
    const cookieProjects = await context.request.get(`${baseURL}/api/manju/projects`);
    let loggedIn = cookieProjects.ok();
    for (const credential of loggedIn ? [] : credentials) {
        await page.goto(`${baseURL}/login?next=%2Fmanju`, { waitUntil: "domcontentloaded" });
        await page.locator('input[autocomplete="username"]').fill(credential.username);
        await page.locator('input[autocomplete="current-password"]').fill(credential.password);
        await page.locator('button[type="submit"]').click();
        await page.waitForTimeout(800);
        if (!new URL(page.url()).pathname.startsWith("/login")) {
            loggedIn = true;
            break;
        }
    }
    if (!loggedIn) throw new Error("No local smoke-test account could log in");

    const projectsResponse = await context.request.get(`${baseURL}/api/manju/projects`);
    if (!projectsResponse.ok()) throw new Error(`Project list failed: ${projectsResponse.status()}`);
    const projectsPayload = await projectsResponse.json();
    const projects = projectsPayload.data?.items || projectsPayload.items || [];
    if (!projects.length) throw new Error("No Manju project exists for visual smoke testing");
    const project = projects[0];
    await page.goto(`${baseURL}/manju/${project.id}?tab=stages&stage=p2`, { waitUntil: "domcontentloaded" });
    await page.locator(".manju-asset-workspace").waitFor({ timeout: 20_000 });
    const panelCount = await page.locator(".manju-canvas-package").count();
    const metrics = await page.evaluate(() => ({ scrollWidth: document.documentElement.scrollWidth, clientWidth: document.documentElement.clientWidth }));
    const screenshot = resolve(outputDir, "manju-canvas-package-desktop.png");
    await page.screenshot({ path: screenshot, fullPage: true });
    console.log(JSON.stringify({ projectID: project.id, projectName: project.name, panelCount, metrics, screenshot }, null, 2));
} finally {
    await browser.close();
}
