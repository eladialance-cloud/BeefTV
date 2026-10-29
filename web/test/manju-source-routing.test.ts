import { describe, expect, test } from "bun:test";

import {
    buildManjuScriptRunInput,
    canCreateManjuScriptRun,
} from "@/lib/canvas/manju-script-routing";
import { createManjuScriptRun } from "@/services/api/manju";
import { apiClient } from "@/services/api/request";

describe("manju source routing behavior", () => {
    test("trend run creation stays disabled until a resolution is selected", () => {
        expect(canCreateManjuScriptRun("trend", "")).toBe(false);
        expect(canCreateManjuScriptRun("trend", "original")).toBe(true);
        expect(canCreateManjuScriptRun("trend", "benchmark")).toBe(true);
        expect(canCreateManjuScriptRun("adaptation", "")).toBe(true);
    });

    test("run input always carries both routing fields", () => {
        expect(buildManjuScriptRunInput({
            mode: "STEPWISE",
            source: "adaptation",
            inputKind: "finished_script",
            trendResolution: "",
        })).toEqual({
            mode: "STEPWISE",
            inputKind: "finished_script",
            trendResolution: "",
        });
        expect(buildManjuScriptRunInput({
            mode: "STEPWISE",
            source: "trend",
            inputKind: "outline",
            trendResolution: "benchmark",
        })).toEqual({
            mode: "STEPWISE",
            inputKind: "outline",
            trendResolution: "benchmark",
        });
    });

    test("createManjuScriptRun submits the routing fields in the request body", async () => {
        const originalRequest = apiClient.request;
        let body: unknown;
        apiClient.request = (async (config: { data?: unknown }) => {
            body = config.data;
            return {
                data: {
                    code: 0,
                    data: {
                        item: {
                            run: { id: "run-1" },
                            stages: [],
                            artifacts: [],
                        },
                    },
                    msg: "ok",
                },
                status: 200,
                headers: {},
            };
        }) as typeof apiClient.request;

        try {
            await createManjuScriptRun("project-1", buildManjuScriptRunInput({
                mode: "STEPWISE",
                source: "trend",
                inputKind: "outline",
                trendResolution: "original",
            }));
        } finally {
            apiClient.request = originalRequest;
        }

        expect(body).toEqual({
            mode: "STEPWISE",
            inputKind: "outline",
            trendResolution: "original",
        });
    });
});
