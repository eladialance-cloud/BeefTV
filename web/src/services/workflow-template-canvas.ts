import { buildCanvasWorkflowFromTemplate, type PublicCreationTemplate } from "@/lib/canvas/creation-template-workflow";
import { buildWeddingProductionFromTemplate } from "@/lib/wedding/template-state";
import { createWorkspaceCanvasProject } from "@/services/workspace-project-repository";

export async function createWorkspaceCanvasFromTemplate(template: PublicCreationTemplate, variables: Record<string, unknown>) {
    const document = buildCanvasWorkflowFromTemplate(template, { variables });
    const weddingProduction = template.itemKey === "cross-era-wedding-volc"
        ? buildWeddingProductionFromTemplate(template, variables, document.nodes, new Date().toISOString())
        : undefined;
    const stamp = new Date().toLocaleString("zh-CN", { hour12: false });
    const created = await createWorkspaceCanvasProject(`${template.name} · ${stamp}`, undefined, {
        nodes: document.nodes,
        connections: document.connections,
        ...(weddingProduction ? { weddingProduction } : {}),
    });
    return { ...created, document };
}
