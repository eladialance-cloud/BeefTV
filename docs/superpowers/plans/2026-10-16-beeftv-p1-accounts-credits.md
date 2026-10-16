# BeefTV P1 实施计划：账号体系与积分账本

**Goal:** 把「单工作区本地工具」抬成「有登录、有租户隔离、有积分账本」的多用户平台基座，为 P2 充值支付与 P3 技能计费铺好底账。

**Spec:** `docs/superpowers/specs/2026-10-16-beeftv-web-credits-design.md` 第 3、4、7、8 节。

**Architecture:** 托管专属能力（认证、会话、钱包、账本、发积分）全部放「只在 server profile 装配」的包与文件里；桌面形态只看到既有 `currentUser` 契约，不引入任何新依赖。身份表与账本表命名对齐上游 `hostedTables()`：`users`、`auth_sessions`、`user_identities`、`credit_accounts`、`credit_ledger_entries`。

## 硬约束（写错就跑不过守卫）

1. `TestDesktopDependencyBoundary`：`cmd/desktop` 的依赖图**不得出现** `infinite-canvas/backend/internal/auth`。因此 auth 只在 `cmd/server` / `internal/hostedapi` 里构造与引用；`bootstrap` 只接收注入的接口或钩子。
2. `TestLocalSourceExcludesHostedContentVerticals`：`internal/model/models_identity.go` **不得出现** `PasswordHash`、`type AuthSession struct`、`type UserIdentity struct` 等成员。托管身份模型另起文件（`models_hosted_identity.go`）。
3. 同一守卫的 `nm` 清单禁止 `app.Service` 上出现 `Wallet`、`AdminAdjustCredits`、`SettleBilling`、`RefundBilling`、`AdminUsers` 等**方法名**。积分与后台逻辑放独立包或换名，别挂在这些名字上。
4. 迁移分流已就绪：托管表只进 `database.HostedModels()`，绝不进 `LocalModels()`，否则桌面迁移会先建后删。
5. `model.User` 表名是 `users`，属于托管表；本地 owner 是 `model.Workspace` 的投影（`workspace.LocalPrincipal`）。本地形态不得写入 `users`。

## Task 1: `internal/auth` 基座（密码哈希 + 会话令牌 + 仓储端口）

**Files:**
- Create: `backend/internal/model/models_hosted_identity.go`
- Create: `backend/internal/auth/ports.go`
- Create: `backend/internal/auth/password.go`
- Create: `backend/internal/auth/session.go`
- Create: `backend/internal/auth/service.go`
- Test: `backend/internal/auth/service_test.go`
- Modify: `backend/internal/database/hosted_schema.go`
- Test: `backend/internal/database/hosted_schema_test.go`

**行为**

- `UserIdentity`：`provider`（首发 `password`）+ `identifier`（登录名或邮箱）唯一，`password_hash` 只存 bcrypt 摘要，JSON 永不外泄（`json:"-"`）。
- `AuthSession`：只存**令牌哈希**（SHA-256），明文令牌只在签发时返回一次；带 `expires_at` 与 `revoked_at`。
- `auth.Service`：`Register`（建用户 + 建密码身份，用户名冲突报错）、`Authenticate`（校验身份与密码）、`IssueSession`（签发令牌）、`ValidateSession`（校验并返回用户，过期/撤销一律拒绝）、`RevokeSession`（登出）。
- 仓储走端口 `auth.Repository`，`internal/repository` 提供 GORM 适配；端口不依赖 gin。

- [x] **Step 1: 写失败测试**：密码哈希可校验且错误密码不通过；同一密码两次哈希不同（带盐）；签发令牌不回存明文（仓储里只有哈希）；过期会话被拒；撤销会话立刻失效；注册重名报错；校验通过返回正确用户。
- [x] **Step 2: 运行确认失败**。
- [x] **Step 3: 实现**。
- [x] **Step 4: 运行 `go test ./internal/auth/ ./internal/database/` 通过**。
- [x] **Step 5: 提交**：`feat(backend): 账号体系 - 新增密码与会话基座`。
## Task 2: 登录态 HTTP 入口（仅 server profile）

