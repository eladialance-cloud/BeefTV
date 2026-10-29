import { Button, Checkbox } from "antd";
import { Eye, EyeOff, LockKeyhole, Ticket, UserRound } from "lucide-react";
import { useEffect, useState } from "react";

import { http } from "@/services/api/request";
import { applyAppearanceMetadata, useAppearanceStore } from "@/stores/use-appearance-store";

type AuthUser = { id: string; username: string; displayName?: string; role: string };
type AuthResponse = { user: AuthUser };

const REMEMBER_KEY = "visideep.login.remember";

function readRememberedCredentials(): { username: string; password: string } | null {
    if (typeof window === "undefined") return null;
    try {
        const raw = window.localStorage.getItem(REMEMBER_KEY);
        if (!raw) return null;
        const parsed = JSON.parse(raw) as { username?: unknown; password?: unknown };
        const username = typeof parsed.username === "string" ? parsed.username : "";
        const password = typeof parsed.password === "string" ? parsed.password : "";
        return username || password ? { username, password } : null;
    } catch {
        return null;
    }
}

/**
 * 网页版用户端登录入口。
 *
 * 视觉比例对齐管理后台：左侧固定宽度视觉区，右侧表单区，
 * 整页锁定在视口内（100dvh + overflow hidden），不产生上下滚动。
 */
