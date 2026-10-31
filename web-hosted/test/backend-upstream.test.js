import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const nginx = readFileSync(resolve(here, "..", "..", "nginx.conf"), "utf8");

test("public gateway re-resolves the backend after container recreation", () => {
    assert.match(nginx, /resolver 127\.0\.0\.11 valid=10s ipv6=off;/u);
    assert.match(nginx, /upstream backend_upstream \{/u);
    assert.match(nginx, /server backend:8080 resolve;/u);
    assert.match(nginx, /proxy_pass http:\/\/backend_upstream;/u);
    assert.doesNotMatch(nginx, /proxy_pass http:\/\/backend:8080;/u);
});

test("SPA entry HTML is never cached across deployments", () => {
    assert.match(nginx, /location = \/index\.html \{/u);
    assert.match(nginx, /add_header Cache-Control "no-store, no-cache, must-revalidate" always;/u);
    assert.match(nginx, /expires -1;/u);
});