**Files:**
- Create: `backend/internal/hostedapi/auth.go`
- Test: `backend/internal/hostedapi/auth_test.go`
- Modify: `backend/cmd/server/main.go`

**行为**

- 路由（对齐上游既有命名）：`POST /api/register`、`POST /api/login`、`POST /api/logout`、`GET /api/auth/me`。
- 登录态用 HttpOnly + SameSite=Lax 的会话 cookie 承载明文令牌；响应体不回传令牌。
- 登录态解析收敛到 `SessionUser/SessionUserResolver`：解析 cookie → `ValidateSession` → 返回用户。它是 `/api/auth/me` 与后台路由复用同一个「谁在操作」定义的接缝。
- 只在 `registerHostedRoutes` 装配；桌面路由图不变。

- [x] **Step 1: 写失败测试**：未登录访问 `/api/auth/me` 返回 401；登录成功后带 cookie 再访问返回同一用户；登出后 cookie 失效；错误密码不建会话。
- [x] **Step 2: 运行确认失败**。
- [x] **Step 3: 实现**。
- [x] **Step 4: 运行 `go test ./internal/hostedapi/ ./cmd/server/` 通过**。
- [x] **Step 5: 提交**：`feat(backend): 账号体系 - 新增登录注册与会话中间件`。

## Task 3: `currentUser` 接入会话并做租户隔离

**Files:**
- Create: `backend/internal/handler/session_identity.go`
- Modify: `backend/internal/handler/local_identity.go`
- Create: `backend/internal/hostedapi/session.go`
- Modify: `backend/internal/bootstrap/config.go`、`backend/internal/bootstrap/runtime.go`
- Modify: `backend/cmd/server/main.go`
- Test: `backend/internal/handler/session_identity_test.go`、`backend/internal/hostedapi/session_test.go`

**行为**

- 托管形态下 `currentUser` 先取会话用户；没有会话就是 401，绝不回退到共享工作区 owner。本地形态行为与文案不变。
- 会话中间件只由 `cmd/server` 注入（`HostedRouterMiddleware` 钩子），且必须在 `/api` Group 创建前挂上：gin 的 `RouterGroup.Use` 不回溯已注册路由。
- 后台存储路由的操作人从「工作区 owner」改成会话用户，堵住「未登录就是管理员」的洞；`CANVAS_BOOTSTRAP_ADMIN_USERNAME` 指定注册即管理员的引导账号。
- 路由已普遍按 `user.ID` 取数据，因此换成真用户后自然分租；画布维度的端到端隔离已由 Task 3b 验证，其余资源类型待补。

- [x] **Step 1: 身份接缝测试**：托管请求优先取会话用户；缺会话返回 401；无 hosted 标记时桌面路径不采纳会话用户。
- [x] **Step 2: 中间件测试**：每个请求都被标记为托管；有效 cookie 发布用户；伪造 cookie 不发布；两名用户解析成不同身份。
- [x] **Step 3: 实现**。
- [x] **Step 4: 运行 `go test ./internal/handler/ ./internal/hostedapi/ ./internal/bootstrap/ ./cmd/server/` 通过，边界守卫全绿**。
- [x] **Step 5: 提交**：`feat(backend): 多用户 - 会话用户接管工作区身份`。

## Task 3b: 两用户互不可见的端到端验证（未交付）

- [x] 画布隔离（`internal/handler/hosted_isolation_test.go`）：在真实路由 + 托管 schema 下建两个用户，验证无会话 401、Bob 读 Alice 画布返回 404 且响应不含其内容、列表不泄露、同 ID 覆盖写被拒且原内容不变。
- [ ] 把同样的隔离用例扩到素材、任务、项目维度的其他资源类型。
- [x] 管理员账号回归（`internal/hostedapi/admin_bootstrap_test.go`）：走完整链路——注册拿会话 cookie → 后续请求解析会话 → 后台路由按角色拦截。引导账号能进后台；普通注册用户 403；无会话 401；普通用户仍可读自己的钱包。

## Task 4: 积分账本与三段式扣费

