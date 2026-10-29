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

describe("管理后台的接口契约", () => {
    test("只调用后端已经注册的路径", () => {
        for (const path of [
            "/auth/me",
            "/login",
            "/register",
            "/logout",
            "/admin/credits/accounts",
            "/admin/credits/adjust",
            "/admin/storage-locations",
            "/admin/payment-providers",
            "/admin/pricing",
            "/admin/topup-products",
            "/admin/payment-orders",
            "/admin/payment-orders/reconciliation",
            "/admin/payment-orders/expire",
            "/admin/creation-catalog",
            "/admin/registration/policy",
            "/admin/invite-codes",
            "/admin/overview",
            "/admin/users",
            "/admin/projects",
            "/admin/assets",
            "/admin/credit-ledger",
            "/admin/site",
            "/admin/audit",
        ]) {
            expect(source).toContain(`"${path}`);
        }
    });

    test("价格规则后台使用统一 CRUD 路径", () => {
        expect(source).toContain("/admin/pricing/${encodeURIComponent(kind)}");
        expect(source).toContain("/admin/pricing/${encodeURIComponent(kind)}/${encodeURIComponent(id)}");
        expect(source).toContain('name="unitPriceCredits"');
        expect(source).toContain("creditPriceToMicrocredits(form.unitPriceCredits.value)");
        expect(source).not.toMatch(/name="unitPriceMicrocredits"/u);
    });

    test("价格页默认维护渠道档并可从渠道模型直接设价", () => {
        expect(source).toContain('pricingKind: "channel_tier"');
        expect(source).toContain('data-action="set-channel-model-price"');
        expect(source).toContain("state.pricingPreset");
    });

    test("渠道模型页读取并展示上游报价", () => {
        expect(source).toContain('/admin/channels/${encodeURIComponent(channelId)}/models/preview');
        expect(source).toContain('id="channel-catalog-modal"');
        expect(source).toContain("upstreamPricingText");
    });

    test("渠道模型编辑和新增都使用弹窗", () => {
        expect(source).toContain('id="channel-model-modal"');
        expect(source).toContain('data-action="new-channel-model"');
        expect(source).toContain("openChannelModelModal(model)");
        expect(source).toContain("closeChannelModelModal()");
    });

    test("模型渠道页使用左侧渠道列表和右侧模型工作区", () => {
        expect(source).toContain('id="channel-workspace"');
        expect(source).toContain('id="channel-list"');
        expect(source).toContain('id="channel-model-list"');
        expect(source).toContain("renderChannelList");
    });

    test("新增和编辑渠道使用独立弹窗", () => {
        expect(source).toContain('id="channel-form-modal"');
        expect(source).toContain("openChannelFormModal");
        expect(source).toContain("closeChannelFormModal");
        expect(source).toContain('data-action="close-channel-form"');
    });

    test("渠道模型支持关键词、能力和画布状态筛选", () => {
        expect(source).toContain('id="channel-model-filter-form"');
        expect(source).toContain('name="modelKeyword"');
        expect(source).toContain('name="modelCapability"');
        expect(source).toContain('name="modelCanvasStatus"');
        expect(source).toContain("filterChannelModels");
        expect(source).toContain('id="channel-model-filter-count"');
    });

    test("模型关键词失焦不重建表格，避免吞掉行内按钮点击", () => {
        expect(source).toContain('if (target.name === "modelKeyword") return;');
    });

    test("渠道模型价格档支持勾选多个模型批量保存", () => {
        expect(source).toContain('id="pricing-batch-form"');
        expect(source).toContain("data-pricing-batch-model");
        expect(source).toContain('savePricing("channel_tier"');
        expect(source).toContain("findChannelTierRule(channelId, model.id, selector)");
    });

    test("每次请求都带登录 cookie", () => {
        expect(source).toContain('credentials: "include"');
    });

    test("人工调整带幂等键，网络重试不会重复入账", () => {
        expect(source).toContain("buildAdjustIdempotencyKey");
        expect(source).toContain("amountMicrocredits: amount");
    });

    test("积分账户的提交调整表单会实际调用入账接口", () => {
        assert.match(
            source,
            /getAttribute\("id"\) === "adjust-form"\)\s*\{\s*withBusy\(async \(\) => \{[\s\S]*?await adjustCredits\(/s,
            "adjust-form 不能是空分支，也不能把入账逻辑塞进站点配置分支",
        );
    });

    test("不包含用户端页面和用户端接口", () => {
        // 用户端（钱包、充值、项目库、画布）是另一个应用，不该长在管理后台里。
        expect(source).not.toMatch(/\/credits\/(?:wallet|ledger)/u);
        expect(source).not.toMatch(/\/topup\/(?:products|orders)/u);
        expect(source).not.toMatch(/data-action="switch-console"/u);
    });

    test("支付订单后台提供退款和超时清理入口", () => {
        expect(source).toContain("/admin/payment-orders/${encodeURIComponent(id)}/refund");
        expect(source).toContain("data-action=\"expire-orders\"");
        expect(source).toContain("data-action=\"refund-order\"");
    });

    test("七个系统管理页使用真实数据渲染，不再引用占位模块", () => {
        expect(source).not.toMatch(/from "\.\/modules\.js"/u);
        expect(source).not.toMatch(/MODULE_SPECS/u);
        expect(source).not.toMatch(/function renderModule/u);
        for (const marker of ["renderAdminOverview", "renderAdminUserManagement", "renderAdminProjects", "renderAdminAssets", "renderAdminLedger", "renderAdminSite", "renderAdminAudit"]) {
            expect(source).toContain(marker);
        }
    });

    test("用户管理支持添加用户、修改状态角色和重置密码", () => {
        expect(source).toContain('id="user-create-modal"');
        expect(source).toContain('id="create-user-form"');
        expect(source).toContain('name="initialCredits"');
        expect(source).toContain('data-action="open-create-user"');
        expect(source).toContain('data-action="toggle-user-status"');
        expect(source).toContain('data-action="toggle-user-role"');
        expect(source).toContain('data-action="reset-user-password"');
    });

    test("项目和素材治理只提供归档恢复与下架恢复", () => {
        expect(source).toContain('data-action="archive-project"');
        expect(source).toContain('data-action="restore-project"');
        expect(source).toContain('data-action="block-asset"');
        expect(source).toContain('data-action="unblock-asset"');
        expect(source).toContain("下架素材必须填写原因");
    });

    test("站点配置、流水和审计页提供真实筛选与保存表单", () => {
        expect(source).toContain('id="site-config-form"');
        expect(source).toContain('id="ledger-filter-form"');
        expect(source).toContain('id="audit-filter-form"');
        expect(source).toContain('name="start"');
        expect(source).toContain('name="end"');
    });

    test("渠道模型显示画布可见性和有效设价状态", () => {
        expect(source).toContain("画布可见");
        expect(source).toContain("未设价");
        expect(source).toContain('data-action="toggle-channel-model-visibility"');
        expect(source).toContain("renderChannelModelSummary");
    });

    test("价格规则支持人民币成本换算积分", () => {
        expect(source).toContain('data-action="apply-rmb-conversion"');
        expect(source).toContain("pricingRmbConversion");
        expect(source).toContain("元 / 百万 Token");
        expect(source).toContain("normalizeRmbCost");
    });

    test("价格页以渠道模型清单为主，并用单模型弹窗设价", () => {
        expect(source).toContain('id="pricing-workspace-channel"');
        expect(source).toContain('id="pricing-channel-model-table"');
        expect(source).toContain('id="pricing-editor-modal"');
        expect(source).toContain("openPricingEditorModal");
        expect(source).toContain("closePricingEditorModal");
        expect(source).toContain(">设置价格</button>");
        expect(source).toContain(">去定价</button>");
    });

    test("批量设置和高级价格规则默认折叠", () => {
        expect(source).toContain('<details class="pricing-disclosure" id="pricing-batch-details">');
        expect(source).toContain('<details class="pricing-disclosure" id="pricing-advanced-details">');
        expect(source).toContain("高级规则");
        expect(source).toContain("逻辑模型 SKU");
    });

    test("文本价格按每千 Token 计费且最低扣费支持小数", () => {
        expect(source).toContain("每千 Token");
        expect(source).toContain('name="minimumChargeCredits" type="number" min="0" step="0.000001" value="1"');
        expect(source).toContain("minimumChargeMicrocredits");
    });

    test("纯函数收敛在 format.js，不在 app.js 里复制一份", () => {
        expect(source).not.toMatch(/^function escapeHtml/mu);
        expect(source).not.toMatch(/^function formatPoints/mu);
        expect(source).toContain('from "./format.js?v=');
    });
});