export default function LoginPage() {
    const brandName = useAppearanceStore((state) => state.appearance.brandName);
    const appearance = useAppearanceStore((state) => state.appearance);
    const [mode, setMode] = useState<"login" | "register">("login");
    const [remembered] = useState(() => readRememberedCredentials());
    const [username, setUsername] = useState(remembered?.username || "");
    const [displayName, setDisplayName] = useState("");
    const [inviteCode, setInviteCode] = useState("");
    const [password, setPassword] = useState(remembered?.password || "");
    const [remember, setRemember] = useState(Boolean(remembered));
    const [showPassword, setShowPassword] = useState(false);
    const [submitting, setSubmitting] = useState(false);
    const [error, setError] = useState(() => {
        const reason = new URLSearchParams(window.location.search).get("reason");
        return reason === "session_replaced" ? "账号已在其他设备登录，当前设备需要重新登录。" : "";
    });

    // 离开登录页时凭据元数据会重新应用到 document.title，这里只负责进入登录页时改写。
    useEffect(() => {
        document.title = `${brandName} · 登录`;
        return () => applyAppearanceMetadata(appearance);
    }, [brandName, appearance]);

    const submit = async () => {
        const name = username.trim();
        if (!name || !password) {
            setError("请填写登录名和密码");
            return;
        }
        setSubmitting(true);
        setError("");
        try {
            const body = mode === "register" ? { username: name, displayName: displayName.trim() || name, password, inviteCode: inviteCode.trim() } : { username: name, password };
            await http.post<AuthResponse>(mode === "register" ? "/register" : "/login", body);
            try {
                if (remember) window.localStorage.setItem(REMEMBER_KEY, JSON.stringify({ username: name, password }));
                else window.localStorage.removeItem(REMEMBER_KEY);
            } catch {
                // 浏览器禁用本地存储时登录仍然成立，只是不再记住凭据。
            }
            const target = new URLSearchParams(window.location.search).get("next");
            window.location.assign(target && target.startsWith("/") ? target : "/");
        } catch (submitError) {
            setError(submitError instanceof Error ? submitError.message : "操作失败，请稍后重试");
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <div className="login-shell grid h-[100dvh] w-full overflow-hidden bg-[#09090b] text-[#fafafa] lg:grid-cols-[minmax(320px,36%)_minmax(0,1fr)]">
            <aside className="relative hidden overflow-hidden bg-[#101013] lg:block">
                <img className="absolute inset-0 h-full w-full object-cover object-center" src="/login-hero.jpg" alt="" />
                <div className="absolute inset-0 bg-[linear-gradient(to_top,rgba(0,0,0,.82)_0%,rgba(0,0,0,.30)_36%,rgba(0,0,0,0)_64%),linear-gradient(to_right,rgba(0,0,0,.34)_0%,rgba(0,0,0,0)_48%)]" />
                <div className="absolute bottom-11 left-10 right-10">
                    <div className="flex items-center gap-2.5 text-[20px] font-bold [text-shadow:0_1px_14px_rgba(0,0,0,.65)]">
                        <img className="size-7 rounded-lg object-cover" src="/visideep-ai.png" alt="" />
                        {brandName}
                    </div>
                    <p className="mt-2.5 max-w-[340px] text-[13.5px] leading-relaxed text-white/80 [text-shadow:0_1px_14px_rgba(0,0,0,.65)]">释放您的无限创意潜能，打造独一无二的 AI 影视作品。</p>
                </div>
            </aside>

            <section className="grid content-center justify-items-center overflow-y-auto px-8 py-10">
                <form
                    className="w-full max-w-[448px]"
                    onSubmit={(event) => {
                        event.preventDefault();
                        void submit();
                    }}
                >
                    <div className="mb-7 flex items-center gap-2.5 text-[17px] font-bold lg:hidden">
                        <img className="size-7 rounded-lg object-cover" src="/visideep-ai.png" alt="" />
                        {brandName}
                    </div>

                    <h1 className="text-[30px] font-bold leading-tight">{mode === "register" ? "创建账号" : "欢迎登录"}</h1>
                    <p className="mt-2.5 text-sm leading-6 text-white/55">{mode === "register" ? "注册后即可创建你的 AI 影视项目" : "登录后即可进入工作台继续创作"}</p>

                    <div className="mt-8 grid gap-4">
                        <label className="login-field grid gap-2">
                            <span className="text-[13px] font-medium text-white/70">登录名</span>
                            <span className="login-control flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/[0.03] px-3 transition-colors focus-within:border-white/25">
                                <UserRound className="size-4 text-white/40" />
                                <input
                                    className="h-full min-w-0 flex-1 bg-transparent text-sm text-white outline-none placeholder:text-white/30"
                                    autoComplete="username"
                                    placeholder="请输入登录名"
                                    value={username}
                                    onChange={(event) => setUsername(event.target.value)}
                                />
                            </span>
                        </label>

                        {mode === "register" ? (
                            <label className="login-field grid gap-2">
                                <span className="text-[13px] font-medium text-white/70">显示名称</span>
                                <span className="login-control flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/[0.03] px-3 transition-colors focus-within:border-white/25">
                                    <UserRound className="size-4 text-white/40" />
                                    <input
                                        className="h-full min-w-0 flex-1 bg-transparent text-sm text-white outline-none placeholder:text-white/30"
                                        autoComplete="nickname"
                                        placeholder="可选，默认与登录名相同"
                                        value={displayName}
                                        onChange={(event) => setDisplayName(event.target.value)}
                                    />
                                </span>
                            </label>
                        ) : null}

                        {mode === "register" ? (
                            <label className="login-field grid gap-2">
                                <span className="text-[13px] font-medium text-white/70">邀请码（仅邀请注册时需要）</span>
                                <span className="login-control flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/[0.03] px-3 transition-colors focus-within:border-white/25">
                                    <Ticket className="size-4 text-white/40" />
                                    <input
                                        className="h-full min-w-0 flex-1 bg-transparent text-sm text-white outline-none placeholder:text-white/30"
                                        autoComplete="off"
                                        placeholder="开放注册时可留空"
                                        value={inviteCode}
                                        onChange={(event) => setInviteCode(event.target.value)}
                                    />
                                </span>
                            </label>
                        ) : null}

                        <label className="login-field grid gap-2">
                            <span className="text-[13px] font-medium text-white/70">密码</span>
                            <span className="login-control flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/[0.03] px-3 transition-colors focus-within:border-white/25">
                                <LockKeyhole className="size-4 text-white/40" />
                                <input
                                    className="h-full min-w-0 flex-1 bg-transparent text-sm text-white outline-none placeholder:text-white/30"
                                    type={showPassword ? "text" : "password"}
                                    autoComplete={mode === "register" ? "new-password" : "current-password"}
                                    placeholder="请输入密码"
                                    value={password}
                                    onChange={(event) => setPassword(event.target.value)}
                                />
                                <button type="button" className="grid size-6 shrink-0 place-items-center text-white/40 transition-colors hover:text-white/80" aria-label={showPassword ? "隐藏密码" : "显示密码"} onClick={() => setShowPassword((value) => !value)}>
                                    {showPassword ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                                </button>
                            </span>
                        </label>
                    </div>

                    <Checkbox className="mt-4 !text-[13px] !text-white/55" checked={remember} onChange={(event) => setRemember(event.target.checked)}>
                        记住账号密码
                    </Checkbox>

                    {error ? <div className="mt-4 rounded-xl border border-red-500/25 bg-red-500/10 px-3.5 py-2.5 text-[13px] leading-5 text-red-300">{error}</div> : null}

                    <Button className="mt-6 !h-11 !w-full !rounded-xl !border-transparent !bg-white !text-[15px] !font-semibold !text-[#09090b] hover:!bg-white/90" type="primary" htmlType="submit" loading={submitting}>
                        {mode === "register" ? "注册并进入" : "登录"}
                    </Button>

                    <div className="mt-5 text-center text-[13px] text-white/45">
                        {mode === "register" ? "已有账号？" : "还没有账号？"}
                        <button
                            type="button"
                            className="ml-1.5 font-medium text-white/85 transition-colors hover:text-white"
                            onClick={() => {
                                setMode((current) => (current === "register" ? "login" : "register"));
                                setError("");
                            }}
                        >
                            {mode === "register" ? "去登录" : "立即注册"}
                        </button>
                    </div>
                </form>
            </section>
        </div>
    );
}