**Files:**
- Create: `backend/internal/model/models_credits.go`
- Create: `backend/internal/credits/ports.go`（仓储端口契约）
- Create: `backend/internal/credits/service.go`（三段式扣费服务）
- Test: `backend/internal/credits/service_test.go`（内存假仓储）
- Create: `backend/internal/repository/credits.go`（GORM 适配器，单事务 + 幂等）
- Test: `backend/internal/repository/credits_test.go`（真 SQLite 事务/回滚/重放）
- Modify: `backend/internal/database/hosted_schema.go`、`backend/internal/database/hosted_schema_test.go`
- Modify: `backend/internal/bootstrap/desktop_dependencies_test.go`（把 `internal/credits` 加进桌面禁用依赖清单）

**行为**

- `credit_accounts`：`balance` + `frozen` + `version`（乐观锁），余额恒不为负。
- `credit_ledger_entries`：不可变流水，幂等键 `task_id + stage`，含变更前后余额、类型、操作人、原因。
- 三段式：提交冻结 → 执行不改余额 → 成功按实际用量结算释放多余冻结，失败/取消全额释放。重复提交、重试、回调收敛到同一笔账。
- 不变量测试：`balance + frozen` 等于流水累计；并发提交不超扣；重复结算不双扣。

- [x] **Step 1: 服务层测试**（`internal/credits/service_test.go`，内存假仓储）：冻结不动余额、可用额不足拒绝且不落流水、同一任务重复冻结幂等、结算释放冻结并按实际扣款、重复结算不双扣、超估在余额内可结算、余额不足超估拒绝并整体回滚、无冻结结算报 409、释放不动余额、无冻结释放拒绝、超扣调整拒绝、充值按订单号幂等、流水可重算账户。
- [x] **Step 2: 适配器测试**（`internal/repository/credits_test.go`，真 SQLite）：首次移动自动建钱包、适配器回填前后值、重放收敛到既有流水且不重复冻结、mutate 报错整事务回滚、负余额不变量被拒、流水按用户隔离且时间倒序。
- [x] **Step 3: 并发测试**：`TestConcurrentReserveNeverOversubscribes` 用 20 路并发冻结验证「余额 100、每笔 10 只成功 10 笔」。
- [x] **Step 4: 运行**：`go test ./internal/credits/`、`go test -run 'TestApplyEntry|TestCreditEntries|TestWalletMissing' ./internal/repository/`、`go test -run 'TestMigrateHostedSchema|TestRequireHostedSchema' ./internal/database/` 全通过；`TestDesktopDependencyBoundary` 通过（`internal/credits` 已加入桌面禁用依赖清单）。
- [x] **Step 5: 提交**：`feat(backend): 积分账本 - 新增账户冻结结算与不可变流水`。

## Task 5a: 钱包查询与管理员发积分（用户端 + 管理端 HTTP）

**Files:**
- Create: `backend/internal/hostedapi/credits.go`
- Test: `backend/internal/hostedapi/credits_test.go`
- Modify: `backend/internal/credits/{ports,service}.go`、`backend/internal/repository/credits.go`（新增 `CreditAccounts` / `Accounts` 管理端列表）
- Modify: `backend/cmd/server/main.go`（只在 server profile 装配）

**行为**

- 用户端：`GET /api/credits/wallet`（余额 / 冻结 / 可用 / 版本）、`GET /api/credits/ledger?limit=`（只看自己，倒序）。
- 管理端：`GET /api/admin/credits/accounts`（钱包列表 + 用户目录拼接）、`POST /api/admin/credits/adjust`（人工加减积分，必填原因，写不可变流水，可选幂等键）。
- 方法名避开守卫清单（用 `Account` / `Accounts` / `Adjust`，不用 `Wallet` / `AdminAdjustCredits`）。
- 价格规则表（`model_pricings` 等）已由 Task 5b 落地；这里只保留钱包与人工发积分。

