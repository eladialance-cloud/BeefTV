import { useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent, type ReactNode } from "react";
import { Button, Input, Popconfirm, Popover, Tooltip } from "antd";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Check, Download, ImagePlus, LoaderCircle, Maximize2, Play, Plus, Settings2, Trash2, Upload } from "lucide-react";

import { CachedResourceImage } from "@/components/cached-resource-image";
import { ImageSettingsPanel } from "@/components/image-settings-panel";
import { ModelPicker } from "@/components/model-picker";
import type { CanvasTheme } from "@/lib/canvas-theme";
import {
    buildManjuAssetRowOutputMediaMap,
    buildManjuStoryboardAssetReferenceMap,
    MANJU_ASSET_KIND_ORDER,
    manjuAssetInputHandleId,
    manjuAssetKindLabel,
    manjuAssetOutputHandleId,
    manjuAssetRows,
    MANJU_ASSET_TABLE_ROW_HEIGHT,
    preferredManjuAssetKind,
    type ManjuStoryboardAssetReference,
} from "@/lib/canvas/canvas-manju-asset-table";
import { generationSettingsLabel, generationSettingsScopeLabel, patchGenerationSettings, resolveCanvasGenerationSettings, type CanvasGenerationSettingsScope } from "@/lib/canvas/canvas-manju-generation-settings";
import { useEffectiveConfig } from "@/stores/use-config-store";
import type { CanvasBatchRow, CanvasGenerationBatch, CanvasGenerationSettings, CanvasManjuAssetKind, CanvasNodeData } from "@/types/canvas";

type Props = {
    node: CanvasNodeData;
    nodes: CanvasNodeData[];
    batch?: CanvasGenerationBatch;
    theme: CanvasTheme;
    readOnly?: boolean;
    onAddRow: () => void;
    onUpdateRow: (rowId: string, patch: Partial<CanvasBatchRow>) => void;
    onGenerate: (rowIds?: string[]) => void;
    onUploadAsset: (rowId: string, file: File) => void;
    onAdoptInput: (rowId: string) => void;
    onDeleteAsset: (rowId: string) => void;
    onPreviewAsset: (assetNode: CanvasNodeData) => void;
    onReloadResource?: (assetNode: CanvasNodeData) => void;
    onConnectInputDrop: (event: ReactPointerEvent, handleId: string) => void;
    onConnectOutputStart: (event: ReactPointerEvent, handleId: string) => void;
    onFocusAsset?: (code: string, kind: CanvasManjuAssetKind, rowId?: string) => void;
    onScrollTopChange?: (scrollTop: number) => void;
    onActiveKindChange?: (kind: CanvasManjuAssetKind) => void;
    onPatchGenerationSettings?: (scope: CanvasGenerationSettingsScope, patch: Partial<CanvasGenerationSettings>) => void;
};

const MANJU_ASSET_COLUMNS = "120px 88px 88px minmax(260px, 1fr) 100px 132px 42px";
const MANJU_STORYBOARD_COLUMNS = "120px 208px 88px minmax(260px, 1fr) 100px 132px 42px";

