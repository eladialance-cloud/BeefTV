let capabilitySnapshot: WorkspaceCapabilityContract | null = null;

export type WorkspaceProfile = "local" | "remote";

export type WorkspaceCapabilityContract = {
    contractVersion: number;
    profile: WorkspaceProfile;
    capabilities: {
        localAssets: boolean;
        providerCalls: boolean;
    };
};

/** Central runtime boundary shared by local workspace UI and hosted sync. */
export function isLocalWorkspaceMode() {
    return capabilitySnapshot?.profile !== "remote";
}

export function isRemoteWorkspaceMode() {
    return capabilitySnapshot?.profile === "remote";
}

/**
 * Single capability projection for UI boundaries. Keep the hosted profile
 * available, but make the local-first contract explicit instead of repeating
 * storageMode/username checks throughout pages.
 */
export type WorkspaceCapabilities = { local: boolean; remote: boolean; localAssets: boolean; providerCalls: boolean };

export function workspaceCapabilities(): WorkspaceCapabilities {
    const remote = isRemoteWorkspaceMode();
    return {
        local: !remote,
        remote,
        localAssets: capabilitySnapshot?.capabilities.localAssets ?? true,
        providerCalls: capabilitySnapshot?.capabilities.providerCalls ?? true,
    };
}

export function setWorkspaceCapabilitySnapshot(snapshot: WorkspaceCapabilityContract | null) {
    capabilitySnapshot = snapshot;
}

export function resetWorkspaceMode() {
    capabilitySnapshot = null;
}
