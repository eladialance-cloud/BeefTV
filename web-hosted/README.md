# BeefTV 托管工作台（网页版前端）

给 BeefTV 网页版（后端 `server` profile）用的独立前端：登录/注册、我的钱包与积分流水、
管理后台的用户积分、对象存储与支付渠道配置。

## 为什么单独一个目录

`web/src` 是本地桌面版前端，仓库里有一条硬边界：`web/test/local-only-source-boundary.test.ts`
明确要求 `src/pages/auth/login.tsx`、`src/services/api/wallet.ts`、`src/pages/admin/index.tsx`
这些托管入口**物理不存在**，这样桌面安装包里就不会混进 SaaS 控制面。

所以托管界面放在这里，不进 `web/src`、不进桌面构建。两边共用同一个后端 JSON API。

## 零依赖

只有一个 HTML、一个 CSS、一个 ES module，没有构建步骤，也没有 npm 依赖：

```
web-hosted/
├── index.html
├── styles.css
└── app.js
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

## 还没做

- 真实网关联调：支付宝电脑网站支付与微信 Native 已接入签名、验签和金额校验，但还需要拿商户真实密钥联调。
- 微信二维码目前先把 `code_url` 交给用户复制，后续可接二维码组件直接显示。
- 订单超时自动关闭、退款与对账。
- 模型价格规则：等 Task 5b 的价格表落地。
