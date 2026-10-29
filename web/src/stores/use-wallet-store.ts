import { create } from "zustand";

import { getCreditWallet } from "@/services/api/credits";
import { isRemoteWorkspaceMode } from "@/services/workspace-mode";

/**
 * 用户端钱包快照。
 *
 * 只在托管网页会话里向服务端取余额；本地桌面工作区没有积分概念，
 * 读取会被直接短路，避免桌面入口出现无意义的 401。
 */
type WalletStore = {
    balance: number;
    frozen: number;
    available: number;
    loading: boolean;
    loaded: boolean;
    refresh: () => Promise<void>;
    reset: () => void;
};

export const useWalletStore = create<WalletStore>()((set, get) => ({
    balance: 0,
    frozen: 0,
    available: 0,
    loading: false,
    loaded: false,
    refresh: async () => {
        if (!isRemoteWorkspaceMode()) return;
        if (get().loading) return;
        set({ loading: true });
        try {
            const { account } = await getCreditWallet();
            set({ balance: account.balance, frozen: account.frozen, available: account.available, loaded: true });
        } catch {
            // 余额读取失败不能阻断创作页；下次进入或手动刷新再取。
        } finally {
            set({ loading: false });
        }
    },
    reset: () => set({ balance: 0, frozen: 0, available: 0, loaded: false }),
}));
