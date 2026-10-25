import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App as AntApp } from "antd";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes } from "react-router";

import CanvasPage from "@/pages/canvas/project";
import { useCanvasStore } from "@/stores/canvas/use-canvas-store";

useCanvasStore.setState({ projects: [], folders: [], hydrated: true });

const queryClient = new QueryClient({
    defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
    },
});

createRoot(document.getElementById("root")!).render(
    <AntApp>
        <QueryClientProvider client={queryClient}>
            <MemoryRouter initialEntries={["/canvas/c1?canvasView=wedding"]}>
                <Routes>
                    <Route path="/canvas/:id" element={<CanvasPage />} />
                    <Route path="/canvas" element={<div data-testid="canvas-library">canvas library</div>} />
                    <Route path="/" element={<div>home</div>} />
                </Routes>
            </MemoryRouter>
        </QueryClientProvider>
    </AntApp>,
);