- [x] **Step 1: 写测试**：未登录读钱包 401；钱包返回余额/冻结/可用；用户只看自己的流水；非法 limit 400；非管理员调整 403；管理员调整后余额与流水一致且带操作人与原因；重复幂等键不二次入账；账户列表拼上用户名；非管理员查列表 403。
- [x] **Step 2: 实现路由与服务方法**。
- [x] **Step 3: 运行**：`go test -run TestCredit ./internal/hostedapi/`、`go test ./internal/credits/`、`go vet ./internal/hostedapi/ ./internal/credits/ ./internal/repository/ ./cmd/server/` 全通过。
- [x] **Step 4: 提交**：`feat(backend): 积分 - 新增钱包查询与管理员发积分`。

## Task 5b: 价格规则表（本次完成）

**Files:**
- Create: `backend/internal/model/models_pricing.go`、`backend/internal/credits/pricing.go`、`backend/internal/hostedapi/pricing.go`
- Modify: `backend/internal/database/hosted_schema.go`
- Create: `backend/internal/repository/pricing.go`
- Test: `backend/internal/credits/pricing_test.go`、`backend/internal/repository/pricing_test.go`、`backend/internal/hostedapi/pricing_test.go`
- Modify: `backend/cmd/server/main.go`、`web-hosted/app.js`、`web-hosted/format.js`

**行为**

- `model_pricings` / `channel_model_price_tiers` / `logical_model_price_skus` 落库，带生效区间，管理员可在后台改价，无需发版。
- 定价维度：渠道 / 模型、能力（文本 / 图片 / 视频 / 音频）、分辨率与时长档、单价、生效区间。
- 报价优先级：逻辑模型 SKU → 渠道模型价格档 → 模型默认价；同一生效时间按排序号升序决胜。
- 单价以微积分存储（`1 积分 = 1,000,000 微积分`），支持每次请求、每秒、每张图、每千 token、每 MB 五种计费方式。
- 与 Task 4 的三段式合同衔接：任务提交时按生效价规则算预冻结，结算时按实际用量算实扣。

- [x] **Step 1: 写失败测试**：报价按生效区间取新价/旧价；同一逻辑模型可选多个 SKU；同刻按排序号决胜；最低扣费可改回 0；管理员校验。
- [x] **Step 2: 运行确认失败**。
- [x] **Step 3: 实现模型、仓储、报价服务、后台 CRUD 路由与前端价格页**。
- [x] **Step 4: 运行相关包测试通过**。
- [x] **Step 5: 提交**：`feat(backend): 积分 - 新增价格规则与后台改价`。

**仍未做**：任务提交时保存报价快照，并接入 `credits.Reserve/Settle/Release`，这是价格规则真正驱动扣费的下一步。

## Task 6: 托管工作台前端（登录、钱包、后台）

**关键修正：托管界面不能放进 `web/src`。** `web/test/local-only-source-boundary.test.ts` 硬性禁止
`src/pages/auth/login.tsx`、`src/services/api/wallet.ts`、`src/pages/admin/index.tsx` 等托管入口存在，
目的是让桌面安装包不携带 SaaS 控制面。因此托管前端独立成 `web-hosted/`，与 `web/` 共用同一后端 API。

**Files:**
- Create: `web-hosted/index.html`、`web-hosted/styles.css`、`web-hosted/app.js`、`web-hosted/format.js`
- Create: `web-hosted/package.json`、`web-hosted/README.md`
- Test: `web-hosted/test/format.test.js`、`web-hosted/test/api-paths.test.js`

**行为**

- 未登录只有登录/注册卡；登录后按角色显示侧栏（普通用户看不到后台）。
- 钱包页：余额 / 冻结 / 可用三个指标 + 最近 50 条流水，类型与阶段翻译成中文。
- 后台积分页：账户列表（拼用户名）+ 调整表单（必填原因、每次带唯一幂等键）。
- 后台对象存储页：列表 + 新增 + 连通性测试 + 设默认 + 删除，密钥只回显掩码。
- 零依赖：无构建步骤、无 npm 依赖，任意静态服务器可托管。

- [x] **Step 1: 写测试**：纯函数（转义/时间/金额/枚举映射/API 基地址/幂等键）；接口契约（只调用已注册路径、每次都带 cookie、人工调整带幂等键）。
- [x] **Step 2: 实现**。
- [x] **Step 3: 运行**：`node --check app.js format.js`、`node --test --test-isolation=none test/format.test.js test/api-paths.test.js` 全通过（16 项）。
- [x] **Step 4: 提交**：`feat(web-hosted): 多用户 - 新增登录、钱包与后台工作台`。

