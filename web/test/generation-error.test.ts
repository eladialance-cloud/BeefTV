import { describe, expect, test } from "bun:test";

import { explainGenerationError } from "../src/lib/generation-error";

function apiQuotaError(message: string) {
    return Object.assign(new Error(message), {
        name: "ApiError",
        reason: "quota_exceeded",
        status: 403,
    });
}

describe("generation quota errors", () => {
    test("wallet insufficiency points to the account balance instead of a channel budget", () => {
        const explained = explainGenerationError(apiQuotaError("可用积分不足"));

        expect(explained.category).toBe("quota_user");
        expect(explained.reason).toBe("可用积分不足");
        expect(explained.action).toContain("充值");
    });

    test("storage quotas keep their workspace cleanup guidance", () => {
        const explained = explainGenerationError(apiQuotaError("账号素材数量已达到 100 个上限"));

        expect(explained.category).toBe("quota_limit");
        expect(explained.action).toContain("清理");
    });
});
