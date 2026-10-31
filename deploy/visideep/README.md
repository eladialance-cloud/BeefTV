# VisiDeep AI 同机生产部署

这套目录把 VisiDeep AI 作为独立 Docker Compose 项目运行，默认只把用户端和管理后台绑定到服务器回环地址，公网入口继续交给服务器现有的 Nginx 或 Caddy。

## 组件

- `web`：正式用户端，来自仓库根目录 `Dockerfile` 和 `web/`。
- `admin`：管理后台静态站，挂载在同域 `/admin/`。
- `portal`：轻量交互式用户门户（`portal/`，含阶段推进交互与文件中心），挂载在同域 `/portal/`。
- `backend`：Go 服务端，不发布宿主机端口。
- `postgres`：独立 PostgreSQL，不发布宿主机端口。
- 持久卷：`visideep_postgres_data`、`visideep_backend_data`。

默认宿主机端口：

- `127.0.0.1:3100`：用户端。
- `127.0.0.1:3101`：管理后台。
- `127.0.0.1:3102`：交互式用户门户。

## 初始化

在服务器仓库根目录执行：

```bash
cd deploy/visideep
sh init-env.sh manju.example.com
docker compose config
docker compose build
docker compose up -d
docker compose ps
```

检查服务：

```bash
curl -f http://127.0.0.1:3100/api/health/ready
curl -f http://127.0.0.1:3101/healthz
```

首次启动后立即打开 `https://你的域名/login`，使用 `.env` 里的 `CANVAS_BOOTSTRAP_ADMIN_USERNAME` 注册管理员。注册完成后进入 `/admin/`，把注册策略改为关闭或邀请码。

## 反向代理

`visideep.nginx.conf.template` 是宿主机 Nginx 样例。替换三个占位符后放到现有 Nginx 配置目录，先检查再重载：

```bash
sudo nginx -t
sudo systemctl reload nginx
```

HTTPS 证书应继续使用服务器现有的 Certbot、Caddy 或证书管理方式，不要在这套 Compose 内抢占 `80/443`。

## 备份

数据库和 `/data` 必须一起备份。`/data/.settings-key` 丢失后，已保存的模型、支付和对象存储密钥可能无法解密。

```bash
mkdir -p backups
docker compose exec -T postgres pg_dump -U visideep -d visideep > "backups/visideep-$(date +%F-%H%M%S).sql"
docker run --rm -v visideep_backend_data:/data -v "$PWD/backups":/backup alpine \
  tar czf "/backup/visideep-data-$(date +%F-%H%M%S).tar.gz" -C /data .
```

## 升级

```bash
git pull --ff-only
docker compose build
docker compose up -d --remove-orphans
docker compose ps
```

不要执行 `docker compose down -v`，它会删除数据库和持久数据卷。