export function CanvasManjuAssetTableNodeContent({
    node,
    nodes,
    batch,
    theme,
    readOnly = false,
    onAddRow,
    onUpdateRow,
    onGenerate,
    onUploadAsset,
    onAdoptInput,
    onDeleteAsset,
    onPreviewAsset,
    onReloadResource,
    onConnectInputDrop,
    onConnectOutputStart,
    onFocusAsset,
    onScrollTopChange,
    onActiveKindChange,
    onPatchGenerationSettings,
}: Props) {
    const effectiveConfig = useEffectiveConfig();
    const table = node.metadata?.batchTable;
    const rows = table?.rows || [];
    const storyboardTable = Boolean(table?.manjuStoryboardTable);
    const [activeKind, setActiveKind] = useState<CanvasManjuAssetKind>(() =>
        preferredManjuAssetKind(table, node.metadata?.manjuAssetActiveKind || "character"),
    );
    const [uploadRowId, setUploadRowId] = useState("");
    const [settingsScope, setSettingsScope] = useState<CanvasGenerationSettingsScope | null>(null);
    const [settingsOpen, setSettingsOpen] = useState(false);
    const fileInputRef = useRef<HTMLInputElement>(null);
    const tableScrollRef = useRef<HTMLDivElement>(null);
    const scrollUpdateTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
    const pendingScrollTopRef = useRef(0);
    const previousRowCountRef = useRef(rows.length);
    const nodeById = useMemo(() => new Map(nodes.map((item) => [item.id, item])), [nodes]);
    const rowMediaById = useMemo(() => buildManjuAssetRowOutputMediaMap(rows, nodes), [nodes, rows]);
    const storyboardReferencesByRowId = useMemo(
        () => (storyboardTable ? buildManjuStoryboardAssetReferenceMap(node, rows, nodes, nodeById) : new Map<string, ManjuStoryboardAssetReference[]>()),
        [node, nodeById, nodes, rows, storyboardTable],
    );
    const batchItemByRowId = useMemo(() => new Map((batch?.items || []).map((item) => [item.rowId, item])), [batch?.items]);
    const activeRows = storyboardTable ? rows : manjuAssetRows(table, activeKind);
    const rowIndexById = useMemo(() => new Map(rows.map((row, index) => [row.id, index])), [rows]);
    const rowVirtualizer = useVirtualizer({
        count: activeRows.length,
        getScrollElement: () => tableScrollRef.current,
        estimateSize: () => MANJU_ASSET_TABLE_ROW_HEIGHT,
        getItemKey: (index) => activeRows[index]?.id || index,
        overscan: 6,
    });
    const countByKind = Object.fromEntries(MANJU_ASSET_KIND_ORDER.map((kind) => [kind, manjuAssetRows(table, kind).length])) as Record<CanvasManjuAssetKind, number>;
    const readyCount = activeRows.filter((row) => {
        const media = rowMediaById.get(row.id);
        return Boolean(media?.metadata?.content || media?.metadata?.storageKey);
    }).length;
    const settingsRow = settingsScope?.rowId
        ? rows.find((row) => row.id === settingsScope.rowId)
        : settingsScope?.kind
          ? ({ id: "category-preview", enabled: true, inputNodeIds: [], prompt: "", manjuAsset: { taskId: "", code: "", kind: settingsScope.kind, name: "" } } satisfies CanvasBatchRow)
          : undefined;
    const resolvedSettings = resolveCanvasGenerationSettings(table, settingsRow, effectiveConfig);
    const settingsConfig = {
        ...effectiveConfig,
        model: resolvedSettings.model,
        size: resolvedSettings.size,
        quality: resolvedSettings.quality,
        count: String(resolvedSettings.count),
    };
    useEffect(() => () => {
        if (scrollUpdateTimerRef.current) clearTimeout(scrollUpdateTimerRef.current);
    }, []);
    useEffect(() => {
        const previousRowCount = previousRowCountRef.current;
        previousRowCountRef.current = rows.length;
        if (storyboardTable || rows.length <= previousRowCount || countByKind[activeKind] > 0) return;
        const nextKind = preferredManjuAssetKind(table, activeKind);
        if (nextKind !== activeKind) {
            setActiveKind(nextKind);
            onActiveKindChange?.(nextKind);
        }
    }, [activeKind, countByKind, onActiveKindChange, rows.length, storyboardTable, table]);

    const handleTableScroll = (scrollTop: number) => {
        pendingScrollTopRef.current = scrollTop;
        if (scrollUpdateTimerRef.current) return;
        scrollUpdateTimerRef.current = setTimeout(() => {
            scrollUpdateTimerRef.current = undefined;
            onScrollTopChange?.(pendingScrollTopRef.current);
        }, 80);
    };

    const pickUpload = (rowId: string) => {
        if (readOnly) return;
        setUploadRowId(rowId);
        fileInputRef.current?.click();
    };

    return (
        <div data-manju-asset-table data-manju-asset-name-version="1" data-manju-asset-auto-read-version="11" data-manju-asset-prompt-repair-version="2" data-manju-asset-p2-only-version="1" data-manju-internal-connections-version="2" data-manju-style-reference-version="2" data-manju-reference-node-cleanup-version="1" data-manju-table-virtualization-version="1" data-manju-content-render-isolation-version="1" data-manju-reference-index-version="1" data-manju-scroll-coalesce-version="1" data-manju-model-scope-save-version="1" data-manju-model-save-button-version="1" data-manju-batch-model-confirm-version="1" data-manju-asset-result-recovery-version="1" data-manju-recovered-asset-link-version="1" data-manju-asset-task-identity-version="2" data-manju-manual-rows-version="1" data-manju-upload-timeout-version="1" data-canvas-result-serializer-version="1" data-manju-table-node-id={node.id} data-canvas-no-zoom data-canvas-wheel-scroll className="flex h-full w-full flex-col overflow-hidden text-xs" style={{ color: theme.node.text }}>
            {!readOnly ? (
                <input
                    ref={fileInputRef}
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    className="hidden"
                    onChange={(event) => {
                        const file = event.currentTarget.files?.[0];
                        const rowId = uploadRowId;
                        event.currentTarget.value = "";
                        setUploadRowId("");
                        if (file && rowId) onUploadAsset(rowId, file);
                    }}
                />
            ) : null}

            <header
                data-canvas-batch-drag
                className="flex h-[52px] shrink-0 cursor-grab items-center gap-3 border-b px-3 active:cursor-grabbing"
                style={{ borderColor: theme.node.stroke, background: `color-mix(in srgb, ${theme.node.text} 4%, transparent)` }}
                onPointerDown={(event) => event.stopPropagation()}
            >
                <div className="flex min-w-0 items-center gap-1">
                    {storyboardTable ? (
                        <span className="rounded-lg px-3 py-1.5 text-[11px] font-medium" style={{ background: theme.accent.primary, color: theme.accent.onPrimary }}>
                            故事板 {rows.length}
                        </span>
                    ) : (
                        MANJU_ASSET_KIND_ORDER.map((kind) => {
                            const selected = activeKind === kind;
                            return (
                                <button
                                    key={kind}
                                    type="button"
                                    className="rounded-lg px-3 py-1.5 text-[11px] font-medium"
                                    style={{
                                        background: selected ? theme.accent.primary : theme.node.panel,
                                        color: selected ? theme.accent.onPrimary : theme.node.muted,
                                    }}
                                    onClick={() => {
                                        setActiveKind(kind);
                                        onActiveKindChange?.(kind);
                                        onScrollTopChange?.(0);
                                        if (scrollUpdateTimerRef.current) clearTimeout(scrollUpdateTimerRef.current);
                                        scrollUpdateTimerRef.current = undefined;
                                        pendingScrollTopRef.current = 0;
                                        if (tableScrollRef.current) tableScrollRef.current.scrollTop = 0;
                                    }}
                                >
                                    {manjuAssetKindLabel(kind)} {countByKind[kind]}
                                </button>
                            );
                        })
                    )}
                </div>
                <span className="ml-auto tabular-nums" style={{ color: theme.node.muted }}>
                    已完成 {readyCount}/{activeRows.length}
                </span>
                <Popover
                    trigger="click"
                    placement="bottomRight"
                    open={settingsOpen}
                    onOpenChange={(open) => {
                        setSettingsOpen(open);
                        if (open) setSettingsScope((current) => current || (storyboardTable ? {} : { kind: activeKind }));
                    }}
                    content={
                        <div className="w-[330px]" onPointerDown={(event) => event.stopPropagation()} onWheel={(event) => event.stopPropagation()}>
                            <div className="mb-3 flex flex-wrap items-center gap-1">
                                {[{ label: "表格统一", scope: {} as CanvasGenerationSettingsScope }, ...(storyboardTable ? [] : MANJU_ASSET_KIND_ORDER.map((kind) => ({ label: manjuAssetKindLabel(kind), scope: { kind } })))].map((item) => {
                                    const selected = settingsScope && !settingsScope.rowId && settingsScope.kind === item.scope.kind;
                                    return (
                                        <button
                                            key={item.label}
                                            type="button"
                                            className="rounded-full px-2.5 py-1 text-[11px]"
                                            style={{
                                                background: selected ? theme.accent.primary : "transparent",
                                                color: selected ? theme.accent.onPrimary : theme.node.text,
                                            }}
                                            onClick={() => setSettingsScope(item.scope)}
                                        >
                                            {item.label}
                                        </button>
                                    );
                                })}
                            </div>
                            <div className="mb-2 text-xs font-semibold">{generationSettingsScopeLabel(settingsScope?.kind, settingsRow)}</div>
                            <div className="mb-3 text-[10px]" style={{ color: theme.node.muted }}>
                                当前生效：{generationSettingsLabel(resolvedSettings)}
                            </div>
                            <div className="mb-3">
                                <ModelPicker config={effectiveConfig} value={resolvedSettings.model} capability="image" fullWidth onChange={(model) => onPatchGenerationSettings?.(settingsScope || {}, { model })} />
                            </div>
                            <ImageSettingsPanel
                                config={settingsConfig}
                                theme={theme}
                                showTitle={false}
                                showTransparent={false}
                                className="space-y-3"
                                onConfigChange={(key, value) => {
                                    if (key === "count") {
                                        onPatchGenerationSettings?.(settingsScope || {}, { count: Math.max(1, Number(value) || 1) });
                                        return;
                                    }
                                    onPatchGenerationSettings?.(settingsScope || {}, { [key]: value });
                                }}
                            />
                            <div className="mt-3 flex items-center justify-between gap-2">
                                <Button
                                    size="small"
                                    onClick={() => {
                                        onPatchGenerationSettings?.(settingsScope || {}, { model: "", size: "", quality: "", count: undefined });
                                    }}
                                >
                                    恢复继承
                                </Button>
                                <Button size="small" type="primary" icon={<Check className="size-3.5" />} disabled={readOnly} onClick={() => setSettingsOpen(false)}>
                                    保存设置
                                </Button>
                            </div>
                        </div>
                    }
                >
                    <Button
                        size="small"
                        type="text"
                        icon={<Settings2 className="size-3.5" />}
                        disabled={readOnly}
                        onClick={() => setSettingsScope(storyboardTable ? {} : { kind: activeKind })}
                    >
                        生成设置
                    </Button>
                </Popover>
                {!readOnly ? (
                    <Button
                        size="small"
                        type="primary"
                        icon={<Play className="size-3.5" />}
                        disabled={!activeRows.some((row) => row.enabled && !row.outputNodeId && row.prompt.trim())}
                        onClick={() => onGenerate(activeRows.filter((row) => row.enabled && !row.outputNodeId && row.prompt.trim()).map((row) => row.id))}
                    >
                        生成本分类未完成项
                    </Button>
                ) : null}
            </header>

            <div ref={tableScrollRef} data-manju-table-scroll className="min-h-0 flex-1 overflow-auto" onScroll={(event) => handleTableScroll(event.currentTarget.scrollTop)}>
                <div
                    className="grid h-9 items-center border-b px-3 text-[10px] font-medium"
                    style={{
                        gridTemplateColumns: storyboardTable ? MANJU_STORYBOARD_COLUMNS : MANJU_ASSET_COLUMNS,
                        borderColor: theme.node.stroke,
                        background: theme.node.panel,
                        color: theme.node.muted,
                    }}
                >
                    <span>{storyboardTable ? "段号 / 标题" : "编号 / 名称"}</span>
                    <span className="text-center">{storyboardTable ? "参考图" : "引入图"}</span>
                    <span className="text-center">{storyboardTable ? "故事板图" : "当前资产"}</span>
                    <span>{storyboardTable ? "故事板提示词" : "生成提示词"}</span>
                    <span className="text-center">状态</span>
                    <span className="text-center">操作</span>
                    <span className="text-center">引出</span>
                </div>

                {activeRows.length ? (
                    <div style={{ height: rowVirtualizer.getTotalSize(), position: "relative" }}>
                    {rowVirtualizer.getVirtualItems().map((virtualRow) => {
                        const row = activeRows[virtualRow.index];
                        if (!row) return null;
                        const rowIndex = rowIndexById.get(row.id) ?? 0;
                        const rowCode = storyboardTable ? row.manjuStoryboard?.code : row.manjuAsset?.code;
                        const rowName = storyboardTable ? row.manjuStoryboard?.title : row.manjuAsset?.name;
                        const manualAsset = Boolean(row.manjuAsset?.taskId.startsWith("manual:asset:"));
                        const manualStoryboard = Boolean(row.manjuStoryboard?.taskId.startsWith("manual:storyboard:"));
                        const input = row.inputNodeIds.map((id) => nodeById.get(id)).find(Boolean);
                        const output = rowMediaById.get(row.id);
                        const recoverable = Boolean(
                            output?.metadata?.taskId
                            && (
                                output.metadata.resourceReloadAvailable
                                || (output.metadata.taskStatus === "succeeded" && !output.metadata.content && !output.metadata.storageKey)
                            ),
                        );
                        const internalReferences = storyboardTable ? storyboardReferencesByRowId.get(row.id) || [] : [];
                        const item = batchItemByRowId.get(row.id);
                        const rowSettings = resolveCanvasGenerationSettings(table, row, effectiveConfig);
                        const loading = Boolean(item && ["waiting", "submitting", "queued", "running"].includes(item.status));
                        const failed = item?.status === "failed" || output?.metadata?.status === "error";
                        const pendingReference = internalReferences.some((reference) => reference.pending);
                        const status = loading ? "生成中" : recoverable ? "结果待取回" : failed ? "失败" : output ? "已完成" : pendingReference ? "待补资产" : input ? "待生成/可采用" : "待补";
                        return (
                            <div
                                key={row.id}
                                ref={rowVirtualizer.measureElement}
                                data-index={virtualRow.index}
                                style={{ position: "absolute", top: 0, left: 0, width: "100%", transform: `translateY(${virtualRow.start}px)` }}
                            >
                            <div
                                data-manju-asset-row-id={row.id}
                                className="relative grid min-h-[132px] items-center border-b px-3 py-3"
                                style={{
                                    gridTemplateColumns: storyboardTable ? MANJU_STORYBOARD_COLUMNS : MANJU_ASSET_COLUMNS,
                                    borderColor: theme.node.stroke,
                                    background: row.id === activeRows[0]?.id ? `color-mix(in srgb, ${theme.accent.primary} 3%, transparent)` : "transparent",
                                }}
                            >
                                <div className="min-w-0 pr-2">
                                    {manualAsset && row.manjuAsset ? (
                                        <div className="space-y-1">
                                            <Input
                                                size="small"
                                                value={row.manjuAsset.code}
                                                readOnly={readOnly}
                                                aria-label="资产编号"
                                                onChange={(event) => onUpdateRow(row.id, { manjuAsset: { ...row.manjuAsset!, code: event.target.value } })}
                                                onPointerDown={(event) => event.stopPropagation()}
                                            />
                                            <Input
                                                size="small"
                                                value={row.manjuAsset.name}
                                                readOnly={readOnly}
                                                aria-label="资产名称"
                                                onChange={(event) => onUpdateRow(row.id, { manjuAsset: { ...row.manjuAsset!, name: event.target.value } })}
                                                onPointerDown={(event) => event.stopPropagation()}
                                            />
                                        </div>
                                    ) : manualStoryboard && row.manjuStoryboard ? (
                                        <div className="space-y-1">
                                            <Input
                                                size="small"
                                                value={row.manjuStoryboard.code}
                                                readOnly={readOnly}
                                                aria-label="故事板段号"
                                                onChange={(event) => onUpdateRow(row.id, { manjuStoryboard: { ...row.manjuStoryboard!, code: event.target.value } })}
                                                onPointerDown={(event) => event.stopPropagation()}
                                            />
                                            <Input
                                                size="small"
                                                value={row.manjuStoryboard.title}
                                                readOnly={readOnly}
                                                aria-label="故事板标题"
                                                onChange={(event) => onUpdateRow(row.id, { manjuStoryboard: { ...row.manjuStoryboard!, title: event.target.value } })}
                                                onPointerDown={(event) => event.stopPropagation()}
                                            />
                                        </div>
                                    ) : (
                                        <>
                                            <strong className="block truncate text-[11px]">{rowCode || `${storyboardTable ? "故事板" : "资产"} ${rowIndex + 1}`}</strong>
                                            <span className="mt-1 block truncate text-[10px]" style={{ color: theme.node.muted }}>
                                                {rowName || row.prompt.slice(0, 18)}
                                            </span>
                                        </>
                                    )}
                                </div>
                                <div className="flex justify-center">
                                    {storyboardTable ? (
                                        <StoryboardReferenceStrip
                                            references={internalReferences}
                                            manualInput={input}
                                            theme={theme}
                                            readOnly={readOnly}
                                            onUpload={() => pickUpload(row.id)}
                                            onFocusAsset={(reference) => onFocusAsset?.(reference.code, reference.kind, reference.rowId)}
                                        />
                                    ) : (
                                        <AssetThumb
                                            media={input}
                                            label="引入"
                                            theme={theme}
                                            empty={<Upload className="size-4" />}
                                            onPointerDown={(event) => event.stopPropagation()}
                                            onPointerUp={(event) => onConnectInputDrop(event, manjuAssetInputHandleId(row.id))}
                                            onClick={() => pickUpload(row.id)}
                                        />
                                    )}
                                </div>
                                <div className="flex justify-center">
                                    <AssetThumb
                                        media={output}
                                        label={storyboardTable ? "故事板" : "资产"}
                                        theme={theme}
                                        empty={<ImagePlus className="size-4" />}
                                        onClick={output ? () => onPreviewAsset(output) : undefined}
                                    />
                                </div>
                                <Input.TextArea
                                    value={row.prompt}
                                    readOnly={readOnly}
                                    autoSize={{ minRows: 3, maxRows: 5 }}
                                    className="thin-scrollbar"
                                    placeholder={storyboardTable ? "填写当前段故事板生成提示词" : "填写角色、场景或道具生成提示词"}
                                    onChange={(event) => onUpdateRow(row.id, { prompt: event.target.value })}
                                    onPointerDown={(event) => event.stopPropagation()}
                                    onWheel={(event) => event.stopPropagation()}
                                />
                                <span className="min-w-0 text-center text-[10px]" style={{ color: failed ? "var(--status-error)" : loading ? "var(--status-loading)" : theme.node.muted }}>
                                    {loading ? <LoaderCircle className="mx-auto mb-1 size-4 animate-spin" /> : null}
                                    <span className="block">{status}</span>
                                    <span className="mt-1 block truncate" title={generationSettingsLabel(rowSettings)}>
                                        {rowSettings.size} · {rowSettings.quality?.toUpperCase() || "AUTO"} · {rowSettings.count}张
                                    </span>
                                </span>
                                <div className="flex flex-wrap items-center justify-center gap-1">
                                    {recoverable ? (
                                        <Tooltip title="取回后台已生成的原结果，不会重新生成">
                                            <Button
                                                size="small"
                                                type="primary"
                                                disabled={readOnly}
                                                icon={<Download className="size-3.5" />}
                                                onClick={() => output && onReloadResource?.(output)}
                                            />
                                        </Tooltip>
                                    ) : (
                                        <Tooltip title={row.outputNodeId ? "重新生成当前资产" : "生成当前资产"}>
                                            <Button
                                                size="small"
                                                type={row.outputNodeId ? "text" : "primary"}
                                                disabled={readOnly || loading || !row.prompt.trim()}
                                                icon={loading ? <LoaderCircle className="size-3.5 animate-spin" /> : <Play className="size-3.5" />}
                                                onClick={() => onGenerate([row.id])}
                                            />
                                        </Tooltip>
                                    )}
                                    <Tooltip title="上传并设为当前资产">
                                        <Button size="small" type="text" disabled={readOnly} icon={<Upload className="size-3.5" />} onClick={() => pickUpload(row.id)} />
                                    </Tooltip>
                                    <Tooltip title="把引入图直接设为当前资产">
                                        <Button size="small" type="text" disabled={readOnly || !input || row.outputNodeId === input?.id} icon={<Check className="size-3.5" />} onClick={() => onAdoptInput(row.id)} />
                                    </Tooltip>
                                    <Tooltip title="放大查看当前图片">
                                        <Button size="small" type="text" disabled={!output} icon={<Maximize2 className="size-3.5" />} onClick={() => output && onPreviewAsset(output)} />
                                    </Tooltip>
                                    <Popconfirm title={storyboardTable ? "删除当前故事板图？" : "删除当前资产图？"} okText="删除" cancelText="取消" disabled={readOnly || !output} onConfirm={() => onDeleteAsset(row.id)}>
                                        <Tooltip title="删除当前图片">
                                            <Button size="small" type="text" danger disabled={readOnly || !output} icon={<Trash2 className="size-3.5" />} />
                                        </Tooltip>
                                    </Popconfirm>
                                    <Tooltip title="设置当前行">
                                        <Button
                                            size="small"
                                            type="text"
                                            disabled={readOnly}
                                            icon={<Settings2 className="size-3.5" />}
                                            onClick={() => {
                                                setSettingsScope({ rowId: row.id });
                                                setSettingsOpen(true);
                                            }}
                                        />
                                    </Tooltip>
                                </div>
                                <div className="flex justify-center">
                                    <button
                                        type="button"
                                        aria-label={`${rowCode || row.id} 引出连线`}
                                        className="grid size-6 place-items-center rounded-full border text-[9px] font-semibold"
                                        style={{ borderColor: theme.accent.primary, background: theme.node.panel, color: theme.accent.primary, cursor: "crosshair" }}
                                        onPointerDown={(event) => {
                                            event.stopPropagation();
                                            onConnectOutputStart(event, manjuAssetOutputHandleId(row.id));
                                        }}
                                    >
                                        ○
                                    </button>
                                </div>
                                <button
                                    type="button"
                                    aria-label={`${rowCode || row.id} 引入连线`}
                                    className="absolute left-0 top-1/2 z-20 size-[18px] -translate-x-1/2 -translate-y-1/2 rounded-full border"
                                    style={{ borderColor: theme.accent.primary, background: theme.node.panel }}
                                    title="接入图片节点"
                                    onPointerUp={(event) => onConnectInputDrop(event, manjuAssetInputHandleId(row.id))}
                                />
                            </div>
                            </div>
                        );
                    })}
                    </div>
                ) : (
                    <div className="grid min-h-48 place-items-center px-6 text-center" style={{ color: theme.node.muted }}>
                        {storyboardTable ? "当前没有故事板任务。" : "当前分类没有数字资产。"}
                    </div>
                )}
                <div className="p-3">
                    <Button block type="dashed" icon={<Plus className="size-3.5" />} disabled={readOnly} onClick={onAddRow}>
                        {storyboardTable ? "添加故事板" : `添加${manjuAssetKindLabel(activeKind)}资产`}
                    </Button>
                </div>
            </div>
        </div>
    );
}

