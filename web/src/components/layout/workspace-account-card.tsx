import { ArrowUpRight, LogOut, Settings } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router";
import { appHref } from "@/lib/app-routing";
import { logoutCurrentSession } from "@/services/api/auth";
import { useUserStore, type LocalUser } from "@/stores/use-user-store";
import { UserAvatar } from "./user-avatar";
import "./workspace-account-card.css";

/** 本地工作区账户卡片，只保留本地设置入口。 */
export function WorkspaceAccountCard({
    onNavigate,
    user: providedUser,
    storageMode: providedStorageMode,
}: {
    onNavigate: () => void;
    user?: LocalUser | null;
    storageMode?: "local" | "remote";
}) {
    const storedUser = useUserStore((state) => state.user);
    const user = providedUser === undefined ? storedUser : providedUser;
    const storedStorageMode = useUserStore((state) => state.storageMode);
    const storageMode = providedStorageMode || storedStorageMode;
    const clearSession = useUserStore((state) => state.clearSession);
    const [loggingOut, setLoggingOut] = useState(false);
    if (!user) return null;
    const canLogout = storageMode === "remote" && user.username !== "local";
    const handleLogout = async () => {
        if (loggingOut) return;
        setLoggingOut(true);
        try {
            await logoutCurrentSession();
        } catch {
            // The local session must still be cleared if the server is unavailable.
        } finally {
            clearSession();
            onNavigate();
            window.location.assign(appHref("/login"));
        }
    };
    return <section className="workspace-account-card" aria-label="我的账户">
        <header className="workspace-account-card-identity">
            <UserAvatar user={user} className="workspace-account-card-avatar" />
            <div><strong>{user.displayName || user.username}</strong><span>@{user.username}</span></div>
            <em>{user.role === "admin" ? "管理员" : "创作者"}</em>
        </header>
        <nav className="workspace-account-card-actions" aria-label="账户操作">
            <Link to="/settings" onClick={onNavigate}><Settings /><span>本地设置</span><ArrowUpRight /></Link>
            {canLogout ? (
                <button type="button" className="workspace-account-card-logout" onClick={() => void handleLogout()} disabled={loggingOut}>
                    <LogOut />
                    <span>{loggingOut ? "正在退出" : "退出登录"}</span>
                </button>
            ) : null}
        </nav>
    </section>;
}
