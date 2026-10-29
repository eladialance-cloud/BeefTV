const canvasResultTails = new Map<string, Promise<void>>();

export function runCanvasResultSerially<T>(canvasId: string, run: () => Promise<T>): Promise<T> {
    const previous = canvasResultTails.get(canvasId) || Promise.resolve();
    const next = previous.catch(() => undefined).then(run);
    const tail = next.then(() => undefined, () => undefined).finally(() => {
        if (canvasResultTails.get(canvasId) === tail) canvasResultTails.delete(canvasId);
    });
    canvasResultTails.set(canvasId, tail);
    return next;
}
