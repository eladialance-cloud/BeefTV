import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { App } from "antd";
import { useNavigate } from "react-router";

import { CanvasWorkflowSetupModal } from "@/components/canvas/canvas-workflow-setup-modal";
import type { PublicCreationTemplate } from "@/lib/canvas/creation-template-workflow";
import { useCanvasStore } from "@/stores/canvas/use-canvas-store";
import { useUserStore } from "@/stores/use-user-store";
import { listPublicCreationTemplates } from "@/services/api/creation-templates";
import { listWorkspaceCanvasProjectsPage, type CanvasLibrarySummary } from "@/services/api/workspace-data";
import { createWorkspaceCanvasFromTemplate } from "@/services/workflow-template-canvas";
import { isLocalWorkspaceMode } from "@/services/workspace-mode";
import { listCanvasWorkspaceProjectRoots, previewNodesForWorkspaceProject } from "@/lib/canvas/canvas-workspace-project";
import {
    weddingTemplateInitialVariables,
    weddingWorkbenchPathForTemplate,
} from "@/lib/wedding/routing";
import { HomeDashboard } from "./home-dashboard";
import "./home-dashboard.css";

export default function HomePage() {
    const { message } = App.useApp();
    const navigate = useNavigate();
    const userId = useUserStore((state) => state.user?.id);
    const storageMode = useUserStore((state) => state.storageMode);
    const sessionHydrated = useUserStore((state) => state.hydrated);
    const localProjects = useCanvasStore((state) => state.projects);
    const localHydrated = useCanvasStore((state) => state.hydrated);
    const localMode = isLocalWorkspaceMode() || storageMode === "local";
    const [setupTemplate, setSetupTemplate] = useState<PublicCreationTemplate | null>(null);
    const [creatingWorkflow, setCreatingWorkflow] = useState(false);
    const query = useQuery({
        queryKey: ["beeftv-home-canvases", userId],
        queryFn: () => listWorkspaceCanvasProjectsPage({ page: 1, pageSize: 4, sort: "updated" }),
        enabled: !localMode && Boolean(userId) && sessionHydrated,
    });
    const localSummaries = useMemo<CanvasLibrarySummary[]>(() => listCanvasWorkspaceProjectRoots(localProjects)
        .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
        .slice(0, 4)
        .map((project) => ({
            ...project,
            nodeCount: project.nodes.length,
            previewNodes: previewNodesForWorkspaceProject(localProjects, project.id),
        })), [localProjects]);
    const projects = localMode ? localSummaries : query.data?.projects || [];
    const workflowsQuery = useQuery({
        queryKey: ["home-public-workflow-templates"],
        queryFn: ({ signal }) => listPublicCreationTemplates({ kind: "workflow", signal }),
        enabled: !localMode && Boolean(userId) && sessionHydrated,
        staleTime: 60_000,
    });
    const createWorkflow = async (template: PublicCreationTemplate, variables: Record<string, unknown>) => {
        setCreatingWorkflow(true);
        try {
            const created = await createWorkspaceCanvasFromTemplate(template, variables);
            setSetupTemplate(null);
            navigate(weddingWorkbenchPathForTemplate(template, created.id));
        } catch (error) {
            message.error(error instanceof Error ? error.message : "工作流画布创建失败，请重试");
        } finally {
            setCreatingWorkflow(false);
        }
    };
    const submitWorkflow = async (variables: Record<string, unknown>) => {
        if (!setupTemplate) return;
        await createWorkflow(setupTemplate, variables);
    };
    const selectWorkflow = (template: PublicCreationTemplate) => {
        if (template.itemKey === "cross-era-wedding-volc") {
            void createWorkflow(template, weddingTemplateInitialVariables(template));
            return;
        }
        setSetupTemplate(template);
    };

    return (
        <>
            <HomeDashboard
                projects={projects}
                loading={!sessionHydrated || !localHydrated || (!localMode && Boolean(userId) && query.isPending)}
                error={!localMode && query.isError}
                onRetry={() => void query.refetch()}
                workflows={localMode ? [] : workflowsQuery.data || []}
                workflowsLoading={!localMode && Boolean(userId) && workflowsQuery.isPending}
                workflowsError={!localMode && workflowsQuery.isError}
                onRetryWorkflows={() => void workflowsQuery.refetch()}
                onSelectWorkflow={selectWorkflow}
            />
            <CanvasWorkflowSetupModal
                open={Boolean(setupTemplate)}
                template={setupTemplate}
                submitting={creatingWorkflow}
                onCancel={() => {
                    if (!creatingWorkflow) setSetupTemplate(null);
                }}
                onSubmit={(variables) => void submitWorkflow(variables)}
            />
        </>
    );
}
