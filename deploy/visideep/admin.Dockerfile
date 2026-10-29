FROM nginx:1.27-alpine

COPY deploy/visideep/admin-nginx.conf /etc/nginx/conf.d/default.conf
COPY web-hosted/ /usr/share/nginx/html/admin/

EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1/healthz >/dev/null || exit 1
