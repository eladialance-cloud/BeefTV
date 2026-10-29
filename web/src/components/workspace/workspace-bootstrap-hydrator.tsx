import type { ReactNode } from "react";
import { useEffect, useRef, useState } from "react";

import { FullScreenLoader } from "@/components/ui/aceternity/full-screen-loader";
import { WorkspaceErrorState } from "@/components/layout/workspace-state";
import { appPathname } from "@/lib/app-routing";
import { isPublicAuthPath } from "@/lib/auth-routing";
import { preloadWorkspaceRoute } from "@/lib/workspace-route-modules";
import { applyUserSession, localWorkspaceConfig } from "@/lib/user-session";
import { getWorkspaceBootstrap, type WorkspaceBootstrapPayload } from "@/services/api/workspace";
import { commitModelConfig, flushModelConfig, hydrateModelConfig } from "@/services/model-config-repository";
import { normalizeConfigSnapshot, useConfigStore } from "@/stores/use-config-store";
import { useUserStore } from "@/stores/use-user-store";

export function WorkspaceBootstrapHydrator({ children }: { children: ReactNode }) {
    const hydrated = useUserStore((state) => state.hydrated);
    const modelConfigReady = useRef(false);
    const remoteMode = useRef(false);
    const [restoreStatus, setRestoreStatus] = useState<"loading" | "ready" | "error">("loading");

    useEffect(() => {
        let cancelled = false;
        // 登录/注册页在会话建立前渲染，不能触发 bootstrap，否则 401 会把用户推回登录页。
        if (isPublicAuthPath()) {
            useUserStore.getState().setHydrated(true);
            setRestoreStatus("ready");
            return () => {
                cancelled = true;
            };
        }
        const loadWorkspace = () => {
            const bootstrap = getWorkspaceBootstrap();
            const timeout = new Promise<never>((_, reject) => window.setTimeout(() => reject(new Error("工作区启动超时")), 15_000));
            return Promise.race([bootstrap, timeout]);
        };
        void initializeWorkspaceState({
            loadWorkspace,
            createLocalWorkspace: createLocalWorkspacePayload,
            applySession: applyUserSession,
            restoreModelConfig: hydrateLocalModelConfig,
            // 托管网页构建（VITE_CANVAS_LOCAL_MODE=false）没有本地工作区回退：
            // 会话缺失必须去登录页，不能静默退化成单机画布。
            allowLocalFallback: import.meta.env.VITE_CANVAS_LOCAL_MODE !== "false",
            onProfile: (profile) => {
                remoteMode.current = profile === "remote";
            },
        })
            .then(() => {
                if (cancelled) return;
                modelConfigReady.current = true;
                setRestoreStatus("ready");
                preloadWorkspaceRoute(appPathname());
            })
            .catch((error: unknown) => {
                if (cancelled) return;
                if (isUnauthorizedError(error)) {
                    const next = encodeURIComponent(appPathname());
                    window.location.assign(`/login?next=${next}`);
                    return;
                }
                // The browser cache deliberately has no credentials. If the
                // canonical file could not be read, never autosave that cache
                // over the existing workspace when the backend comes back.
                modelConfigReady.current = false;
                useUserStore.getState().setHydrated(true);
                setRestoreStatus("error");
            });
        return () => {
            cancelled = true;
        };
    }, []);

    useEffect(() => {
        let ready = false;
        const unsubscribe = useConfigStore.subscribe((state) => {
            // 托管模式下模型目录由管理后台下发，用户的浏览器不能写回。
            if (remoteMode.current) return;
            if (!shouldSaveLocalModelConfig({ subscriptionReady: ready, modelConfigReady: modelConfigReady.current, channelCount: state.config.channels.length })) return;
            void commitModelConfig(state.config);
        });
        const flush = () => { void flushModelConfig(); };
        const flushIfLocal = () => {
            if (!remoteMode.current) flush();
        };
        window.addEventListener("pagehide", flushIfLocal);
        const markReady = () => { ready = true; };
        if (modelConfigReady.current) markReady();
        else window.setTimeout(markReady, 0);
        return () => {
            unsubscribe();
            window.removeEventListener("pagehide", flushIfLocal);
            flushIfLocal();
        };
    }, []);

    if (restoreStatus === "error") return <WorkspaceErrorState title="工作区暂时无法加载" description="项目或模型配置未能完成读取。原有配置不会被覆盖，请重新加载后再试。" onRetry={() => window.location.reload()} />;
    return hydrated && restoreStatus === "ready" ? children : <FullScreenLoader label="正在准备工作区" detail="加载项目、画布与模型目录" />;
}

export async function initializeWorkspaceState<T>({
    loadWorkspace,
    createLocalWorkspace,
    applySession,
    restoreModelConfig,
    onProfile,
    allowLocalFallback = true,
}: {
    loadWorkspace: () => Promise<T>;
    createLocalWorkspace: () => T;
    applySession: (payload: T) => Promise<void>;
    restoreModelConfig: () => Promise<void>;
    onProfile?: (profile: "local" | "remote") => void;
    allowLocalFallback?: boolean;
}) {
    let profile: "local" | "remote" = "local";
    try {
        try {
            const payload = await loadWorkspace();
            profile = workspaceProfileOf(payload);
            onProfile?.(profile);
            await applySession(payload);
        } catch (error) {
            if (!allowLocalFallback) throw error;
            // Keep the desktop usable while the embedded backend is still
            // starting. There is no remote/account fallback in a local build.
            profile = "local";
            onProfile?.("local");
            await applySession(createLocalWorkspace());
        }
    } finally {
        // The provider snapshot is an independent, canonical desktop file.
        // Browser persistence may be corrupt or unavailable; that must never
        // prevent configured channels and credentials from being restored.
        // 托管模式没有桌面端模型配置文件；系统渠道已由 bootstrap 下发，不能写回本地快照。
        if (profile === "local") await restoreModelConfig();
    }
}

export function workspaceProfileOf(payload: unknown): "local" | "remote" {
    const profile = (payload as { profile?: unknown } | null)?.profile;
    return profile === "remote" ? "remote" : "local";
}

export function isUnauthorizedError(error: unknown) {
    const status = (error as { status?: number } | null)?.status;
    return status === 401 || status === 403;
}
export function shouldSaveLocalModelConfig({ subscriptionReady, modelConfigReady, channelCount }: { subscriptionReady: boolean; modelConfigReady: boolean; channelCount: number }) {
    return subscriptionReady && modelConfigReady && channelCount > 0;
}

async function hydrateLocalModelConfig() {
    const result = await hydrateModelConfig();
    const normalizedConfig = localWorkspaceConfig(normalizeConfigSnapshot({ config: result.config }).config);
    useConfigStore.getState().replaceConfig(normalizedConfig);
    if (shouldPersistHydratedModelConfig(result.health)) await commitModelConfig(normalizedConfig);
}

export function shouldPersistHydratedModelConfig(health: string) {
    return health === "migrated" || health === "recovered";
}

function createLocalWorkspacePayload(): WorkspaceBootstrapPayload {
    return {
        contractVersion: 1,
        profile: "local",
        capabilities: {
            localAssets: true,
            providerCalls: true,
        },
        user: { id: "local", username: "local", displayName: "本地工作区", role: "user", status: "active", createdAt: "", updatedAt: "" },
        workspace: { id: "local", name: "本地工作区", owner: "local", storage: "sqlite" },
        storageMode: "local",
        features: { shortDramaEnabled: true, taskCenterEnabled: true, customChannelsEnabled: true, frontendModelsEnabled: false, pluginCenterEnabled: true, systemPluginsVisibleToUsers: true },
    };
}
