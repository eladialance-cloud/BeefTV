import { App, Button, Modal } from "antd";
import { Coins, RefreshCw } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { beginTopupPayment, createTopupOrder, formatYuan, getTopupProducts, topupProductCredits, type TopupProduct } from "@/services/api/credits";
import { useWalletStore } from "@/stores/use-wallet-store";

/**
 * 充值弹窗：只展示管理后台启用的积分包，下单后由支付渠道接管。
 * 套餐或支付渠道没配好时给出明确提示，不伪造支付链接。
 */
export function RechargeDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
    const { message } = App.useApp();
    const refreshWallet = useWalletStore((state) => state.refresh);
    const [products, setProducts] = useState<TopupProduct[]>([]);
    const [loading, setLoading] = useState(false);
    const [submittingId, setSubmittingId] = useState("");
    const [error, setError] = useState("");

    const load = useCallback(async () => {
        setLoading(true);
        setError("");
        try {
            const { items } = await getTopupProducts();
            setProducts(items.filter((item) => item.enabled).sort((a, b) => a.sortOrder - b.sortOrder));
        } catch (loadError) {
            setError(loadError instanceof Error ? loadError.message : "充值套餐读取失败");
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        if (open) void load();
    }, [open, load]);

    const submit = async (product: TopupProduct) => {
        setSubmittingId(product.id);
        try {
            const { item } = await createTopupOrder(product.id, product.provider);
            if (item.status === "paid") {
                await refreshWallet();
                message.success(`充值成功，已到账 ${item.credits} 积分`);
                onClose();
                return;
            }
            const { payment } = await beginTopupPayment(item.id);
            if (payment.mode === "redirect" && payment.url) {
                window.location.assign(payment.url);
                return;
            }
            message.info(payment.qrCode ? "支付二维码已生成，请使用对应支付应用扫码付款。" : "订单已创建，请在支付渠道完成付款后返回刷新余额。");
            await refreshWallet();
            onClose();
        } catch (submitError) {
            message.error(submitError instanceof Error ? submitError.message : "创建订单失败");
        } finally {
            setSubmittingId("");
        }
    };

    return (
        <Modal title="充值积分" open={open} onCancel={onClose} footer={null} width={520} destroyOnHidden>
            <p className="m-0 text-[13px] leading-6 text-foreground/55">积分用于生成图片、视频与音频任务；任务失败或取消会自动退回未消耗部分。</p>
            <div className="mt-4 grid gap-2.5">
                {loading ? (
                    <div className="grid gap-2.5">
                        {Array.from({ length: 3 }, (_, index) => (
                            <div key={index} className="h-[68px] animate-pulse rounded-xl bg-foreground/[.06]" />
                        ))}
                    </div>
                ) : null}
                {!loading && error ? (
                    <div className="flex items-center justify-between gap-3 rounded-xl border border-red-500/20 bg-red-500/[.06] px-3.5 py-3 text-[13px] text-red-500">
                        <span>{error}</span>
                        <Button size="small" icon={<RefreshCw className="size-3.5" />} onClick={() => void load()}>
                            重试
                        </Button>
                    </div>
                ) : null}
                {!loading && !error && products.length === 0 ? <div className="rounded-xl border border-dashed border-foreground/15 px-3.5 py-6 text-center text-[13px] text-foreground/50">管理员暂未配置充值套餐，请联系管理员。</div> : null}
                {products.map((product) => (
                    <button
                        key={product.id}
                        type="button"
                        className="flex w-full items-center gap-3 rounded-xl border border-foreground/10 bg-foreground/[.03] px-3.5 py-3 text-left transition-colors hover:border-foreground/25 hover:bg-foreground/[.06] disabled:opacity-60"
                        disabled={Boolean(submittingId)}
                        onClick={() => void submit(product)}
                    >
                        <span className="grid size-9 shrink-0 place-items-center rounded-lg border border-amber-400/25 bg-amber-400/10 text-amber-500">
                            <Coins className="size-4" />
                        </span>
                        <span className="min-w-0 flex-1">
                            <strong className="block truncate text-sm">{product.name}</strong>
                            <small className="block text-xs text-foreground/50">
                                到账 {topupProductCredits(product)} 积分{product.bonus > 0 ? `（含赠送 ${product.bonus}）` : ""}
                            </small>
                        </span>
                        <span className="shrink-0 text-sm font-semibold tabular-nums">¥{formatYuan(product.priceFen)}</span>
                    </button>
                ))}
            </div>
        </Modal>
    );
}
