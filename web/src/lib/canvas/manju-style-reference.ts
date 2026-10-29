export type ManjuStyleImagePixels = {
    width: number;
    height: number;
    data: Uint8ClampedArray;
};

export type ManjuStyleReferenceAnalysis = {
    brightness: "低调偏暗" | "中间调" | "明亮高调";
    contrast: "低反差" | "中低反差" | "中高反差" | "高反差";
    saturation: "低饱和" | "中等饱和" | "高饱和";
    temperature: "偏冷" | "中性" | "偏暖";
    texture: "柔和平滑" | "中等纹理" | "颗粒或强纹理";
    palette: string[];
    summary: string;
    prompt: string;
};

export async function analyzeManjuStyleReferenceFile(file: Blob): Promise<ManjuStyleReferenceAnalysis> {
    const bitmap = await createImageBitmap(file);
    try {
        const width = 128;
        const height = Math.max(1, Math.round(bitmap.height * width / Math.max(1, bitmap.width)));
        const canvas = document.createElement("canvas");
        canvas.width = width;
        canvas.height = height;
        const context = canvas.getContext("2d", { willReadFrequently: true });
        if (!context) throw new Error("浏览器无法读取风格参考图");
        context.drawImage(bitmap, 0, 0, width, height);
        const pixels = context.getImageData(0, 0, width, height);
        return analyzeManjuStylePixels({ width, height, data: pixels.data });
    } finally {
        bitmap.close();
    }
}

export function analyzeManjuStylePixels(image: ManjuStyleImagePixels): ManjuStyleReferenceAnalysis {
    const count = Math.max(1, image.width * image.height);
    const luminances: number[] = [];
    let luminanceSum = 0;
    let saturationSum = 0;
    let warmthSum = 0;
    let edgeDeltaSum = 0;
    let edgeSamples = 0;
    const buckets = new Map<string, { count: number; r: number; g: number; b: number }>();
    for (let index = 0; index < count; index += 1) {
        const offset = index * 4;
        const r = image.data[offset] ?? 0;
        const g = image.data[offset + 1] ?? 0;
        const b = image.data[offset + 2] ?? 0;
        const max = Math.max(r, g, b);
        const min = Math.min(r, g, b);
        const luminance = 0.2126 * r + 0.7152 * g + 0.0722 * b;
        if (index % image.width > 0) {
            const previousOffset = (index - 1) * 4;
            const previousLuminance = 0.2126 * (image.data[previousOffset] ?? 0) + 0.7152 * (image.data[previousOffset + 1] ?? 0) + 0.0722 * (image.data[previousOffset + 2] ?? 0);
            edgeDeltaSum += Math.abs(luminance - previousLuminance);
            edgeSamples += 1;
        }
        luminanceSum += luminance;
        luminances.push(luminance);
        saturationSum += max === 0 ? 0 : (max - min) / max;
        warmthSum += r - b;
        const key = `${(r >> 5) << 6 | (g >> 5) << 3 | (b >> 5)}`;
        const bucket = buckets.get(key) || { count: 0, r: 0, g: 0, b: 0 };
        bucket.count += 1;
        bucket.r += r;
        bucket.g += g;
        bucket.b += b;
        buckets.set(key, bucket);
    }
    const meanLuminance = luminanceSum / count;
    const variance = luminances.reduce((sum, value) => sum + (value - meanLuminance) ** 2, 0) / count;
    const deviation = Math.sqrt(variance);
    const meanSaturation = saturationSum / count;
    const meanWarmth = warmthSum / count;
    const meanEdgeDelta = edgeDeltaSum / Math.max(1, edgeSamples);
    const brightness = meanLuminance < 82 ? "低调偏暗" : meanLuminance > 174 ? "明亮高调" : "中间调";
    const contrast = deviation < 22 ? "低反差" : deviation < 38 ? "中低反差" : deviation < 58 ? "中高反差" : "高反差";
    const saturation = meanSaturation < 0.22 ? "低饱和" : meanSaturation < 0.48 ? "中等饱和" : "高饱和";
    const temperature = meanWarmth > 16 ? "偏暖" : meanWarmth < -16 ? "偏冷" : "中性";
    const texture = meanEdgeDelta < 4 ? "柔和平滑" : meanEdgeDelta < 14 ? "中等纹理" : "颗粒或强纹理";
    const palette = Array.from(buckets.values())
        .sort((left, right) => right.count - left.count)
        .slice(0, 5)
        .map((bucket) => rgbHex(
            Math.round(bucket.r / bucket.count),
            Math.round(bucket.g / bucket.count),
            Math.round(bucket.b / bucket.count),
        ));
    const summary = `${brightness} · ${contrast} · ${saturation} · ${temperature} · ${texture} · 主色 ${palette.join(" / ")}`;
    return {
        brightness,
        contrast,
        saturation,
        temperature,
        texture,
        palette,
        summary,
        prompt: [
            `参考图风格分析：${summary}。`,
            `整体${temperature}，主色 ${palette.join("、")}，${brightness}，${contrast}，${saturation}，${texture}。`,
            "只迁移光线结构、主辅色、色温、饱和度、材质响应、镜头质感、颗粒与后期风格；",
            "禁止复刻参考图构图、人物身份、服装款式、文字、Logo 与具体场景。",
        ].join(""),
    };
}

function rgbHex(r: number, g: number, b: number) {
    return `#${[r, g, b].map((value) => Math.max(0, Math.min(255, value)).toString(16).padStart(2, "0")).join("").toUpperCase()}`;
}
