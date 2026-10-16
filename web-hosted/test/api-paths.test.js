import assert from "node:assert/strict";
import { describe, test } from "node:test";

// node:test 不提供 expect；这里给最小断言包装，保持用例可读。
function expect(actual) {
    return {
        toContain: (wanted) => assert.ok(String(actual).includes(wanted), `缺少 ${wanted}`),
        not: {
            toMatch: (pattern) => assert.doesNotMatch(actual, pattern),
        },
    };
}
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const source = readFileSync(resolve(here, "..", "app.js"), "utf8");

describe("托管工作台的接口契约", () => {
    test("只调用后端已经注册的路径", () => {
        for (const path of [
            "/auth/me",
            "/login",
            "/register",
            "/logout",
            "/credits/wallet",
            "/credits/ledger",
            "/admin/credits/accounts",
            "/admin/credits/adjust",
            "/admin/storage-locations",
            "/admin/payment-providers",
            "/admin/pricing",
            "/topup/products",
            "/topup/orders",
            "/admin/topup-products",
            "/admin/payment-orders",
        ]) {
            expect(source).toContain(`"${path}`);
        }
    });

    test("价格规则后台使用统一 CRUD 路径", () => {
        expect(source).toContain("/admin/pricing/${encodeURIComponent(kind)}");
        expect(source).toContain("/admin/pricing/${encodeURIComponent(kind)}/${encodeURIComponent(id)}");
    });

    test("每次请求都带登录 cookie", () => {
        expect(source).toContain('credentials: "include"');
    });

    test("人工调整带幂等键，网络重试不会重复入账", () => {
        expect(source).toContain("buildAdjustIdempotencyKey");
    });

    test("充值下单后继续拉起支付", () => {
        expect(source).toContain("/topup/orders/${encodeURIComponent(orderId)}/pay");
    });

    test("纯函数收敛在 format.js，不在 app.js 里复制一份", () => {
        expect(source).not.toMatch(/^function escapeHtml/mu);
        expect(source).not.toMatch(/^function formatPoints/mu);
        expect(source).toContain('from "./format.js"');
    });
});