function StoryboardReferenceStrip({
    references,
    manualInput,
    theme,
    readOnly,
    onUpload,
    onFocusAsset,
}: {
    references: ManjuStoryboardAssetReference[];
    manualInput?: CanvasNodeData;
    theme: CanvasTheme;
    readOnly: boolean;
    onUpload: () => void;
    onFocusAsset: (reference: ManjuStoryboardAssetReference) => void;
}) {
    const referenceNodeIds = new Set(references.map((reference) => reference.node?.id).filter(Boolean));
    const showManualInput = manualInput && !referenceNodeIds.has(manualInput.id);
    return (
        <div data-canvas-no-zoom className="thin-scrollbar grid max-h-[96px] w-full grid-cols-2 gap-1 overflow-y-auto pr-1" onPointerDown={(event) => event.stopPropagation()} onWheel={(event) => event.stopPropagation()}>
            {references.map((reference) => {
                const source = reference.node?.metadata?.previewContent || reference.node?.metadata?.content;
                return (
                    <button
                        key={`${reference.code}:${reference.rowId || ""}`}
                        type="button"
                        className="flex min-w-0 items-center gap-1 rounded-md border p-1 text-left transition hover:brightness-105 focus-visible:outline focus-visible:outline-2"
                        style={{
                            borderColor: reference.pending ? "var(--status-warning)" : theme.node.stroke,
                            background: reference.pending ? "color-mix(in srgb, var(--status-warning) 8%, transparent)" : theme.node.panel,
                            color: reference.pending ? "var(--status-warning)" : theme.node.text,
                            outlineColor: theme.accent.primary,
                        }}
                        title={`${manjuAssetKindLabel(reference.kind)} · ${reference.code} · ${reference.name}${reference.pending ? "（待补）" : ""}`}
                        aria-label={`${reference.pending ? "定位待补资产" : "定位资产"} ${reference.code}`}
                        onClick={() => onFocusAsset(reference)}
                    >
                        <span className="relative size-8 shrink-0 overflow-hidden rounded border" style={{ borderColor: theme.node.stroke }}>
                            {source ? (
                                <CachedResourceImage eager src={source} storageKey={reference.node?.metadata?.storageKey} alt={reference.name} className="size-8 object-cover" />
                            ) : (
                                <span className="grid size-full place-items-center">
                                    <ImagePlus className="size-3.5" />
                                </span>
                            )}
                        </span>
                        <span className="min-w-0">
                            <span className="block truncate text-[9px] font-semibold">{reference.code}</span>
                            <span className="block truncate text-[8px] opacity-70">{reference.pending ? "待补" : manjuAssetKindLabel(reference.kind)}</span>
                        </span>
                    </button>
                );
            })}
            {showManualInput ? <StoryboardManualInput media={manualInput} theme={theme} /> : null}
            {!readOnly ? (
                <button
                    type="button"
                    className="flex min-h-10 items-center justify-center gap-1 rounded-md border border-dashed px-1.5 text-[9px] focus-visible:outline focus-visible:outline-2"
                    style={{ borderColor: theme.node.stroke, color: theme.node.muted, outlineColor: theme.accent.primary }}
                    title="上传额外参考图"
                    aria-label="上传额外故事板参考图"
                    onClick={onUpload}
                >
                    <Upload className="size-3" />
                    额外参考
                </button>
            ) : null}
        </div>
    );
}

