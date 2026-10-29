import { afterEach, expect, test } from "bun:test";

import {
    resetWorkspaceMode,
    setWorkspaceCapabilitySnapshot,
    workspaceCapabilities,
} from "../src/services/workspace-mode";

afterEach(() => resetWorkspaceMode());

import { isLocalWorkspaceMode, isRemoteWorkspaceMode } from "../src/services/workspace-mode";

test("workspace capabilities expose only the local runtime contract", () => {
    setWorkspaceCapabilitySnapshot({
        contractVersion: 1,
        profile: "local",
        capabilities: {
            localAssets: true,
            providerCalls: true,
        },
    });

    expect(workspaceCapabilities()).toEqual({
        local: true,
        remote: false,
        localAssets: true,
        providerCalls: true,
    });
});

test("workspace capabilities use safe local defaults before bootstrap", () => {
    expect(workspaceCapabilities()).toEqual({
        local: true,
        remote: false,
        localAssets: true,
        providerCalls: true,
    });
});

test("remote profile switches the runtime boundary away from local", () => {
    setWorkspaceCapabilitySnapshot({
        contractVersion: 1,
        profile: "remote",
        capabilities: {
            localAssets: false,
            providerCalls: true,
        },
    });

    expect(isRemoteWorkspaceMode()).toBe(true);
    expect(isLocalWorkspaceMode()).toBe(false);
    expect(workspaceCapabilities()).toEqual({
        local: false,
        remote: true,
        localAssets: false,
        providerCalls: true,
    });
});
