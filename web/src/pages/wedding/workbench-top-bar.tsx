import { ArrowLeft, SlidersHorizontal } from "lucide-react";
import { Link } from "react-router";

export function WorkbenchTopBar({
    canvasId,
    stageLabel,
    modelsHref,
}: {
    canvasId: string;
    stageLabel: string;
    modelsHref?: string;
}) {
    return (
        <header className="wedding-workbench-topbar">
            <div>
                <span>制作方工作台</span>
                <strong>{stageLabel}</strong>
            </div>
            <div className="wedding-workbench-topbar-actions">
                {modelsHref ? <Link to={modelsHref}><SlidersHorizontal />模型选择</Link> : null}
                <Link to={`/canvas/${encodeURIComponent(canvasId)}`}>
                    <ArrowLeft />
                    打开底层画布
                </Link>
            </div>
        </header>
    );
}
