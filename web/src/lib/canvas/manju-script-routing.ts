import type {
    ManjuScriptInputKind,
    ManjuScriptMode,
    ManjuScriptRunInput,
    ManjuSource,
    ManjuTrendResolution,
} from "@/services/api/manju";

export function canCreateManjuScriptRun(source: ManjuSource | "", trendResolution: ManjuTrendResolution) {
    return source !== "trend" || Boolean(trendResolution);
}

export function buildManjuScriptRunInput(input: Omit<ManjuScriptRunInput, "inputKind" | "trendResolution"> & {
    mode: ManjuScriptMode;
    source: ManjuSource | "";
    inputKind: ManjuScriptInputKind;
    trendResolution: ManjuTrendResolution;
}): ManjuScriptRunInput {
    const { source, inputKind, trendResolution, ...rest } = input;
    return {
        ...rest,
        inputKind,
        trendResolution: source === "trend" ? trendResolution : "",
    };
}
