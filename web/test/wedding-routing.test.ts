import { describe, expect, test } from "bun:test";

import { weddingWorkbenchPathForTemplate } from "@/lib/wedding/routing";

describe("wedding workbench routing", () => {
    test("cross-era wedding opens the producer workbench", () => {
        expect(weddingWorkbenchPathForTemplate(
            { itemKey: "cross-era-wedding-volc" },
            "canvas 1",
        )).toBe("/wedding/canvas%201");
    });

    test("other workflows keep the generic canvas route", () => {
        expect(weddingWorkbenchPathForTemplate(
            { itemKey: "wedding-video-guided-wizard" },
            "canvas-2",
        )).toBe("/canvas/canvas-2");
    });

    test("encodes canvas ids in the dedicated path", () => {
        expect(weddingWorkbenchPathForTemplate(
            { itemKey: "cross-era-wedding-volc" },
            "a/b",
        )).toBe("/wedding/a%2Fb");
    });
});
