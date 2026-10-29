import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Button, Checkbox, Input, Select, Tag } from "antd";
import { BookOpenText, Check, FileUp, ListChecks, Play, RefreshCw } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { buildManjuScriptRunInput } from "@/lib/canvas/manju-script-routing";
import {
    createManjuScriptRun,
    createManjuSourceDocument,
    getManjuSourceDocument,
    getManjuSourceLibraryState,
    listManjuSourceDocuments,
    type ManjuScriptInputKind,
    type ManjuScriptMode,
    type ManjuScriptRun,
    type ManjuScriptRunDetail,
} from "@/services/api/manju";

type Props = {
    projectId: string;
    canEdit: boolean;
    inputKind: ManjuScriptInputKind;
    completedRuns: ManjuScriptRun[];
    runsLoading: boolean;
    hasActiveRun: boolean;
    onCreated: (item: ManjuScriptRunDetail) => void;
};

const MAX_SOURCE_BYTES = 16 * 1024 * 1024;

export function SourceLibraryStepsPanel({ projectId, canEdit, inputKind, completedRuns, runsLoading, hasActiveRun, onCreated }: Props) {
    const { message } = App.useApp();
    const queryClient = useQueryClient();
    const fileInputRef = useRef<HTMLInputElement>(null);
    const [selectedDocumentId, setSelectedDocumentId] = useState("");
    const [chapterIds, setChapterIds] = useState<string[]>([]);
    const [confirmedChapterIds, setConfirmedChapterIds] = useState<string[]>([]);
    const [unitTitle, setUnitTitle] = useState("");
    const [mode, setMode] = useState<ManjuScriptMode>("STEPWISE");
    const [reuseGlobalStages, setReuseGlobalStages] = useState(true);
    const [title, setTitle] = useState("");
    const [authorizationScope, setAuthorizationScope] = useState("仅用于本项目改编，不得转授权");
    const [fileName, setFileName] = useState("");
    const [content, setContent] = useState("");
    const finishedScript = inputKind === "finished_script";
    const sourceName = finishedScript ? "外部剧本" : "原著";

    const documentsQuery = useQuery({
        queryKey: ["manju", "source-documents", projectId],
        queryFn: () => listManjuSourceDocuments(projectId),
    });
    const sourceStateQuery = useQuery({
        queryKey: ["manju", "source-library-state", projectId],
        queryFn: () => getManjuSourceLibraryState(projectId),
    });
    const documents = documentsQuery.data?.items || [];
    const baseRun = completedRuns[0];
    const sourceLibraryChanged = Boolean(
        baseRun &&
        sourceStateQuery.data?.item.contentHash &&
        (!baseRun.sourceSetHash || baseRun.sourceSetHash !== sourceStateQuery.data.item.contentHash),
    );

    useEffect(() => {
        if (!selectedDocumentId && documents[0]) setSelectedDocumentId(documents[0].id);
    }, [documents, selectedDocumentId]);

    const documentQuery = useQuery({
        queryKey: ["manju", "source-document", projectId, selectedDocumentId],
        queryFn: () => getManjuSourceDocument(projectId, selectedDocumentId),
        enabled: Boolean(selectedDocumentId),
    });
    const selectedDocument = documentQuery.data?.item;

    useEffect(() => {
        setChapterIds([]);
        setConfirmedChapterIds([]);
        setUnitTitle("");
    }, [selectedDocumentId]);

    useEffect(() => {
        setReuseGlobalStages(completedRuns.length > 0 && !sourceLibraryChanged);
    }, [completedRuns.length, sourceLibraryChanged]);

    const createDocumentMutation = useMutation({
        mutationFn: () => createManjuSourceDocument(projectId, {
            title: title.trim(),
            authorizationScope: authorizationScope.trim(),
            fileName: fileName.trim(),
            content,
        }),
        onSuccess: async ({ item }) => {
            message.success(`已登记 ${item.document.chapterCount} 个章节`);
            setTitle("");
            setFileName("");
            setContent("");
            await queryClient.invalidateQueries({ queryKey: ["manju", "source-documents", projectId] });
            await queryClient.invalidateQueries({ queryKey: ["manju", "source-library-state", projectId] });
            setSelectedDocumentId(item.document.id);
            queryClient.setQueryData(["manju", "source-document", projectId, item.document.id], { item });
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "来源登记失败"),
    });

    const createUnitMutation = useMutation({
        mutationFn: () => createManjuScriptRun(projectId, buildManjuScriptRunInput({
            mode,
            source: "adaptation",
            inputKind,
            trendResolution: "",
            title: unitTitle.trim() || selectedDocument?.chapters.find((chapter) => chapter.id === confirmedChapterIds[0])?.title,
            sourceDocumentId: selectedDocumentId,
            chapterIds: confirmedChapterIds,
            baseRunId: reuseGlobalStages ? completedRuns[0]?.id : undefined,
            reuseGlobalStages: reuseGlobalStages && completedRuns.length > 0,
        })),
        onSuccess: ({ item }) => {
            message.success("已创建剧本任务版，并继承当前 P0");
            setChapterIds([]);
            setConfirmedChapterIds([]);
            setUnitTitle("");
            onCreated(item);
        },
        onError: (error) => {
            message.error(error instanceof Error ? error.message : "改编单元创建失败");
            void queryClient.invalidateQueries({ queryKey: ["manju", "script-runs", projectId] });
        },
    });

    const upload = async (file: File | undefined) => {
        if (!file) return;
        if (file.size > MAX_SOURCE_BYTES) {
            message.error(`${sourceName}文件不能超过 16MB`);
            return;
        }
        if (!/\.(txt|md|markdown)$/i.test(file.name) && !/^text\//.test(file.type)) {
            message.error("请上传 TXT 或 Markdown 文本文件");
            return;
        }
        try {
            const bytes = await file.arrayBuffer();
            let text = new TextDecoder("utf-8").decode(bytes);
            if (text.includes("\uFFFD")) {
                try {
                    text = new TextDecoder("gb18030").decode(bytes);
                } catch {
                    // Keep UTF-8 when the browser lacks the legacy decoder.
                }
            }
            if (!text.trim()) {
                message.error("文件没有可读取的文字");
                return;
            }
            setContent(text);
            setFileName(file.name);
            if (!title.trim()) setTitle(file.name.replace(/\.(txt|md|markdown)$/i, ""));
        } catch {
            message.error(`读取${sourceName}文件失败`);
        } finally {
            if (fileInputRef.current) fileInputRef.current.value = "";
        }
    };

    const selectedChapters = selectedDocument?.chapters.filter((chapter) => chapterIds.includes(chapter.id)) || [];
    const confirmedChapters = selectedDocument?.chapters.filter((chapter) => confirmedChapterIds.includes(chapter.id)) || [];
    const selectedCharacterCount = selectedChapters.reduce((total, chapter) => total + chapter.characterCount, 0);
    const confirmedCharacterCount = confirmedChapters.reduce((total, chapter) => total + chapter.characterCount, 0);
    const confirmChapterSelection = () => {
        if (!selectedDocument || chapterIds.length === 0) return;
        setConfirmedChapterIds(selectedDocument.chapters.filter((chapter) => chapterIds.includes(chapter.id)).map((chapter) => chapter.id));
    };

    return (
        <section className="manju-source-steps" aria-label={`${finishedScript ? "外部剧本" : "小说改编"}三步流程`}>
            <header className="manju-source-steps-head">
                <div>
                    <span className="manju-kicker">P1 · {finishedScript ? "外来剧本润色" : "小说改编"}</span>
                    <h3>{sourceName}与本次范围</h3>
                    <p>{finishedScript
                        ? "先登记已成型剧本，再确认本次润色范围，最后创建润色任务版。P0 沿用当前项目锁。"
                        : "先登记原著，再选择并确认章节，最后创建改编单元。P0 沿用当前项目锁，不重复填写。"}</p>
                </div>
                <Button icon={<RefreshCw />} loading={documentsQuery.isFetching} onClick={() => void documentsQuery.refetch()}>
                    刷新来源库
                </Button>
            </header>

            <section className="manju-source-step" data-step="1">
                <header>
                    <span className="manju-source-step-number">1</span>
                    <div>
                        <strong>{finishedScript ? "登记成型外部剧本" : "登记全文或新章节"}</strong>
                        <small>选择 TXT / Markdown 文本，确认名称与授权后登记到项目来源库。</small>
                    </div>
                    {fileName ? <Tag color="blue">{fileName}</Tag> : null}
                </header>
                <div className="manju-source-upload-grid">
                    <label>
                        <span>{sourceName}名称</span>
                        <Input value={title} disabled={!canEdit} maxLength={240} placeholder="例如：雨夜旧宅" onChange={(event) => setTitle(event.target.value)} />
                    </label>
                    <label>
                        <span>授权范围</span>
                        <Input value={authorizationScope} disabled={!canEdit} maxLength={500} onChange={(event) => setAuthorizationScope(event.target.value)} />
                    </label>
                    <label className="is-wide">
                        <span>{finishedScript ? "外部剧本正文" : "原著全文或本次新增章节"}</span>
                        <Input.TextArea value={content} disabled={!canEdit} rows={7} showCount placeholder={`选择${sourceName}文件后会自动读取正文，也可以直接粘贴。系统按“第X章”“Chapter N”等标题划分范围。`} onChange={(event) => setContent(event.target.value)} />
                    </label>
                </div>
                <div className="manju-source-step-actions">
                    <input
                        ref={fileInputRef}
                        type="file"
                        hidden
                        accept=".txt,.md,.markdown,text/plain,text/markdown"
                        onChange={(event) => void upload(event.target.files?.[0])}
                    />
                    <Button disabled={!canEdit} icon={<FileUp />} onClick={() => fileInputRef.current?.click()}>选择{sourceName}文件</Button>
                    <Button
                        type="primary"
                        disabled={!canEdit || !title.trim() || !content.trim()}
                        loading={createDocumentMutation.isPending}
                        onClick={() => createDocumentMutation.mutate()}
                    >
                        登记到来源库
                    </Button>
                </div>
            </section>

            <section className="manju-source-step" data-step="2">
                <header>
                    <span className="manju-source-step-number">2</span>
                    <div>
                        <strong>已有{sourceName}</strong>
                        <small>选择来源文档，勾选本次要处理的范围，然后确认。</small>
                    </div>
                    {selectedDocument ? <span className="manju-source-step-badge">{selectedDocument.document.chapterCount} 章</span> : null}
                </header>
                <Select
                    className="w-full"
                    value={selectedDocumentId || undefined}
                    loading={documentsQuery.isLoading}
                    placeholder={`选择已登记的${sourceName}`}
                    options={documents.map((document) => ({
                        value: document.id,
                        label: `${document.title} · ${document.chapterCount} 章 · ${document.characterCount.toLocaleString()} 字`,
                    }))}
                    onChange={setSelectedDocumentId}
                />
                {!documentsQuery.isLoading && documents.length === 0 ? <p className="manju-source-step-empty">还没有{sourceName}，请先完成第 1 步。</p> : null}
                {selectedDocument ? (
                    <>
                        <div className="manju-source-document-meta">
                            <strong>{selectedDocument.document.title}</strong>
                            <small>{selectedDocument.document.fileName || "粘贴文本"} · {selectedDocument.document.characterCount.toLocaleString()} 字</small>
                            <code>{selectedDocument.document.contentHash.slice(0, 12)}</code>
                        </div>
                        <div className="manju-chapter-checklist">
                            {selectedDocument.chapters.map((chapter) => (
                                <label key={chapter.id} className={chapterIds.includes(chapter.id) ? "is-selected" : ""}>
                                    <Checkbox
                                        checked={chapterIds.includes(chapter.id)}
                                        disabled={!canEdit}
                                        onChange={(event) => setChapterIds((current) => event.target.checked
                                            ? [...current, chapter.id]
                                            : current.filter((id) => id !== chapter.id))}
                                    />
                                    <span>
                                        <strong>{chapter.title}</strong>
                                        <small>第 {chapter.position} 章 · {chapter.characterCount.toLocaleString()} 字</small>
                                    </span>
                                </label>
                            ))}
                        </div>
                        <div className="manju-source-step-actions">
                            <span className="manju-unit-selection-summary">已选择 {chapterIds.length} 章 · {selectedCharacterCount.toLocaleString()} 字</span>
                            <Button
                                type="primary"
                                icon={<Check />}
                                disabled={!canEdit || !selectedDocumentId || chapterIds.length === 0 || runsLoading || hasActiveRun}
                                onClick={confirmChapterSelection}
                            >
                                {runsLoading ? "正在读取当前任务版" : hasActiveRun ? "先完成当前任务版" : "确认本次范围"}
                            </Button>
                        </div>
                    </>
                ) : null}
            </section>

            {confirmedChapterIds.length ? (
                <section className="manju-source-step is-confirmed" data-step="3">
                    <header>
                        <span className="manju-source-step-number">3</span>
                        <div>
                            <strong>确认本次范围</strong>
                            <small>已自动填入第 2 步确认的范围。设置制作方式后创建任务版。</small>
                        </div>
                        <span className="manju-source-step-badge">{confirmedChapterIds.length} 章 · {confirmedCharacterCount.toLocaleString()} 字</span>
                    </header>
                    <div className="manju-confirmed-chapters">
                        {confirmedChapters.map((chapter) => (
                            <span key={chapter.id}><ListChecks />{chapter.title}</span>
                        ))}
                    </div>
                    <div className="manju-unit-create-row">
                        <label>
                            <span>任务版名称</span>
                            <Input value={unitTitle} disabled={!canEdit} maxLength={240} placeholder="留空则使用第一个选中章节名" onChange={(event) => setUnitTitle(event.target.value)} />
                        </label>
                        <label>
                            <span>制作方式</span>
                            <Select
                                value={mode}
                                disabled={!canEdit}
                                options={[
                                    { value: "STEPWISE", label: "按阶段确认" },
                                    { value: "FULL_SCRIPT_PIPELINE", label: "完整流程" },
                                ]}
                                onChange={setMode}
                            />
                        </label>
                        <label className="manju-reuse-global">
                            <Checkbox checked={reuseGlobalStages} disabled={!canEdit || completedRuns.length === 0 || sourceLibraryChanged} onChange={(event) => setReuseGlobalStages(event.target.checked)} />
                            <span>{sourceLibraryChanged ? "来源库已变化，必须重新生成 IP 与故事圣经" : "继承上一任务版已锁定的 IP 与故事圣经"}</span>
                        </label>
                        <Button
                            type="primary"
                            icon={<Play />}
                            disabled={!canEdit || !selectedDocumentId || confirmedChapterIds.length === 0 || runsLoading || hasActiveRun}
                            loading={createUnitMutation.isPending}
                            onClick={() => createUnitMutation.mutate()}
                        >
                            {runsLoading ? "正在读取当前任务版" : hasActiveRun ? "先完成当前任务版" : finishedScript ? "创建润色任务版" : "创建改编单元"}
                        </Button>
                    </div>
                    <div className="manju-source-step-actions">
                        <Button type="text" onClick={() => setConfirmedChapterIds([])}>返回修改章节</Button>
                    </div>
                    {sourceLibraryChanged ? <p className="manju-source-step-empty">来源库在上一任务版完成后有新增或修改，新任务版会重新生成 IP 与故事圣经。</p> : null}
                    {runsLoading ? <p className="manju-source-step-empty">正在读取当前任务版，请稍候。</p> : null}
                    {!runsLoading && hasActiveRun ? <p className="manju-source-step-empty">当前仍有绑定来源的任务版在进行中，请完成后再创建新任务版。</p> : null}
                </section>
            ) : null}
        </section>
    );
}
