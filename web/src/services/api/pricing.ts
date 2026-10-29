import { http } from "@/services/api/request";

export type PricingQuoteRequest = {
    logicalModelId?: string;
    channelId?: string;
    channelModelId?: string;
    modelKey?: string;
    capability: string;
    billingMode: string;
    selector?: Record<string, string>;
    quantity: number;
};

export type PricingQuote = {
    ruleId: string;
    ruleKind: string;
    ruleName?: string;
    billingMode: string;
    quantity: number;
    unitPriceMicrocredits: number;
    minimumChargeCredits: number;
    estimatedCredits: number;
    effectiveFrom: string;
    effectiveTo?: string;
};

/** Read-only server quote. Task submission remains the authoritative charge. */
export function quotePricing(request: PricingQuoteRequest, signal?: AbortSignal) {
    return http.post<{ quote: PricingQuote }>("/pricing/quote", request, { signal });
}
