#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
    echo "Usage: sh init-env.sh manju.example.com" >&2
    exit 1
fi

DOMAIN="$1"
case "$DOMAIN" in
    *://*|*/*|*\ *)
        echo "Use a bare domain name, without https://, a path, or spaces." >&2
        exit 1
        ;;
esac

if [ -e .env ]; then
    echo ".env already exists; refusing to overwrite it." >&2
    exit 1
fi

if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required to generate deployment secrets." >&2
    exit 1
fi

umask 077
POSTGRES_PASSWORD="$(openssl rand -hex 32)"
BOOTSTRAP_ADMIN="vsd_admin_$(openssl rand -hex 6)"

cat > .env <<EOF
VISIDEEP_DOMAIN=$DOMAIN
VISIDEEP_WEB_PORT=3100
VISIDEEP_ADMIN_PORT=3101
VISIDEEP_IMAGE_TAG=production
GOPROXY=https://goproxy.cn,direct
APK_MIRROR=https://mirrors.aliyun.com/alpine

CANVAS_POSTGRES_DB=visideep
CANVAS_POSTGRES_USER=visideep
CANVAS_POSTGRES_PASSWORD=$POSTGRES_PASSWORD
CANVAS_DATABASE_DRIVER=postgres
DATABASE_URL=postgres://visideep:$POSTGRES_PASSWORD@postgres:5432/visideep?sslmode=disable
CANVAS_AUTO_MIGRATE=true

CANVAS_BACKEND_ADDR=:8080
CANVAS_BACKEND_DATA_DIR=/data
BEEFTV_AGENT_OPS_URL=http://127.0.0.1:8080/api
CANVAS_SHUTDOWN_TIMEOUT=10m
CANVAS_WORKER_CONCURRENCY=3
CANVAS_CHANNEL_CONCURRENCY=3
CANVAS_ALLOW_PRIVATE_UPSTREAMS=false
GIN_MODE=release

CANVAS_CORS_ORIGINS=https://$DOMAIN
CANVAS_PUBLIC_BASE_URL=https://$DOMAIN
CANVAS_BOOTSTRAP_ADMIN_USERNAME=$BOOTSTRAP_ADMIN
EOF

chmod 600 .env
echo "Created .env with mode 600."
echo "Bootstrap admin username: $BOOTSTRAP_ADMIN"
echo "Register this username immediately after the backend starts."
