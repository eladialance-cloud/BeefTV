import { createReadStream } from "node:fs";
import { stat } from "node:fs/promises";
import { createServer, request as proxyRequest } from "node:http";
import { extname, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const webDir = resolve(fileURLToPath(new URL("..", import.meta.url)));
const distDir = resolve(webDir, "dist");
const host = process.env.CANVAS_PREVIEW_HOST?.trim() || "127.0.0.1";
const port = Number(process.env.CANVAS_PREVIEW_PORT || 3000);
const apiTarget = new URL(process.env.CANVAS_API_PROXY_TARGET?.trim() || "http://127.0.0.1:8080");

const contentTypes = new Map([
    [".css", "text/css; charset=utf-8"],
    [".gif", "image/gif"],
    [".html", "text/html; charset=utf-8"],
    [".ico", "image/x-icon"],
    [".jpeg", "image/jpeg"],
    [".jpg", "image/jpeg"],
    [".js", "text/javascript; charset=utf-8"],
    [".json", "application/json; charset=utf-8"],
    [".map", "application/json; charset=utf-8"],
    [".mjs", "text/javascript; charset=utf-8"],
    [".mp3", "audio/mpeg"],
    [".mp4", "video/mp4"],
    [".png", "image/png"],
    [".svg", "image/svg+xml"],
    [".txt", "text/plain; charset=utf-8"],
    [".wasm", "application/wasm"],
    [".webm", "video/webm"],
    [".webp", "image/webp"],
    [".woff", "font/woff"],
    [".woff2", "font/woff2"],
]);

function safeStaticPath(pathname) {
    let decoded;
    try {
        decoded = decodeURIComponent(pathname);
    } catch {
        return null;
    }
    const candidate = resolve(distDir, `.${decoded}`);
    if (candidate !== distDir && !candidate.startsWith(`${distDir}${sep}`)) {
        return null;
    }
    return candidate;
}

async function fileStats(path) {
    try {
        const value = await stat(path);
        return value.isFile() ? value : null;
    } catch {
        return null;
    }
}

function writeStatus(response, code) {
    response.writeHead(code, { "Content-Type": "text/plain; charset=utf-8" });
    response.end(code === 404 ? "Not Found" : "Preview server error");
}

function proxyAPI(req, res) {
    const target = new URL(req.url, apiTarget);
    const upstream = proxyRequest({
        protocol: target.protocol,
        hostname: target.hostname,
        port: target.port,
        method: req.method,
        path: `${target.pathname}${target.search}`,
        headers: {
            ...req.headers,
            host: target.host,
            "x-forwarded-host": req.headers.host || "",
            "x-forwarded-proto": "http",
        },
    }, (upstreamResponse) => {
        res.writeHead(upstreamResponse.statusCode || 502, upstreamResponse.headers);
        upstreamResponse.pipe(res);
    });
    upstream.on("error", () => {
        if (!res.headersSent) {
            res.writeHead(502, { "Content-Type": "application/json; charset=utf-8" });
        }
        res.end(JSON.stringify({ code: 502, data: null, msg: "预览服务器无法连接后端", reason: "unavailable" }));
    });
    req.pipe(upstream);
}

const server = createServer(async (req, res) => {
    const method = req.method || "GET";
    if (!["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"].includes(method)) {
        writeStatus(res, 404);
        return;
    }
    const url = new URL(req.url || "/", `http://${req.headers.host || `${host}:${port}`}`);
    if (url.pathname === "/api" || url.pathname.startsWith("/api/")) {
        proxyAPI(req, res);
        return;
    }
    if (method !== "GET" && method !== "HEAD") {
        writeStatus(res, 404);
        return;
    }

    const requestedPath = safeStaticPath(url.pathname);
    if (!requestedPath) {
        writeStatus(res, 404);
        return;
    }
    let filePath = requestedPath;
    let stats = await fileStats(filePath);
    if (!stats) {
        filePath = resolve(distDir, "index.html");
        stats = await fileStats(filePath);
    }
    if (!stats) {
        writeStatus(res, 404);
        return;
    }

    res.writeHead(200, {
        "Content-Type": contentTypes.get(extname(filePath).toLowerCase()) || "application/octet-stream",
        "Content-Length": stats.size,
        "Cache-Control": extname(filePath) === ".html" ? "no-cache" : "public, max-age=31536000, immutable",
    });
    if (method === "HEAD") {
        res.end();
        return;
    }
    createReadStream(filePath).pipe(res);
});

server.listen(port, host, () => {
    console.log(`VisiDeep AI preview listening on http://${host}:${port}`);
});
