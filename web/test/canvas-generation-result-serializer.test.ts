import { expect, test } from "bun:test";

import { runCanvasResultSerially } from "@/services/canvas-generation-result-serializer";

test("serializes result writes per canvas while allowing different canvases to proceed", async () => {
    const events: string[] = [];
    let releaseFirst!: () => void;
    const firstGate = new Promise<void>((resolve) => { releaseFirst = resolve; });

    const first = runCanvasResultSerially("canvas-a", async () => {
        events.push("a1:start");
        await firstGate;
        events.push("a1:end");
    });
    const sameCanvas = runCanvasResultSerially("canvas-a", async () => {
        events.push("a2:start");
        events.push("a2:end");
    });
    const otherCanvas = runCanvasResultSerially("canvas-b", async () => {
        events.push("b1:start");
        events.push("b1:end");
    });

    for (let attempt = 0; attempt < 20 && !events.includes("b1:end"); attempt += 1) await Promise.resolve();
    expect(events).toEqual(["a1:start", "b1:start", "b1:end"]);
    releaseFirst();
    await Promise.all([first, sameCanvas, otherCanvas]);
    expect(events).toEqual(["a1:start", "b1:start", "b1:end", "a1:end", "a2:start", "a2:end"]);
});
