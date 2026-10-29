# VisiDeep AI 管理后台（网页版前端）

给 VisiDeep AI 网页版（后端 `server` profile）用的管理后台前端，只服务管理员。
用户端（注册登录、项目库、画布、积分充值）是另一个应用，代码在 `../web-user/`，
两边共用同一个后端 JSON API。

登录页是左右分栏的深色版式：左侧整屏主视觉 + 品牌标语，右侧登录表单，
小屏下自动收起左侧图，只留表单。用普通用户账号登录会停在「没有管理后台权限」页，
不会看到任何后台数据。

导航按七个分组排列：

| 分组 | 页面 | 状态 |
| --- | --- | --- |
| 概览 | 运营概览 | 已接真实数据 |
| 用户 | 用户管理、注册策略与邀请码、积分账户 | 均已接通 |
| 内容 | 项目管理、素材库 | 均已接通 |
| 创作模板 | 工作流模板、提示词库、画风预设 | 均已接通 |
| 模型 | 模型渠道、价格规则 | 均已接通 |
| 商业化 | 套餐与积分包、订单、充值流水、支付渠道 | 均已接通 |
| 系统 | 站点配置、对象存储、审计日志 | 均已接通 |

所有导航页都由真实接口驱动，不再保留 `modules.js` 占位渲染器。
用户、项目、素材、站点配置和内容治理写操作都会写入审计日志。

## 为什么单独一个目录

`web/src` 是本地桌面版前端，仓库里有一条硬边界：`web/test/local-only-source-boundary.test.ts`
明确要求 `src/pages/auth/login.tsx`、`src/services/api/wallet.ts`、`src/pages/admin/index.tsx`
这些托管入口**物理不存在**，这样桌面安装包里就不会混进 SaaS 控制面。

所以托管界面放在这里，不进 `web/src`、不进桌面构建。两边共用同一个后端 JSON API。

## 零依赖

只有一个 HTML、一个 CSS、两个 ES module，没有构建步骤，也没有运行时 npm 依赖：

```
web-hosted/
├── index.html
├── styles.css
├── app.js          # 路由、页面渲染、数据加载与表单处理
├── format.js       # 纯格式化函数，带单测
├── assets/         # VisiDeep AI 品牌与登录主视觉
└── test/           # API 契约与格式化单测
```

## 怎么跑

后端（server profile）：

```powershell
cd backend
$env:CANVAS_DATABASE_DRIVER = "postgres"
$env:DATABASE_URL = "postgres://user:pass@host:5432/beeftv?sslmode=disable"
$env:CANVAS_BOOTSTRAP_ADMIN_USERNAME = "root"   # 用这个登录名注册的第一人即管理员
go run ./cmd/server
```

前端：任意静态服务器托管本目录即可，例如

```powershell
cd web-hosted
python -m http.server 4173
```

然后打开 `http://localhost:4173/`。

## 后端地址

默认请求同源 `/api`。要指向别的后端，二选一：

1. 在 `index.html` 里、`app.js` 之前插入一行设置 `window.__BEEFTV_API_BASE__`；
2. 打开页面后执行 `localStorage.setItem("beeftv.apiBase", "https://api.example.com/api")` 并刷新。

跨域时后端需要放行该来源：设置 `CANVAS_CORS_ORIGINS`。登录 cookie 是
`HttpOnly + SameSite=Lax`，跨站部署请用同站子域或反向代理把前后端放到同一站点下。

## 已接通的接口

