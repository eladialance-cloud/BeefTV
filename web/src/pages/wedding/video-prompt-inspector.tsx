import { useQuery } from "@tanstack/react-query";
import { LoaderCircle, RotateCcw, Save } from "lucide-react";
import { useEffect, useState } from "react";

import { previewCreationPrompt } from "@/services/api/creation-templates";
import { weddingVideoPrompt } from "@/lib/wedding/prompt";
import type { WeddingProductionState } from "@/lib/wedding/types";

export function VideoPromptInspector({
    templateId,
    dynastyId,
    dynastyName,
    variables,
    production,
    saving,
    feedback,
    onSave,
    onRestore,
}: {
    templateId: string;
    dynastyId: string;
    dynastyName: string;
    variables: Record<string, unknown>;
    production: WeddingProductionState;
    saving: boolean;
    feedback: string;
    onSave: (prompt: string) => Promise<void>;
    onRestore: () => Promise<void>;
}) {
    const preview = useQuery({
        queryKey: ["wedding-video-prompt-preview", templateId, dynastyId, variables],
        queryFn: ({ signal }) => previewCreationPrompt(templateId, {
            nodeRef: "dynastyVideo",
            dynastyId,
            variables,
        }, signal),
        enabled: Boolean(templateId && dynastyId),
    });
    const dynasty = production.dynasties.find((item) => item.id === dynastyId);
    const effectivePrompt = preview.data
        ? weddingVideoPrompt(production, dynastyId, preview.data.defaultPrompt)
        : "";
    const [draft, setDraft] = useState("");

    useEffect(() => {
        setDraft(effectivePrompt);
    }, [effectivePrompt, dynastyId]);

    const promptTooLong = Array.from(draft).length > 12000;
    const canSave = Boolean(draft.trim()) && !promptTooLong && !saving;

    return (
        <aside className="wedding-prompt-inspector">
            <header>
                <div>
                    <small>视频导演</small>
                    <strong>{dynastyName} · 最终模型提示词</strong>
                </div>
                <span className={dynasty?.promptOverride ? "is-overridden" : ""}>
                    {dynasty?.promptOverride ? "制作方覆盖" : "朝代模板"}
                </span>
            </header>

            {preview.isPending ? (
                <div className="wedding-prompt-state">
                    <LoaderCircle className="wedding-spinner" />
                    <span>正在解析服务端默认提示词</span>
                </div>
            ) : preview.isError ? (
                <div className="wedding-prompt-error">
                    <strong>提示词预览失败</strong>
                    <span>{preview.error instanceof Error ? preview.error.message : "请稍后重试"}</span>
                    <button type="button" onClick={() => void preview.refetch()}>重新读取</button>
                </div>
            ) : (
                <>
                    <div className="wedding-prompt-meta">
                        <span>{preview.data?.settings?.duration ? `${String(preview.data.settings.duration)} 秒` : "按节点设置"}</span>
                        <span>{preview.data?.settings?.ratio ? String(preview.data.settings.ratio) : production.source.ratio}</span>
                        <span>{preview.data?.settings?.resolution ? String(preview.data.settings.resolution) : production.source.resolution}</span>
                    </div>
                    <label className="wedding-prompt-editor">
                        <span>最终英文提示词</span>
                        <textarea
                            value={draft}
                            maxLength={12000}
                            onChange={(event) => setDraft(event.currentTarget.value)}
                            aria-label={`${dynastyName}最终英文视频提示词`}
                        />
                    </label>
                    <div className="wedding-prompt-count">
                        <span>{Array.from(draft).length} / 12000</span>
                        {promptTooLong ? <strong>提示词过长</strong> : null}
                    </div>
                    <details className="wedding-prompt-default">
                        <summary>查看朝代默认提示词</summary>
                        <pre>{preview.data?.defaultPrompt}</pre>
                    </details>
                    <div className="wedding-prompt-actions">
                        <button
                            type="button"
                            disabled={!dynasty?.promptOverride || saving}
                            onClick={() => void onRestore()}
                        >
                            <RotateCcw />
                            恢复默认
                        </button>
                        <button
                            type="button"
                            className="is-primary"
                            disabled={!canSave || draft.trim() === effectivePrompt.trim()}
                            onClick={() => void onSave(draft)}
                        >
                            <Save />
                            保存提示词
                        </button>
                    </div>
                    {feedback ? <p className="wedding-prompt-feedback" role="status">{feedback}</p> : null}
                </>
            )}
        </aside>
    );
}
