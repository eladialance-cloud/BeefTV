import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App as AntApp } from "antd";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes } from "react-router";

import CanvasPage from "@/pages/canvas/project";
import WeddingWorkbenchPage from "@/pages/wedding/workbench";
import { useCanvasStore } from "@/stores/canvas/use-canvas-store";
import { defaultConfig, useConfigStore } from "@/stores/use-config-store";

useCanvasStore.setState({ projects: [], folders: [], hydrated: true });
if (window.location.pathname.endsWith("/models")) {
    useConfigStore.setState({
        config: {
            ...defaultConfig,
            channels: [{
                id: "beefapi",
                name: "VisiDeep AI",
                baseUrl: "/api/ai/system/beefapi",
                apiKey: "system",
                apiFormat: "openai",
                interfaceType: "chat-completion",
                scope: "system",
                enabled: true,
                models: ["doubao-seedream-5-0", "seedance-2.5", "gemini-3.8-flash", "music-generation"],
                modelProfiles: [
                    { model: "doubao-seedream-5-0", displayName: "Seedream 5.0", capability: "image", protocol: "volcengine-ark-image" },
                    { model: "seedance-2.5", displayName: "Seedance 2.5", capability: "video", protocol: "volcengine-ark-video" },
                    { model: "gemini-3.8-flash", displayName: "Gemini 3.8 Flash", capability: "text", protocol: "chat-completion" },
                    { model: "music-generation", displayName: "Music Generation", capability: "audio", protocol: "async-audio" },
                ],
            }],
            imageModel: "beefapi::doubao-seedream-5-0",
            videoModel: "beefapi::seedance-2.5",
            textModel: "beefapi::gemini-3.8-flash",
            audioModel: "beefapi::music-generation",
            model: "beefapi::gemini-3.8-flash",
        },
    });
}

const queryClient = new QueryClient({
    defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
    },
});

const initialEntry = `${window.location.pathname}${window.location.search}`;

createRoot(document.getElementById("root")!).render(
    <AntApp>
        <QueryClientProvider client={queryClient}>
            <MemoryRouter initialEntries={[initialEntry]}>
                <Routes>
                    <Route path="/canvas/:id" element={<CanvasPage />} />
                    <Route path="/wedding/:canvasId" element={<WeddingWorkbenchPage />} />
                    <Route path="/wedding/:canvasId/models" element={<WeddingWorkbenchPage />} />
                    <Route path="/canvas" element={<div data-testid="canvas-library">canvas library</div>} />
                    <Route path="/" element={<div>home</div>} />
                </Routes>
            </MemoryRouter>
        </QueryClientProvider>
    </AntApp>,
);
