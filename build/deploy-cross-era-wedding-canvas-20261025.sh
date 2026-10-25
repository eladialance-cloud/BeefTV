#!/usr/bin/env bash
set -euo pipefail

archive="${1:?archive path is required}"
expected_hash="${2:?archive hash is required}"
repo="${3:-/opt/visideep-manju}"

actual_hash="$(sha256sum "$archive" | cut -d' ' -f1)"
if [[ "$actual_hash" != "$expected_hash" ]]; then
  echo "ARCHIVE_HASH_MISMATCH expected=$expected_hash actual=$actual_hash"
  exit 1
fi

cd "$repo"
stamp="$(date +%Y%m%d-%H%M%S)"
backup="backups/cross-era-wedding-canvas-$stamp"
mkdir -p "$backup/files"

files=(
  backend/internal/app/creation_prompt.go
  backend/internal/hostedapi/creation_catalog.go
  scripts/build-cross-era-wedding-workflow-json.mjs
  build/cross-era-wedding-volc.workflow.v1.json
  build/upsert-cross-era-wedding-workflow.sql
  web/src/lib/wedding
  web/src/pages/wedding
  web/src/components/canvas/infinite-canvas.tsx
  web/src/pages/canvas/canvas-project-top-bar.tsx
  web/src/pages/canvas/project.tsx
  web/src/pages/home/index.tsx
  web/src/lib/workspace-route-modules.ts
  web/src/router.tsx
  web/src/stores/canvas/use-canvas-store.ts
  web/src/services/local-workspace-repository.ts
  web/src/services/local-workspace-sync.ts
  web/src/services/workspace-project-repository.ts
  web/src/services/workflow-template-canvas.ts
  web/src/services/api/creation-templates.ts
)

for file in "${files[@]}"; do
  if [[ -e "$file" ]]; then
    cp --parents -a "$file" "$backup/files/"
  fi
done

cd "$repo/deploy/visideep"
set -a
. ./.env
set +a

sudo docker compose exec -T postgres pg_dump \
  -U "$CANVAS_POSTGRES_USER" -d "$CANVAS_POSTGRES_DB" \
  > "$repo/$backup/database.sql"

cd "$repo"
tar -xzf "$archive" -C "$repo"
cp build/cross-era-wedding-volc.workflow.v1.json /tmp/cross-era-wedding-volc.workflow.v1.json

cd "$repo/deploy/visideep"
sudo docker compose cp \
  /tmp/cross-era-wedding-volc.workflow.v1.json \
  postgres:/tmp/cross-era-wedding-volc.workflow.v1.json

sudo -E docker compose exec -T postgres psql -v ON_ERROR_STOP=1 \
  -U "$CANVAS_POSTGRES_USER" -d "$CANVAS_POSTGRES_DB" \
  -f /dev/stdin < "$repo/build/upsert-cross-era-wedding-workflow.sql"

sudo -E docker compose exec -T postgres psql -v ON_ERROR_STOP=1 \
  -U "$CANVAS_POSTGRES_USER" -d "$CANVAS_POSTGRES_DB" \
  -c "UPDATE creation_catalog_items SET enabled = true, updated_at = now() WHERE kind = 'workflow' AND item_key = 'cross-era-wedding-volc';"

override_ready="$(sudo -E docker compose exec -T postgres psql \
  -U "$CANVAS_POSTGRES_USER" -d "$CANVAS_POSTGRES_DB" -Atqc "
SELECT strpos(payload_json, '\"allowProducerPromptOverride\": true') > 0
FROM creation_catalog_items
WHERE kind = 'workflow'
  AND item_key = 'cross-era-wedding-volc'
")"
if [[ "$override_ready" != "t" ]]; then
  echo "WORKFLOW_OVERRIDE_FLAG=FAIL value=$override_ready"
  sudo -E docker compose exec -T postgres psql \
    -U "$CANVAS_POSTGRES_USER" -d "$CANVAS_POSTGRES_DB" -P pager=off -c "
SELECT jsonb_pretty(
  jsonb_path_query_first(
    payload_json::jsonb,
    '\$.canvas.dynamicNodes[*] ? (@.ref == \"dynastyVideo\")'
  )
)
FROM creation_catalog_items
WHERE kind = 'workflow'
  AND item_key = 'cross-era-wedding-volc';
"
  exit 1
fi

sudo docker compose build backend web
sudo docker compose up -d --no-deps --force-recreate backend

backend_ready=0
for _ in $(seq 1 90); do
  status="$(sudo docker inspect visideep-backend-1 --format '{{.State.Health.Status}}' 2>/dev/null || true)"
  if [[ "$status" == "healthy" ]]; then
    backend_ready=1
    break
  fi
  sleep 2
done
if [[ "$backend_ready" != "1" ]]; then
  sudo docker compose logs --tail=240 backend
  exit 1
fi

sudo docker compose up -d --no-deps --force-recreate web

web_ready=0
for _ in $(seq 1 90); do
  status="$(sudo docker inspect visideep-web-1 --format '{{.State.Health.Status}}' 2>/dev/null || true)"
  if [[ "$status" == "healthy" ]]; then
    web_ready=1
    break
  fi
  sleep 2
done
if [[ "$web_ready" != "1" ]]; then
  sudo docker compose logs --tail=240 web
  exit 1
fi

web_address="$(sudo docker compose port web 3000)"
web_port="${web_address##*:}"
curl --retry 20 --retry-connrefused --retry-delay 2 -fsS \
  "http://127.0.0.1:${web_port}/api/health/ready" >/dev/null
curl --retry 20 --retry-connrefused --retry-delay 2 -fsS \
  "http://127.0.0.1:${web_port}/api/creation-templates?kind=workflow" |
  grep -q '跨时空婚礼电影'

sudo docker run --rm --entrypoint /bin/sh visideep-web:production -c \
  "grep -R -q 'canvasView=wedding' /usr/share/nginx/html/static"

cd "$repo/deploy/visideep"
sudo docker compose ps backend web

echo "ARCHIVE_HASH=PASS"
echo "WORKFLOW_TEMPLATE=PASS"
echo "WORKFLOW_OVERRIDE_FLAG=PASS"
echo "FRONTEND_CANVAS_MODE=PASS"
echo "BACKEND_HEALTH=PASS"
echo "WEB_HEALTH=PASS"
echo "BACKUP=$repo/$backup"
echo "DEPLOY=PASS"