| 界面 | 方法 | 路径 |
| --- | --- | --- |
| 登录 | POST | `/api/login` |
| 注册 | POST | `/api/register` |
| 当前用户 | GET | `/api/auth/me` |
| 退出 | POST | `/api/logout` |
| 运营概览（管理员） | GET | `/api/admin/overview?days=7` |
| 用户列表（管理员） | GET | `/api/admin/users` |
| 新增用户（管理员） | POST | `/api/admin/users` |
| 修改用户状态/角色（管理员） | PATCH | `/api/admin/users/:id` |
| 重置用户密码（管理员） | POST | `/api/admin/users/:id/password` |
| 项目/画布列表（管理员） | GET | `/api/admin/projects` |
| 归档/恢复项目（管理员） | POST | `/api/admin/projects/:id/archive\|restore` |
| 素材列表（管理员） | GET | `/api/admin/assets` |
| 下架/恢复素材（管理员） | POST | `/api/admin/assets/:id/block\|unblock` |
| 积分流水（管理员） | GET | `/api/admin/credit-ledger` |
| 站点配置（管理员） | GET/PUT | `/api/admin/site` |
| 公开站点配置 | GET | `/api/site/config` |
| 审计日志（管理员） | GET | `/api/admin/audit` |
| 我的钱包 | GET | `/api/credits/wallet` |
| 我的流水 | GET | `/api/credits/ledger` |
| 账户列表（管理员） | GET | `/api/admin/credits/accounts` |
| 调整积分（管理员） | POST | `/api/admin/credits/adjust` |
| 存储列表（管理员） | GET | `/api/admin/storage-locations` |
| 新增存储（管理员） | POST | `/api/admin/storage-locations` |
| 连通性测试（管理员） | POST | `/api/admin/storage-locations/:id/test` |
| 设为默认（管理员） | POST | `/api/admin/storage-locations/:id/default` |
| 删除存储（管理员） | DELETE | `/api/admin/storage-locations/:id` |
| 支付渠道列表（管理员） | GET | `/api/admin/payment-providers` |
| 新增支付渠道（管理员） | POST | `/api/admin/payment-providers` |
| 修改支付渠道（管理员） | PUT | `/api/admin/payment-providers/:id` |
| 删除支付渠道（管理员） | DELETE | `/api/admin/payment-providers/:id` |
| 价格规则列表（管理员） | GET | `/api/admin/pricing/:kind` |
| 新增价格规则（管理员） | POST | `/api/admin/pricing/:kind` |
| 修改价格规则（管理员） | PUT | `/api/admin/pricing/:kind/:id` |
| 删除价格规则（管理员） | DELETE | `/api/admin/pricing/:kind/:id` |
| 充值套餐（用户） | GET | `/api/topup/products` |
| 我的充值订单 | GET | `/api/topup/orders` |
| 创建充值订单 | POST | `/api/topup/orders` |
| 拉起支付 | POST | `/api/topup/orders/:id/pay` |
| 支付回调 | POST | `/api/payments/:provider/notify` |
| 充值商品（管理员） | GET/POST | `/api/admin/topup-products` |
| 修改/删除商品（管理员） | PUT/DELETE | `/api/admin/topup-products/:id` |
| 支付订单（管理员） | GET | `/api/admin/payment-orders` |
| 确认支付（管理员） | POST | `/api/admin/payment-orders/:id/mark-paid` |
| 关闭订单（管理员） | POST | `/api/admin/payment-orders/:id/close` |
| 对账概览（管理员） | GET | `/api/admin/payment-orders/reconciliation` |
| 关闭超时订单（管理员） | POST | `/api/admin/payment-orders/expire` |
| 记录退款（管理员） | POST | `/api/admin/payment-orders/:id/refund` |

价格规则的 `:kind` 取 `model`（模型默认价）、`channel_tier`（渠道模型价格档）或 `logical_sku`（逻辑模型 SKU）。运行时报价优先级为：逻辑模型 SKU → 渠道模型价格档 → 模型默认价。单价以微积分存储，`1 积分 = 1,000,000 微积分`；计费方式支持每次请求、每秒、每张图、每千 token、每 MB。

任务提交时按 `capability` 匹配规则：文本任务用 `text`，图片任务用 `image`，视频任务用 `video`，音频任务用 `audio`；时间线转写/渲染、深度捕捉属于本地执行器，用 `local`，没配规则时不收费。
没有命中的托管模型任务会被拒绝提交（失败关闭），因此上线前必须为每个实际使用的 `capability` 配一条 `fixed_request` 或用量单价规则。

## 上线前检查

- 支付宝电脑网站支付与微信 Native 已接入签名、验签和金额校验，上线前仍需使用商户真实密钥完成支付与回调联调。
- 微信 Native 当前先展示 `code_url`，可按部署需要增加二维码展示组件。
- 生产环境使用 PostgreSQL，并完成数据库备份、迁移回滚和最小权限账号配置。
- 为对象存储配置正式凭据、CDN/CORS 和默认存储位置，并执行一次上传、下载、删除测试。
- 管理后台必须通过 HTTPS 反向代理部署，限制 `/api/admin/*` 来源并保留登录 Cookie 的同站配置。
- 接入运行日志、异常告警、数据库备份和积分对账任务。

## 当前计费行为

- 任务提交采用三段式计费：提交前按价格规则报价并写 `task_cost_charges` 快照，余额不足直接拒绝提交，成功后结算，失败或取消时释放冻结。
- 结算读取 `api_call_logs` 中该任务的成功调用，按真实 token、图片张数或视频秒数重算并释放多余冻结；没有可用回执时回退到预估额，真实用量超过冻结额时按冻结额封顶。
- 本地执行器（时间线转写/渲染、深度捕捉）和自定义渠道任务暂不收费；托管模型任务没有任何匹配价格规则时会失败关闭。
