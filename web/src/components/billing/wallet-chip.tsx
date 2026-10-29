import { Coins, Plus } from "lucide-react";
import { useEffect, useState } from "react";

import { RechargeDialog } from "@/components/billing/recharge-dialog";
import { cn } from "@/lib/utils";
import { formatCreditAmount } from "@/services/api/credits";
import { isRemoteWorkspaceMode } from "@/services/workspace-mode";
import { useUserStore } from "@/stores/use-user-store";
import { useWalletStore } from "@/stores/use-wallet-store";

/**
 * 积分胶囊：展示当前可用积分并提供充值入口。
 * 只在托管网页会话出现；本地桌面工作区不渲染。
 */
export function WalletChip({ compact = false, className }: { compact?: boolean; className?: string }) {
    const hydrated = useUserStore((state) => state.hydrated);
    const user = useUserStore((state) => state.user);
    const balance = useWalletStore((state) => state.available);
    const loaded = useWalletStore((state) => state.loaded);
    const refresh = useWalletStore((state) => state.refresh);
    const [open, setOpen] = useState(false);

    useEffect(() => {
        if (!hydrated || !user || !isRemoteWorkspaceMode()) return;
        void refresh();
    }, [hydrated, user, refresh]);

    if (!user || !isRemoteWorkspaceMode()) return null;

    return (
        <>
            <button
                type="button"
                className={cn("wallet-chip inline-flex items-center gap-1.5 rounded-full border border-foreground/12 bg-foreground/[.04] px-2.5 py-1 text-[12px] transition-colors hover:border-foreground/25 hover:bg-foreground/[.08]", className)}
                title="当前可用积分，点击充值"
                onClick={() => setOpen(true)}
            >
                <Coins className="size-3.5 text-amber-500" />
                <span className="font-semibold tabular-nums">{loaded ? formatCreditAmount(balance) : "--"}</span>
                {compact ? null : <span className="text-foreground/45">积分</span>}
                <Plus className="size-3 text-foreground/40" />
            </button>
            <RechargeDialog open={open} onClose={() => setOpen(false)} />
        </>
    );
}
