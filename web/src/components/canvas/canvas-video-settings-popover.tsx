import { useEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { createPortal } from "react-dom";
import { Coins, Settings2 } from "lucide-react";
import { Button } from "antd";

import { VideoSettingsPanel, videoResolutionLabel, videoSecondsLabel, videoSizeLabel } from "@/components/video-settings-panel";
import { useCanvasVideoCreditEstimate } from "@/hooks/use-canvas-video-credit-estimate";
import { canvasThemes } from "@/lib/canvas-theme";
import { modelCapabilityConfigFor, resolveVideoRatioValue, resolveVideoResolutionValue } from "@/lib/model-capabilities";
import { useActiveTheme } from "@/stores/canvas/use-canvas-theme-store";
import type { AiConfig } from "@/stores/use-config-store";
import type { VideoSettingsConstraints } from "@/lib/video-settings-constraints";

type CanvasVideoSettingsPopoverProps = {
    children?: ReactNode;
    taskConstraints?: VideoSettingsConstraints;
    config: AiConfig;
    onConfigChange: (key: keyof AiConfig, value: string) => void;
    buttonClassName?: string;
    placement?: "topLeft" | "top" | "topRight" | "bottomLeft" | "bottom" | "bottomRight";
    summaryOverride?: string;
};

export function CanvasVideoSettingsPopover({ config, onConfigChange, buttonClassName, placement = "topLeft", summaryOverride, taskConstraints, children }: CanvasVideoSettingsPopoverProps) {
    const theme = canvasThemes[useActiveTheme()];
    const buttonRef = useRef<HTMLSpanElement>(null);
    const panelRef = useRef<HTMLDivElement>(null);
    const [open, setOpen] = useState(false);
    const [buttonRect, setButtonRect] = useState<DOMRect | null>(null);
    const videoProfile = modelCapabilityConfigFor(config, config.model).video;
    const resolutionSupported = Boolean(videoProfile?.resolutions.length);
    const sizeSupported = Boolean(videoProfile?.ratios.length);
    const resolution = videoProfile ? resolveVideoResolutionValue(videoProfile, config.vquality) : "";
    const size = videoProfile ? resolveVideoRatioValue(videoProfile, config.size) : "";
    const creditEstimate = useCanvasVideoCreditEstimate(config);
    const summary = [
        ...(resolutionSupported ? [videoResolutionLabel(resolution)] : []),
        ...(sizeSupported ? [taskConstraints?.ratioLabel || videoSizeLabel(size)] : []),
        taskConstraints?.durationLocked ? "随原视频时长" : videoSecondsLabel(config.videoSeconds, videoProfile),
        creditEstimate.text,
    ].join(" · ");
    const displaySummary = summaryOverride || summary;

    useEffect(() => {
        if (!open) return;
        const syncPosition = () => setButtonRect(buttonRef.current?.getBoundingClientRect() || null);
        const closeOnOutsidePointer = (event: PointerEvent) => {
            const target = event.target;
            if (!(target instanceof Node)) return;
            if (buttonRef.current?.contains(target) || panelRef.current?.contains(target)) return;
            if (target instanceof Element && target.closest(".canvas-h3-option-popup")) return;
            setOpen(false);
        };

        syncPosition();
        window.addEventListener("resize", syncPosition);
        window.addEventListener("scroll", syncPosition, true);
        window.addEventListener("pointerdown", closeOnOutsidePointer, true);
        return () => {
            window.removeEventListener("resize", syncPosition);
            window.removeEventListener("scroll", syncPosition, true);
            window.removeEventListener("pointerdown", closeOnOutsidePointer, true);
        };
    }, [open]);

    const panel = open && buttonRect ? <VideoSettingsPortal buttonRect={buttonRect} panelRef={panelRef} placement={placement} theme={theme} config={config} onConfigChange={onConfigChange} taskConstraints={taskConstraints} creditEstimate={creditEstimate}>{children}</VideoSettingsPortal> : null;

    return (
        <>
            <span ref={buttonRef} className="inline-flex min-w-0">
                <Button size="small" type="text" className={`canvas-generation-settings-trigger ${buttonClassName || "!h-8 !max-w-[230px] !justify-start !rounded-full !px-2.5"}`} style={{ background: theme.node.fill, color: theme.node.text }} icon={<Settings2 className="size-3.5" />} aria-expanded={open} aria-label={`高级设置：${displaySummary}`} title={`高级设置 · ${displaySummary}`} onClick={() => setOpen((current) => !current)}>
                    <span className="truncate">{displaySummary}</span>
                </Button>
            </span>
            {panel}
        </>
    );
}

function VideoSettingsPortal({
    buttonRect,
    panelRef,
    placement,
    theme,
    config,
    onConfigChange,
    taskConstraints,
    creditEstimate,
    children,
}: {
    children?: ReactNode;
    taskConstraints?: VideoSettingsConstraints;
    creditEstimate: ReturnType<typeof useCanvasVideoCreditEstimate>;
    buttonRect: DOMRect;
    panelRef: RefObject<HTMLDivElement | null>;
    placement: CanvasVideoSettingsPopoverProps["placement"];
    theme: (typeof canvasThemes)[keyof typeof canvasThemes];
    config: AiConfig;
    onConfigChange: (key: keyof AiConfig, value: string) => void;
}) {
    const gap = 8;
    const margin = 12;
    const width = Math.min(356, window.innerWidth - margin * 2);
    const alignRight = placement?.endsWith("Right");
    const alignCenter = placement === "top" || placement === "bottom";
    const left = alignCenter ? buttonRect.left + buttonRect.width / 2 - width / 2 : alignRight ? buttonRect.right - width : buttonRect.left;
    const topPlacement = placement?.startsWith("top");
    const estimatedHeight = 370;
    const topSpace = buttonRect.top - gap - margin;
    const bottomSpace = window.innerHeight - buttonRect.bottom - gap - margin;
    const placeAbove = topPlacement ? topSpace >= estimatedHeight || topSpace >= bottomSpace : bottomSpace < estimatedHeight && topSpace > bottomSpace;
    const style = {
        position: "fixed",
        zIndex: "var(--z-dialog-popover)",
        width,
        left: Math.max(margin, Math.min(window.innerWidth - width - margin, left)),
        ...(placeAbove ? { bottom: window.innerHeight - buttonRect.top + gap, maxHeight: Math.max(260, topSpace) } : { top: buttonRect.bottom + gap, maxHeight: Math.max(260, bottomSpace) }),
        background: theme.canvas.background,
        border: `1px solid ${theme.toolbar.border}`,
        borderRadius: 10,
        boxShadow: `0 24px 72px ${theme.spatial.shadow}`,
        padding: 12,
        overflowY: "auto",
        color: theme.node.text,
    } as const;

    return createPortal(
        <div
            ref={panelRef}
            className="canvas-image-settings-popover aceternity-floating-panel backdrop-blur-2xl"
            style={style}
            onPointerDown={(event) => event.stopPropagation()}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={(event) => event.stopPropagation()}
        >
            <VideoSettingsPanel config={config} onConfigChange={(key, value) => onConfigChange(key, value)} theme={theme} className="space-y-3" taskConstraints={taskConstraints} />
            <div className="mt-3 flex items-center justify-between gap-3 border-t pt-3" style={{ borderColor: theme.toolbar.border }} aria-live="polite">
                <span className="flex min-w-0 items-center gap-1.5 text-[var(--fs-label)]" style={{ color: theme.node.muted }}>
                    <Coins className="size-3.5 shrink-0" />
                    本次预计
                </span>
                <span className="min-w-0 text-right">
                    <span className={`block truncate text-[var(--fs-body)] font-semibold ${creditEstimate.configured ? "" : "text-amber-500"}`}>{creditEstimate.text}</span>
                    <span className="mt-0.5 block text-[var(--fs-tiny)]" style={{ color: theme.node.muted }}>
                        {creditEstimate.loading ? "正在读取服务端价格" : creditEstimate.source === "server" ? "提交时按服务端价格快照扣费" : creditEstimate.source === "local" ? "本地价格仅作参考" : "请管理员配置有效价格"}
                    </span>
                </span>
            </div>
            {children ? <div className="mt-3">{children}</div> : null}
        </div>,
        document.body,
    );
}
