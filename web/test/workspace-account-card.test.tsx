import { afterEach, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router";

import { WorkspaceAccountCard } from "../src/components/layout/workspace-account-card";
import { useUserStore } from "../src/stores/use-user-store";

afterEach(() => {
    useUserStore.setState({
        hydrated: false,
        user: null,
        storageMode: "remote",
    });
});

test("remote account card exposes logout", () => {
    useUserStore.setState({
        hydrated: true,
        storageMode: "remote",
        user: {
            id: "user-1",
            username: "alice",
            displayName: "Alice",
            role: "user",
            status: "active",
        },
    });
    const html = renderToStaticMarkup(
        <MemoryRouter>
            <WorkspaceAccountCard onNavigate={() => undefined} user={useUserStore.getState().user} storageMode="remote" />
        </MemoryRouter>,
    );
    expect(html).toContain("退出登录");
});

test("local workspace does not expose logout", () => {
    useUserStore.setState({
        hydrated: true,
        storageMode: "local",
        user: {
            id: "local",
            username: "local",
            displayName: "本地工作区",
            role: "admin",
            status: "active",
        },
    });
    const html = renderToStaticMarkup(
        <MemoryRouter>
            <WorkspaceAccountCard onNavigate={() => undefined} user={useUserStore.getState().user} storageMode="local" />
        </MemoryRouter>,
    );
    expect(html).not.toContain("退出登录");
});

test("home account trigger renders the user name", async () => {
    (globalThis as typeof globalThis & { __APP_VERSION__?: string }).__APP_VERSION__ = "test";
    const { WorkspaceAccountMenu } = await import("../src/components/layout/workspace-account-menu");
    const user = {
        id: "user-1",
        username: "alice",
        displayName: "Alice",
        role: "user" as const,
        status: "active" as const,
    };
    const html = renderToStaticMarkup(
        <MemoryRouter>
            <WorkspaceAccountMenu showName user={user} storageMode="remote" />
        </MemoryRouter>,
    );
    expect(html).toContain("Alice");
    expect(html).toContain("账户菜单");
});
