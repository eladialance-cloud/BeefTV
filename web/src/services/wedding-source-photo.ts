import { readImageMeta } from "@/lib/image-utils";
import { assertUserScope, captureUserScope } from "@/lib/user-scope-guard";
import { resourceFileUrl, resourceStorageKey, uploadResourceFile } from "@/services/api/resources";
import { persistWorkspaceAssetLink } from "@/services/workspace-asset-repository";
import { useAssetStore } from "@/stores/use-asset-store";

export type UploadedWeddingSourcePhoto = {
    assetId: string;
    resourceId: string;
    storageKey: string;
    url: string;
    mimeType: string;
    bytes: number;
    width: number;
    height: number;
};

export async function uploadWeddingSourcePhoto(
    file: File,
    label: string,
    fieldKey: "groomPhoto" | "bridePhoto",
): Promise<UploadedWeddingSourcePhoto> {
    if (!isSupportedImage(file)) throw new Error(`请选择 JPG、PNG 或 WebP 照片：${file.name}`);
    if (file.size > 50 * 1024 * 1024) throw new Error(`照片不能超过 50MB：${file.name}`);
    const expectedScope = captureUserScope();
    const objectUrl = URL.createObjectURL(file);
    let imageMeta: Awaited<ReturnType<typeof readImageMeta>>;
    try {
        imageMeta = await readImageMeta(objectUrl);
    } finally {
        URL.revokeObjectURL(objectUrl);
    }
    const resource = await uploadResourceFile(file, "image", {
        fileName: file.name,
        width: imageMeta.width,
        height: imageMeta.height,
        expectedScope,
    });
    assertUserScope(expectedScope);
    const storageKey = resourceStorageKey(resource.id);
    const url = resourceFileUrl(resource.id);
    const assetId = useAssetStore.getState().addAsset({
        kind: "image",
        title: file.name.replace(/\.[^.]+$/, "") || label,
        coverUrl: url,
        tags: ["婚礼原始照片"],
        category: "character",
        status: "confirmed",
        source: "婚礼项目设定上传",
        metadata: { source: "wedding-workbench", uploadField: fieldKey },
        data: {
            dataUrl: url,
            storageKey,
            width: resource.width || imageMeta.width,
            height: resource.height || imageMeta.height,
            bytes: resource.size || file.size,
            mimeType: resource.mimeType || imageMimeType(file),
        },
    });
    const asset = useAssetStore.getState().assets.find((item) => item.id === assetId);
    if (!asset) throw new Error("照片登记到素材库失败");
    await persistWorkspaceAssetLink({
        asset,
        source: "uploaded",
        forceRemote: true,
        expectedScope,
    });
    assertUserScope(expectedScope);
    return {
        assetId,
        resourceId: resource.id,
        storageKey,
        url,
        mimeType: resource.mimeType || imageMimeType(file),
        bytes: resource.size || file.size,
        width: resource.width || imageMeta.width,
        height: resource.height || imageMeta.height,
    };
}

function isSupportedImage(file: File) {
    return file.type.startsWith("image/") || /\.(?:jpe?g|png|webp)$/i.test(file.name);
}

function imageMimeType(file: File) {
    if (file.type.startsWith("image/")) return file.type;
    if (/\.png$/i.test(file.name)) return "image/png";
    if (/\.webp$/i.test(file.name)) return "image/webp";
    return "image/jpeg";
}
