import { http } from "@/services/api/request";

/**
 * 用户端积分与充值接口。
 *
 * 只读取自己的钱包、流水和可购买商品；创建订单后由支付渠道接管跳转。
 * 真实支付参数与结算回调都在服务端，浏览器不接触密钥。
 */

export type CreditWallet = {
    userId: string;
    balance: number;
    frozen: number;
    available: number;
    version: number;
};

export type CreditLedgerEntry = {
    id: string;
    userId: string;
    kind: string;
    stage: string;
    taskId?: string;
    amount: number;
    balanceBefore: number;
    balanceAfter: number;
    frozenBefore: number;
    frozenAfter: number;
    reason: string;
    operatorId?: string;
    createdAt: string;
};

export type TopupProduct = {
    id: string;
    name: string;
    priceFen: number;
    credits: number;
    bonus: number;
    provider: string;
    enabled: boolean;
    sortOrder: number;
};

export type PaymentOrder = {
    id: string;
    userId: string;
    productId: string;
    productName: string;
    provider: string;
    amountFen: number;
    credits: number;
    status: string;
    providerTradeNo?: string;
    createdAt: string;
    expiresAt?: string;
    paidAt?: string;
};

export function getCreditWallet() {
    return http.get<{ account: CreditWallet }>("/credits/wallet");
}

export function getCreditLedger(limit = 50) {
    return http.get<{ items: CreditLedgerEntry[] }>("/credits/ledger", { params: { limit } });
}

export function getTopupProducts() {
    return http.get<{ items: TopupProduct[] }>("/topup/products");
}

export function getPaymentOrders(limit = 50) {
    return http.get<{ items: PaymentOrder[] }>("/topup/orders", { params: { limit } });
}

export type PaymentLaunch = {
    provider: string;
    mode: "redirect" | "qr";
    url?: string;
    qrCode?: string;
};

export function beginTopupPayment(orderId: string) {
    return http.post<{ payment: PaymentLaunch }>(`/topup/orders/${encodeURIComponent(orderId)}/pay`);
}

export function createTopupOrder(productId: string, provider = "") {
    return http.post<{ item: PaymentOrder }>("/topup/orders", { productId, provider });
}

export function topupProductCredits(product: TopupProduct) {
    return product.credits + product.bonus;
}

export function formatCreditAmount(microcredits: number) {
    const value = Number(microcredits);
    if (!Number.isFinite(value)) return "0";
    return (value / 1_000_000).toLocaleString("zh-CN", { maximumFractionDigits: 6 });
}

export function formatYuan(fen: number) {
    return (fen / 100).toFixed(2);
}