**未做（依赖 P2）**：充值页与支付渠道页。后端 `internal/payment` 未落地前不放空壳表单。

## Task 6b: 支付渠道管理后台（本次完成）

**目标**：先把「支付渠道配置」的管理入口做实，为 P2 的充值下单 / 回调 / 对账打地基。

**Files:**
- Create: `backend/internal/model/models_payment.go`、`backend/internal/payment/{config,secrets}.go`、`backend/internal/repository/payment.go`
- Create: `backend/internal/hostedapi/payment.go`、`backend/internal/payment/config_test.go`、`backend/internal/hostedapi/payment_test.go`
- Modify: `backend/internal/database/hosted_schema.go`（只进 `HostedModels()`）、`backend/internal/database/hosted_schema_test.go`、`backend/cmd/server/main.go`、`web-hosted/app.js`、`web-hosted/README.md`

**行为**

- 管理员可增删改查支付宝 / 微信商户号：名称、AppID、商户号、回调地址、启用状态、排序。
- 密钥（API Key / 应用私钥 / 平台公钥）打包成一个 JSON，AES-GCM 加密后落一列密文；响应只回显掩码与 `secretSet`。
- 编辑时密钥留空 = 保留原密文，填了新值才覆盖。
- 服务层与 HTTP 层都校验管理员身份，写操作全部入管理员审计。
- `internal/payment` 已在 `TestDesktopDependencyBoundary` 禁用清单里，桌面二进制不会携带支付控制面。

- [x] **Step 1: 写测试**：不支持渠道被拒；缺失密钥被拒；普通用户被拒；密钥加密且视图不含明文；留空保留原密钥；列表 / 新增 / 更新 / 删除 / 删除不存在 404。
- [x] **Step 2: 实现服务、仓储、路由、托管建表与前端页**。
- [x] **Step 3: 运行**：`go vet ./internal/payment/ ./internal/hostedapi/ ./cmd/server/ ./internal/database/`；`go test -count=1 ./internal/payment/ ./internal/hostedapi/`；`go test -run TestMigrateHostedSchema ./internal/database/`；`go test -run TestDesktopDependencyBoundary ./internal/bootstrap/`；`node --test web-hosted/test/format.test.js web-hosted/test/api-paths.test.js` 全通过。
- [x] **Step 4: 提交**：`feat(backend): 支付 - 新增支付渠道管理后台入口`。

**当时未做（后来由 Task 7、Task 8 完成）**：充值商品、支付订单、回调验签与幂等、支付宝电脑网站支付与微信 Native 的实际适配器。

## Task 7: 充值商品与支付订单（本次完成）

**目标**：把「选套餐 → 生成订单 → 入账」的主链跑通，并预留网关回调接口。

**Files:**
- Create: `backend/internal/model/models_topup.go`、`backend/internal/payment/topup.go`、`backend/internal/payment/topup_test.go`
- Create: `backend/internal/hostedapi/topup.go`、`backend/internal/hostedapi/topup_test.go`
- Modify: `backend/internal/repository/payment.go`、`backend/internal/database/hosted_schema.go`、`backend/cmd/server/main.go`
- Modify: `web-hosted/app.js`、`web-hosted/format.js`、测试与 README

**行为**

- `topup_products`：套餐名、价格（分）、基础积分、赠送积分、默认渠道、上架、排序。
- `payment_orders`：下单时快照套餐名、金额、总积分，写死入账幂等键 `topup:<orderId>:recharge`。
- 用户：看上架套餐、下单、看自己的订单；未登录返回 401。
- 管理员：增删改查套餐、看全部订单、确认线下收款（入账）、关闭未支付订单；非管理员 403。
- `payment_notifications`：回调按 `(provider, providerNotificationId)` 去重，重复回调不重复入账；入账失败保留未处理痕迹。
- 入账复用 `credits.Service.Recharge`，同一订单幂等键只产生一条不可变流水。

