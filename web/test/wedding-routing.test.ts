import { describe, expect, test } from "bun:test";

import {
    legacyWeddingWorkbenchPath,
    weddingModelsPath,
    weddingTemplateInitialVariables,
    weddingWorkbenchPathForTemplate,
} from "@/lib/wedding/routing";

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

    test("legacy wedding route maps back to the canvas workbench view", () => {
        expect(legacyWeddingWorkbenchPath("a/b"))
            .toBe("/wedding/a%2Fb");
    });

    test("opens the dedicated wedding model selection page", () => {
        expect(weddingModelsPath("a/b")).toBe("/wedding/a%2Fb/models");
    });

    test("cross-era wedding creation starts with template defaults and no entry form", () => {
        const variables = weddingTemplateInitialVariables({
            payloadJson: JSON.stringify({
                variables: {
                    maleName: "新郎",
                    femaleName: "新娘",
                    customDesc: "",
                    dynasties: ["tang", "ming", "modern"],
                    ratio: "16:9",
                    duration: 8,
                    resolution: "720p",
                },
            }),
        });

        expect(variables).toEqual({
            maleName: "新郎",
            femaleName: "新娘",
            customDesc: "",
            dynasties: ["tang", "ming", "modern"],
            ratio: "16:9",
            duration: 8,
            resolution: "720p",
            consentConfirmed: false,
        });
    });
});
