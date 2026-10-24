import { ArrowLeft } from "lucide-react";
import { Link } from "react-router";

export function WorkbenchTopBar({
    canvasId,
    stageLabel,
}: {
    canvasId: string;
    stageLabel: string;
}) {
    return (
        <header className="wedding-workbench-topbar">
            <div>
                <span>制作方工作台</span>
                <strong>{stageLabel}</strong>
            </div>
            <Link to={`/canvas/${encodeURIComponent(canvasId)}`}>
                <ArrowLeft />
                打开底层画布
            </Link>
        </header>
    );
}