- [x] **Step 1: 写失败测试**：套餐隐藏下架、下单快照价格、停用商品拒单、确认入账只一次、重复确认幂等、普通用户被拒、关闭幂等、回调去重、未知订单 404、入账失败不改状态。
- [x] **Step 2: 实现模型、仓储、服务、路由与前端三页（充值 / 充值商品 / 支付订单）。**
- [x] **Step 3: 运行**：`go vet ./internal/payment/ ./internal/hostedapi/ ./internal/repository/ ./internal/database/ ./cmd/server/`；`go test ./internal/payment/ ./internal/hostedapi/`；`node --test test/format.test.js test/api-paths.test.js`。
- [x] **Step 4: 提交**：`feat(backend): 充值 - 新增商品订单与确认入账`。

**后续由 Task 8 完成**：支付宝电脑网站支付 / 微信 Native 的拉起支付参数生成、回调验签与解密。

**仍未做**：订单超时自动关闭、退款与对账。

## Task 8: 支付网关拉起与回调（本次完成）

**目标**：把「订单已创建」接到真实支付宝/微信支付，并让回调直接驱动幂等入账。

**Files:**
- Create: `backend/internal/payment/{gateway,crypto,alipay,wechat}.go`
- Create: `backend/internal/hostedapi/payment_gateway.go`
- Create: `backend/internal/payment/gateway_test.go`、`backend/internal/hostedapi/payment_gateway_test.go`
- Modify: `backend/internal/model/models_payment.go`（微信商户证书序列号）、`models_topup.go`（订单渠道配置、回调金额）
- Modify: `backend/internal/payment/{config,secrets,topup}.go`、`repository/payment.go`、`cmd/server/main.go`、`web-hosted/app.js`、`README.md`

**行为**
- 用户下单时快照启用的支付渠道配置 ID；后续后台改渠道不会串到历史订单。
- `POST /api/topup/orders/:id/pay`：
  - 支付宝 RSA2 生成电脑网站支付跳转链接；
  - 微信 APIv3 Native 下单，返回 `code_url`。
- `POST /api/payments/:provider/notify`：
  - 支付宝验 RSA2 签名、校验 `app_id`、金额、订单号；
  - 微信验 `Wechatpay-*` 签名，AES-256-GCM 解密回调，校验商户号、AppID、金额、订单号；
- 回调金额必须与订单快照金额一致；金额缺失或不一致不入账。
- 入账仍走积分账本的同一幂等键，重复回调/网关重推不重复加积分。
- 回调 HTTP 响应按网关契约返回：支付宝 `success`，微信 `{"code":"SUCCESS","message":"成功"}`。
- 前端下单后自动调用拉起支付：支付宝跳转，微信先弹出可复制的支付链接。

- [x] **Step 1: 写测试**：支付宝请求签名、回调验签与金额解析；微信 Native 请求签名、回调签名与密文解密；金额不一致拒绝入账；HTTP 回调原始请求体与响应契约。
- [x] **Step 2: 实现适配器、路由、订单渠道快照与前端拉起**。
- [x] **Step 3: 运行**：`go test ./internal/payment/ ./internal/hostedapi/`、`go vet`、托管迁移测试、桌面依赖边界测试、`node --test` 全通过。
- [x] **Step 4: 提交**：`feat(backend): 支付 - 接入支付宝与微信网关回调`。

**仍未做**：订单超时自动关闭、退款、对账、微信二维码图片渲染。

## 自检结论

- **Spec 覆盖**：覆盖设计文档第 3 节（角色与分层）、第 4 节（积分体系）、第 7 节第 2/3 类（积分与用户后台）、第 8 节第 1/2 条（认证与租户隔离）。
- **不在本计划内**：第 8 节第 3 条（对象存储）已在 Plan 2/3 交付；第 8 节第 4 条（外置任务队列）与 P2 支付、P3 技能各单独立项。
- **边界守卫**：Task 1 的 auth 包只被 `cmd/server`/`hostedapi` 引用；托管身份模型不进 `models_identity.go`；托管表只进 `HostedModels()`。
