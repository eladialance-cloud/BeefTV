import { saveAs } from "file-saver";
import { nanoid } from "nanoid";

import { sanitizeDownloadFileName } from "@/lib/canvas/canvas-media-download";
import { assertUserScope, type CapturedUserScope } from "@/lib/user-scope-guard";
import { getResourceDownloadBlob, importResourceFromUrl, resourceDownloadFileUrl, uploadResourceFile } from "@/services/api/resources";

export type OwnedMediaSaveResult = "saved" | "cancelled";

export const MAX_OWNED_ARTIFACT_BYTES = 32 * 1024 * 1024;

export function isWailsNativeShell() {
    if (typeof window === "undefined") return false;
    return window.location?.protocol === "wails:" || typeof window.go?.main?.DesktopApp?.SaveOwnedMedia === "function" || typeof window.go?.main?.DesktopApp?.SaveOwnedArtifact === "function";
}

export async function downloadOwnedOrBrowserMedia(options: { fileName: string; resourceId?: string; browserUrl?: string }): Promise<OwnedMediaSaveResult> {
    const fileName = sanitizeDownloadFileName(options.fileName);
    if (isWailsNativeShell()) {
        const resourceId = options.resourceId?.trim();
        if (!resourceId) throw new Error("没有可导出的本机文件");
        const save = window.go?.main?.DesktopApp?.SaveOwnedMedia;
        if (!save) throw new Error("当前应用还不能把文件存到所选位置");
        const saved = await save(fileName, resourceId);
        return saved ? "saved" : "cancelled";
    }
    const browserUrl = options.browserUrl?.trim();
    const resourceId = options.resourceId?.trim();
    if (resourceId) {
        startBrowserResourceDownload(resourceDownloadFileUrl(resourceId, fileName), fileName);
        return "saved";
    }
    if (!browserUrl) throw new Error("没有可导出的文件");
    saveAs(await browserMediaDownloadBlob(browserUrl, fileName), fileName);
    return "saved";
}

export async function saveOwnedOrBrowserBlob(fileName: string, blob: Blob): Promise<OwnedMediaSaveResult> {
    const name = sanitizeDownloadFileName(fileName, "未命名导出");
    if (isWailsNativeShell()) {
        if (blob.size > MAX_OWNED_ARTIFACT_BYTES) throw new Error("导出包太大，请减少所选内容后再导出");
        const save = window.go?.main?.DesktopApp?.SaveOwnedArtifact;
        if (!save) throw new Error("当前应用还不能把文件存到所选位置");
        const bytes = new Uint8Array(await blob.arrayBuffer());
        const saved = await save(name, bytesToBase64(bytes));
        return saved ? "saved" : "cancelled";
    }
    saveAs(blob, name);
    return "saved";
}

/** Video exports use the resource upload path so large files never cross Wails as base64. */
export async function saveOwnedOrBrowserVideoBlob(fileName: string, blob: Blob, expectedScope: CapturedUserScope): Promise<OwnedMediaSaveResult> {
    assertUserScope(expectedScope);
    if (!isWailsNativeShell()) return saveOwnedOrBrowserBlob(fileName, blob);
    if (!window.go?.main?.DesktopApp?.SaveOwnedMedia) throw new Error("当前应用还不能把文件存到所选位置");
    const name = sanitizeDownloadFileName(fileName, "未命名导出.mp4");
    const resource = await uploadResourceFile(blob, "video", {
        fileName: name,
        expectedScope,
        idempotencyKey: `video-export:${nanoid()}`,
    });
    assertUserScope(expectedScope);
    return downloadOwnedOrBrowserMedia({ fileName: name, resourceId: resource.id });
}

function bytesToBase64(bytes: Uint8Array) {
    const toBase64 = (bytes as Uint8Array & { toBase64?: () => string }).toBase64;
    if (typeof toBase64 === "function") return toBase64.call(bytes);
    const chunkSize = 0x8000;
    let binary = "";
    for (let offset = 0; offset < bytes.length; offset += chunkSize) {
        binary += String.fromCharCode(...bytes.subarray(offset, offset + chunkSize));
    }
    return btoa(binary);
}

async function browserMediaDownloadBlob(browserUrl: string, fileName: string) {
    try {
        const response = await fetch(browserUrl, { credentials: "include" });
        if (response.ok) {
            const contentType = (response.headers.get("content-type") || "").toLowerCase();
            const blob = await response.blob();
            if (blob.size > 0 && !contentType.includes("application/json") && !contentType.includes("text/html")) return blob;
        }
    } catch {
        // Cross-origin URLs often reject browser fetch; fall back to the controlled server import below.
    }
    if (/^https?:\/\//i.test(browserUrl)) {
        const imported = await importResourceFromUrl(browserUrl, browserDownloadKind(fileName), {
            idempotencyKey: `download-import:${browserUrl}`,
        });
        return getResourceDownloadBlob(imported.id);
    }
    throw new Error("该资产只有已失效的远程地址，请重新保存或替换原文件");
}

function startBrowserResourceDownload(url: string, fileName: string) {
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = fileName;
    anchor.rel = "noopener";
    anchor.style.display = "none";
    document.body.append(anchor);
    anchor.click();
    window.setTimeout(() => anchor.remove(), 60_000);
}

function browserDownloadKind(fileName: string): "image" | "video" | "audio" | "file" {
    const extension = fileName.split(".").pop()?.toLowerCase() || "";
    if (["png", "jpg", "jpeg", "webp", "gif", "svg", "avif"].includes(extension)) return "image";
    if (["mp4", "mov", "webm", "m4v", "avi"].includes(extension)) return "video";
    if (["mp3", "wav", "m4a", "aac", "ogg", "flac"].includes(extension)) return "audio";
    return "file";
}

export function reportOwnedMediaSave(message: { success: (text: string) => void; error: (text: string) => void }, result: Promise<OwnedMediaSaveResult>) {
    return result
        .then((status) => {
            if (status === "saved") message.success(isWailsNativeShell() ? "已保存到所选位置" : "下载已开始");
        })
        .catch((error) => {
            message.error(error instanceof Error && error.message.trim() ? error.message : "保存没有完成，请再试一次");
        });
}
