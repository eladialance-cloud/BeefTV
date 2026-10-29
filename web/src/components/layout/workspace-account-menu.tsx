import { Popover } from "antd";
import { useState } from "react";
import { UserRound } from "lucide-react";

import { AppChangelogButton } from "@/components/layout/app-changelog-modal";
import { WorkspaceAccountCard } from "./workspace-account-card";
import { UserAvatar } from "./user-avatar";
import { useUserStore } from "@/stores/use-user-store";
import { cn } from "@/lib/utils";

/** 顶部与侧栏复用同一账户卡片，并保留版本入口。 */
export function WorkspaceAccountMenu({
    showName = false,
    user: providedUser,
    storageMode: providedStorageMode,
}: {
    showName?: boolean;
    user?: ReturnType<typeof useUserStore.getState>["user"];
    storageMode?: ReturnType<typeof useUserStore.getState>["storageMode"];
}) {
    const storedUser = useUserStore((state) => state.user);
    const storedHydrated = useUserStore((state) => state.hydrated);
    const user = providedUser === undefined ? storedUser : providedUser;
    const hydrated = providedUser === undefined ? storedHydrated : true;
    const [menuOpen, setMenuOpen] = useState(false);

    if (!hydrated) {
        return <span className="size-9 animate-pulse rounded-[var(--r-md)] bg-foreground/[.06]" aria-hidden />;
    }

    return user ? (
        <><Popover
            trigger="click"
            placement="bottomRight"
            rootClassName="workspace-account-popover"
            open={menuOpen}
            onOpenChange={setMenuOpen}
            content={(
                <div className="workspace-topbar-account-menu">
                    <WorkspaceAccountCard onNavigate={() => setMenuOpen(false)} user={user} storageMode={providedStorageMode} />

                    <div className="workspace-topbar-account-section">
                        <AppChangelogButton className="flex h-8 w-full items-center gap-2 rounded px-2 text-[var(--fs-label)] text-foreground/58 hover:bg-surface-hover hover:text-foreground [&_svg]:size-3.5" showLabel showVersion versionClassName="ml-auto text-[var(--fs-micro)] tabular-nums text-foreground/32" />
                    </div>
                </div>
            )}
        >
            <button type="button" className={cn("app-workspace-topbar-icon-button app-workspace-account-trigger", showName && "app-workspace-account-trigger--named")} aria-label="账户菜单" title={user.displayName || user.username}>
                <UserAvatar user={user} className="size-6" />
                {showName ? <span className="app-workspace-account-trigger-name">{user.displayName || user.username}</span> : null}
            </button>
        </Popover></>
    ) : <span className="app-workspace-topbar-icon-button grid place-items-center" aria-label="本地工作区" title="本地工作区"><UserRound className="size-4 opacity-70" /></span>;
}
