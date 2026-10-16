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
- [ ] 补上管理员账号的回归：`CANVAS_BOOTSTRAP_ADMIN_USERNAME` 注册后能进后台，普通用户被 403。

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

## Task 5: 价格规则、钱包查询与管理员发积分

**Files:**
- Create: `backend/internal/credits/pricing.go`
- Create: `backend/internal/hostedapi/credits.go`
- Test: `backend/internal/hostedapi/credits_test.go`
- Modify: `backend/internal/database/hosted_schema.go`

**行为**

- `model_pricings` / `channel_model_price_tiers` / `logical_model_price_skus` 落库，管理员可改，无需发版。
- 用户端：`GET /api/credits/wallet`、`GET /api/credits/ledger`。
- 管理端：`GET /api/admin/credits/accounts`、`POST /api/admin/credits/adjust`（人工加减积分，必填原因，写审计流水）。
- 方法名避开守卫清单（不用 `Wallet` / `AdminAdjustCredits` 等）。

- [ ] **Step 1: 写失败测试**：非管理员调整返回 403；管理员调整后余额与流水一致；用户只能看自己的流水。
- [ ] **Step 2: 运行确认失败**。
- [ ] **Step 3: 实现**。
- [ ] **Step 4: 运行相关包测试通过**。
- [ ] **Step 5: 提交**：`feat(backend): 积分 - 新增价格规则、钱包查询与管理员发积分`。

## Task 6: 前端登录、钱包与后台积分页面

**Files:**
- Modify: `web/src/router.tsx`
- Create: `web/src/pages/LoginPage.tsx`、`web/src/pages/WalletPage.tsx`、`web/src/pages/AdminCreditsPage.tsx`

**行为**

- 未登录访问创作台重定向到登录页；登录后回到原页面。
- 钱包页展示余额、冻结额与流水分页。
- 后台积分页支持查询账户、调整积分、填写原因。

- [ ] **Step 1: 写失败测试**（登录守卫与钱包渲染）。
- [ ] **Step 2: 运行确认失败**。
- [ ] **Step 3: 实现**。
- [ ] **Step 4: 运行前端测试与构建通过**。
- [ ] **Step 5: 提交**：`feat(web): 多用户 - 新增登录、钱包与后台积分页面`。

## 自检结论

- **Spec 覆盖**：覆盖设计文档第 3 节（角色与分层）、第 4 节（积分体系）、第 7 节第 2/3 类（积分与用户后台）、第 8 节第 1/2 条（认证与租户隔离）。
- **不在本计划内**：第 8 节第 3 条（对象存储）已在 Plan 2/3 交付；第 8 节第 4 条（外置任务队列）与 P2 支付、P3 技能各单独立项。
- **边界守卫**：Task 1 的 auth 包只被 `cmd/server`/`hostedapi` 引用；托管身份模型不进 `models_identity.go`；托管表只进 `HostedModels()`。
