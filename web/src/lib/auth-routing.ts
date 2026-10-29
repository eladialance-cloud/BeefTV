import { appPathname, appSearch } from "@/lib/app-routing";

/**
 * 公开的登录/注册路径不走工作区 bootstrap。
 *
 * 这些页面在会话成立之前就要渲染，不能因为 `/workspace/bootstrap` 返回 401
 * 又被重定向回登录页。判断只覆盖明确的认证入口，其余所有路径仍是受保护工作区。
 */
export function isPublicAuthPath(pathname: string = appPathname()) {
    const path = pathname.split("?")[0].replace(/\/+$/, "") || "/";
    return path === "/login" || path === "/register";
}

export function shouldRedirectToLogin(status?: number, pathname: string = appPathname()) {
    return status === 401 && !isPublicAuthPath(pathname);
}

export function loginRedirectPath(pathname: string = appPathname(), search: string = appSearch()) {
    const next = encodeURIComponent(`${pathname}${search || ""}`);
    return `/login?next=${next}&reason=session_replaced`;
}
