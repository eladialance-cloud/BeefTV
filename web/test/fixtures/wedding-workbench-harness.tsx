import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes } from "react-router";

import WeddingWorkbenchPage from "@/pages/wedding/workbench";
import { useCanvasStore } from "@/stores/canvas/use-canvas-store";

useCanvasStore.setState({ projects: [], folders: [], hydrated: true });

const queryClient = new QueryClient({
    defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
    },
});

createRoot(document.getElementById("root")!).render(
    <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={["/wedding/c1"]}>
            <Routes>
                <Route path="/wedding/:canvasId" element={<WeddingWorkbenchPage />} />
                <Route path="/canvas/:id" element={<div data-testid="canvas-link-target">canvas</div>} />
                <Route path="/" element={<div>home</div>} />
            </Routes>
        </MemoryRouter>
    </QueryClientProvider>,
);
