import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const source = readFileSync(resolve(here, "..", "app.js"), "utf8");

test("each channel model exposes a real test action", () => {
    assert.match(source, /data-action="test-channel-model"/u);
    assert.match(source, /\/admin\/channels\/\$\{encodeURIComponent\(channelId\)\}\/models\/test/u);
    assert.match(source, /测试中/u);
    assert.match(source, /会向当前上游发起真实生成请求/u);
});

test("channel model editor exposes ToAPI media protocols and size controls", () => {
    assert.match(source, /\['toapis-image', 'toapis-image（图片）'\]/u);
    assert.match(source, /\['toapis-video', 'toapis-video（视频）'\]/u);
    assert.match(source, /id="channel-model-image-params"/u);
    assert.match(source, /id="channel-model-video-params"/u);
    assert.match(source, /name="imageRatios"/u);
    assert.match(source, /name="imageResolutions"/u);
    assert.match(source, /name="videoRatios"/u);
    assert.match(source, /name="videoResolutions"/u);
    assert.match(source, /capabilityConfig: channelModelCapabilityConfigFromForm/u);
});
