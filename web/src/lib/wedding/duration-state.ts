import type { WeddingProductionState } from "@/lib/wedding/types";
import type { CanvasNodeData } from "@/types/canvas";

export function syncWeddingVideoDurationNodes(
    nodes: CanvasNodeData[],
    production: WeddingProductionState,
    durationSeconds: number,
): CanvasNodeData[] {
    const videoNodeIds = new Set(
        production.dynasties
            .map((dynasty) => dynasty.videoNodeId)
            .filter(Boolean),
    );
    const seconds = String(durationSeconds);

    return nodes.map((node) => {
        if (!videoNodeIds.has(node.id)) return node;

        const metadata = record(node.metadata);
        const pluginData = record(metadata?.pluginData);
        const variables = record(pluginData?.creationTemplateVariables);
        if (metadata?.seconds === seconds && Number(variables?.duration) === durationSeconds) {
            return node;
        }

        return {
            ...node,
            metadata: {
                ...metadata,
                seconds,
                pluginData: {
                    ...pluginData,
                    creationTemplateVariables: {
                        ...variables,
                        duration: durationSeconds,
                    },
                },
            },
        };
    });
}

function record(value: unknown) {
    return value && typeof value === "object" && !Array.isArray(value)
        ? value as Record<string, unknown>
        : undefined;
}
