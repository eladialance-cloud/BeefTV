import { describe, expect, test } from "bun:test";

import { analyzeManjuStylePixels } from "@/lib/canvas/manju-style-reference";

function image(width: number, height: number, pixel: (x: number, y: number) => [number, number, number, number]) {
    const data = new Uint8ClampedArray(width * height * 4);
    for (let y = 0; y < height; y += 1) {
        for (let x = 0; x < width; x += 1) {
            data.set(pixel(x, y), (y * width + x) * 4);
        }
    }
    return { width, height, data };
}

describe("manju style reference analysis", () => {
    test("extracts a strong visual style prompt from cool dark pixels", () => {
        const analysis = analyzeManjuStylePixels(image(64, 64, (x, y) => {
            const value = Math.round(12 + ((x + y) / 126) * 70);
            return [value, value + 12, value + 28, 255];
        }));

        expect(analysis.temperature).toBe("偏冷");
        expect(analysis.brightness).toBe("低调偏暗");
        expect(analysis.contrast).toBe("低反差");
        expect(analysis.palette.length).toBeGreaterThan(0);
        expect(analysis.prompt).toContain("整体偏冷");
        expect(analysis.prompt).toContain("主色");
        expect(analysis.prompt).toContain("只迁移光线");
        expect(analysis.prompt).toContain("禁止复刻参考图构图");
    });

    test("detects warm high-saturation reference images", () => {
        const analysis = analyzeManjuStylePixels(image(64, 64, (x) => x < 32 ? [220, 110, 35, 255] : [245, 165, 70, 255]));

        expect(analysis.temperature).toBe("偏暖");
        expect(analysis.saturation).toBe("高饱和");
        expect(analysis.prompt).toContain("饱和");
    });
});
