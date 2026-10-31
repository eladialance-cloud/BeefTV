import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { normalizeRmbCost } from "../format.js";

const here = dirname(fileURLToPath(import.meta.url));
const source = readFileSync(resolve(here, "..", "app.js"), "utf8");
const indexSource = readFileSync(resolve(here, "..", "index.html"), "utf8");

test("pricing editor keeps only the essential price controls", () => {
    assert.match(source, /id="pricing-editor-billing-mode"/u);
    assert.match(source, /class="pricing-editor-primary"/u);
    assert.match(source, /id="pricing-editor-charge-summary"/u);
    assert.match(source, /<details class="pricing-editor-converter" data-rmb-converter>/u);
    assert.match(source, /pricingEditorChargeText/u);
    assert.match(source, /name="rmbQuoteUnit"/u);
    assert.doesNotMatch(source, /class="pricing-editor-context"/u);
});

test("pricing editor supports resolution-specific channel prices", () => {
    assert.match(source, /class="pricing-resolution-picker"/u);
    assert.match(source, /data-pricing-editor-row/u);
    assert.match(source, /name="pricingEnabled"/u);
    assert.match(source, /name="resolutionPrice"/u);
    assert.match(source, /name="minimumChargeCredits" type="number" min="0" step="0\.000001"/u);
    assert.match(source, /minimumChargeMicrocredits/u);
    assert.match(source, /勾选多个档位/u);
    assert.doesNotMatch(source, /id="pricing-editor-resolution"/u);
});

test("per-million-token RMB prices normalize without manual division", () => {
    assert.equal(normalizeRmbCost(2, "per_1k_tokens", "per_million_tokens"), 0.002);
    assert.equal(normalizeRmbCost(0.002, "per_1k_tokens", "per_1k_tokens"), 0.002);
    assert.equal(normalizeRmbCost(3, "fixed_request", "per_unit"), 3);
});

test("admin modules use cache-busting versions", () => {
    assert.match(source, /from "\.\/format\.js\?v=/u);
    assert.match(indexSource, /src="\.\/app\.js\?v=/u);
});
