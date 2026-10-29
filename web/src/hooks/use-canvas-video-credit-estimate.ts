import { useEffect, useMemo, useRef, useState } from "react";

import {
    canvasModelCreditFallback,
    canvasModelPricingRequest,
    canvasModelPricingSignature,
    createLatestQuoteGate,
    formatServerCreditEstimate,
    missingCreditEstimate,
    type CanvasCreditEstimate,
} from "@/lib/canvas/canvas-credit-estimate";
import { ApiError } from "@/services/api/request";
import { quotePricing } from "@/services/api/pricing";
import type { AiConfig, ModelCapability } from "@/stores/use-config-store";

type StoredEstimate = {
    requestKey: string;
    estimate: CanvasCreditEstimate;
};

const unavailableEstimate: CanvasCreditEstimate = {
    configured: false,
    text: "积分报价暂不可用",
    title: "暂时无法读取服务端报价，本地也没有可用的模型价格",
    source: "none",
};

export function useCanvasVideoCreditEstimate(config: AiConfig) {
    return useCanvasModelCreditEstimate(config, "video", config.model || config.videoModel);
}

export function useCanvasModelCreditEstimate(config: AiConfig, capability: ModelCapability, modelValue?: string, enabled = true) {
    const requestKey = enabled ? canvasModelPricingSignature(config, capability, modelValue) : "";
    const request = useMemo(() => canvasModelPricingRequest(config, capability, modelValue), [requestKey]);
    const fallback = useMemo(
        () => canvasModelCreditFallback(config, capability, modelValue),
        [capability, config.channels, config.count, config.model, config.quality, config.size, config.videoModel, config.videoSeconds, config.vquality, modelValue],
    );
    const gate = useRef(createLatestQuoteGate());
    const [stored, setStored] = useState<StoredEstimate | null>(null);

    useEffect(() => {
        if (!request || !requestKey) {
            gate.current.invalidate();
            setStored(null);
            return;
        }
        const requestRevision = gate.current.begin();
        const controller = new AbortController();
        void quotePricing(request, controller.signal)
            .then(({ quote }) => {
                if (!gate.current.isCurrent(requestRevision)) return;
                setStored({ requestKey, estimate: formatServerCreditEstimate(quote) });
            })
            .catch((error: unknown) => {
                if (controller.signal.aborted || !gate.current.isCurrent(requestRevision)) return;
                const missing = error instanceof ApiError && (error.status === 404 || error.code === 404);
                setStored({
                    requestKey,
                    estimate: missing ? missingCreditEstimate() : fallback.configured ? fallback : unavailableEstimate,
                });
            });
        return () => {
            controller.abort();
            gate.current.invalidate();
        };
    }, [fallback.configured, fallback.source, fallback.text, fallback.title, requestKey]);

    const current = stored?.requestKey === requestKey ? stored.estimate : null;
    if (current) return { ...current, loading: false };
    if (!request) return { ...fallback, loading: false };
    if (fallback.configured) return { ...fallback, loading: true };
    return {
        configured: false,
        text: "正在计算积分",
        title: "正在按当前模型参数读取服务端价格",
        source: "none" as const,
        loading: true,
    };
}
