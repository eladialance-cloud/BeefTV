import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Input } from "antd";
import { RefreshCw } from "lucide-react";
import { AppModal } from "@/components/ui/product/app-modal";
import type { ManjuCompilePreview } from "@/services/api/manju";

type ManjuArtifactEditorProps = {
    open: boolean;
    title: string;
    content: string;
    validationJson?: string;
    loading?: boolean;
    preview?: ManjuCompilePreview;
    previewLoading?: boolean;
    onCancel: () => void;
    onSubmit: (content: string) => void;
    onPreview?: (content: string) => void;
};

export function ManjuArtifactEditor({
    open,
    title,
    content,
    validationJson,
    loading = false,
    preview,
    previewLoading = false,
    onCancel,
    onSubmit,
    onPreview,
}: ManjuArtifactEditorProps) {
    const [draft, setDraft] = useState(content);
    useEffect(() => {
        if (open) setDraft(content);
    }, [content, open]);
    const issues = useMemo(() => validationIssues(validationJson), [validationJson]);
    return (
        <AppModal
            open={open}
            title={title}
            width={980}
            onCancel={onCancel}
            footer={
                <div className="manju-artifact-editor-actions">
                    <Button onClick={onCancel}>取消</Button>
                    {onPreview ? (
                        <Button loading={previewLoading} onClick={() => onPreview(draft)}>
                            查看编译后正文
                        </Button>
                    ) : null}
                    <Button
                        type="primary"
                        icon={<RefreshCw />}
                        loading={loading}
                        disabled={!draft.trim()}
                        onClick={() => onSubmit(draft)}
                    >
                        校验并保存
                    </Button>
                </div>
            }
        >
            <div className="manju-artifact-editor">
                <Alert
                    showIcon
                    type="info"
                    message="保存后会重新运行当前阶段对应的官方机检；通过后仍需人工确认，不会自动锁定。"
                />
                {issues.length ? (
                    <div className="manju-artifact-editor-issues">
                        <strong>上次校验问题</strong>
                        <ul>
                            {issues.slice(0, 8).map((issue) => (
                                <li key={issue}>{issue}</li>
                            ))}
                        </ul>
                    </div>
                ) : null}
                {preview ? (
                    <div className="manju-artifact-editor-preview">
                        <div className="manju-artifact-editor-preview-head">
                            <strong>编译后投喂单元</strong>
                            <span>
                                {preview.model} · 硬上限 {preview.hardLimit} 字
                            </span>
                        </div>
                        {preview.segments.map((segment) => (
                            <details key={segment.segmentCode} open={segment.overHardLimit}>
                                <summary className={segment.overHardLimit ? "is-over" : ""}>
                                    <span>{segment.segmentCode}</span>
                                    <strong>
                                        {segment.totalChars}/{preview.hardLimit}
                                    </strong>
                                </summary>
                                <div className="manju-artifact-editor-breakdown">
                                    <span>画风 {segment.style}</span>
                                    <span>人物 {segment.characters}</span>
                                    <span>场景道具 {segment.sceneProps}</span>
                                    <span>分镜 {segment.shots}</span>
                                    <span>其他 {segment.bansAndOther}</span>
                                </div>
                                <pre>{segment.body}</pre>
                            </details>
                        ))}
                    </div>
                ) : null}
                <label>
                    <span>工件正文</span>
                    <Input.TextArea
                        aria-label="工件正文"
                        className="manju-artifact-editor-text"
                        value={draft}
                        onChange={(event) => setDraft(event.target.value)}
                        autoSize={{ minRows: 20, maxRows: 30 }}
                    />
                </label>
            </div>
        </AppModal>
    );
}

function validationIssues(value?: string): string[] {
    if (!value?.trim()) return [];
    try {
        const parsed = JSON.parse(value) as { issues?: unknown; error?: unknown };
        if (Array.isArray(parsed.issues)) {
            return parsed.issues.map(String).filter(Boolean);
        }
        if (typeof parsed.error === "string") return [parsed.error];
    } catch {
        return [value];
    }
    return [];
}