function StoryboardManualInput({ media, theme }: { media: CanvasNodeData; theme: CanvasTheme }) {
    const source = media.metadata?.previewContent || media.metadata?.content;
    return (
        <span className="flex min-w-0 items-center gap-1 rounded-md border p-1" style={{ borderColor: theme.node.stroke, background: theme.node.panel }} title={`手动参考 · ${media.title}`}>
            <span className="relative size-8 shrink-0 overflow-hidden rounded border" style={{ borderColor: theme.node.stroke }}>
                {source ? <CachedResourceImage eager src={source} storageKey={media.metadata?.storageKey} alt={media.title} className="size-8 object-cover" /> : <span className="grid size-full place-items-center"><ImagePlus className="size-3.5" /></span>}
            </span>
            <span className="min-w-0">
                <span className="block truncate text-[9px] font-semibold">手动参考</span>
                <span className="block truncate text-[8px] opacity-70">{media.title}</span>
            </span>
        </span>
    );
}

function AssetThumb({
    media,
    label,
    theme,
    empty,
    onClick,
    onPointerDown,
    onPointerUp,
}: {
    media?: CanvasNodeData;
    label: string;
    theme: CanvasTheme;
    empty: ReactNode;
    onClick?: () => void;
    onPointerDown?: (event: ReactPointerEvent) => void;
    onPointerUp?: (event: ReactPointerEvent) => void;
}) {
    const source = media?.metadata?.previewContent || media?.metadata?.content;
    return (
        <button
            type="button"
            className="relative size-16 overflow-hidden rounded-lg border"
            style={{ borderColor: source ? theme.node.stroke : "transparent", background: theme.node.panel, color: theme.node.muted }}
            onClick={(event) => {
                event.stopPropagation();
                onClick?.();
            }}
            onPointerDown={onPointerDown}
            onPointerUp={onPointerUp}
        >
            {source ? <CachedResourceImage eager src={source} storageKey={media?.metadata?.storageKey} alt={label} className="size-16 object-cover" /> : <span className="grid size-full place-items-center">{empty}</span>}
            <span className="absolute bottom-1 left-1 rounded bg-black/60 px-1 py-0.5 text-[8px] text-white">{label}</span>
        </button>
    );
}
