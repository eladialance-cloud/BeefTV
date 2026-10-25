# 跨时空婚礼包装与交付执行链 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成跨时空婚礼制作台最后两个真实阶段：包装制作与正式成片交付。

**Architecture:** 包装阶段把已确认视频、确定性标题卡和 BGM 组成 CanvasTimeline；交付阶段通过现有 `/timeline/renders` 服务端 FFmpeg 任务渲染 MP4，并把资源、QC 和工程清单写回 `weddingProduction`。继续复用现有时间线、任务和资源系统。

**Tech Stack:** React 19、TypeScript、CanvasTimeline、Go timeline render task、Bun test、Playwright。

**Spec:** `docs/superpowers/specs/2026-10-25-cross-era-wedding-dedicated-production-workbench-design.md`

## Global Constraints

- 包装预览和成片必须由真实选中视频版本构成。
- 标题卡必须保存为真实图片资源，不能只在页面显示文案。
- BGM 生成使用现有音频任务；上传使用现有媒体存储。
- 成片必须通过现有服务端 timeline render 任务生成 MP4。
- 成片确认前必须验证视频可加载且有时长。
- 工程导出必须包含提示词版本、seed、QC、资源 ID 和渲染任务 ID。
- 不伪造渲染进度、可播放状态或下载结果。

## File Structure

```text
web/src/lib/wedding/
  packaging-runtime.ts
  packaging-timeline.ts
  delivery-runtime.ts

web/src/services/
  wedding-packaging-generation.ts
  wedding-delivery.ts

web/src/pages/wedding/
  packaging-stage.tsx
  delivery-stage.tsx
  workbench.tsx
  wedding-workbench.css

web/test/
  wedding-packaging-runtime.test.ts
  wedding-packaging-timeline.test.ts
  wedding-packaging-stage.test.tsx
  wedding-delivery-runtime.test.ts
  wedding-delivery.test.ts
  wedding-delivery-stage.test.tsx
  wedding-workbench.browser.test.ts
```

---

### Task 1: 包装状态与时间线构建

**Files:**
- Create: `web/test/wedding-packaging-runtime.test.ts`
- Create: `web/test/wedding-packaging-timeline.test.ts`
- Create: `web/src/lib/wedding/packaging-runtime.ts`
- Create: `web/src/lib/wedding/packaging-timeline.ts`
- Modify: `web/src/lib/wedding/types.ts`
- Modify: `web/src/lib/wedding/production-state.ts`

**Interfaces:**
- Produces:
  - `updateWeddingPackaging(state, patch, now)`
  - `markWeddingPackagingReady(state, timeline, now)`
  - `confirmWeddingPackaging(state, now)`
  - `buildWeddingPackagingTimeline(input): TimelineProject`

- [x] **Step 1: Write failing state test**

```ts
const updated = updateWeddingPackaging(production, {
    intro: { text: "三生石上", durationSeconds: 4, enabled: true },
}, now);
expect(updated.packaging.intro.text).toBe("三生石上");
expect(updated.stages.delivery.status).toBe("stale");
```

- [x] **Step 2: Write failing timeline test**

```ts
expect(timeline.clips.map((clip) => clip.title)).toEqual([
    "片头标题",
    "唐 · 朝代视频",
    "明 · 朝代视频",
    "片尾标题",
]);
expect(timeline.tracks.map((track) => track.kind)).toContain("audio");
```

- [x] **Step 3: Verify RED**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-packaging-runtime.test.ts test/wedding-packaging-timeline.test.ts
```

Expected: FAIL because packaging modules do not exist.

- [x] **Step 4: Implement state and timeline**

Timeline construction uses `TimelineClip.directMedia` for title images, selected dynasty videos, and BGM. Video/images are sequential on the video track; BGM starts at zero and ends at total duration. Use `normalizeTimelineProject`.

- [x] **Step 5: Run tests and commit**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-packaging-runtime.test.ts test/wedding-packaging-timeline.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
git add web/src/lib/wedding/types.ts web/src/lib/wedding/production-state.ts web/src/lib/wedding/packaging-runtime.ts web/src/lib/wedding/packaging-timeline.ts web/test/wedding-packaging-runtime.test.ts web/test/wedding-packaging-timeline.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增包装时间线"
```

---

### Task 2: 标题卡和 BGM 真实资源

**Files:**
- Create: `web/src/services/wedding-packaging-generation.ts`
- Modify: `web/src/lib/wedding/packaging-runtime.ts`

**Interfaces:**
- Produces:
  - `generateWeddingTitleCards(runtime): Promise<{introNodeId;outroNodeId}>`
  - `generateWeddingBgm(runtime): Promise<{nodeId;assetId}>`
  - `uploadWeddingBgm(runtime, file): Promise<{nodeId;assetId}>`

- [ ] **Step 1: Add behavior tests around deterministic titles**

Test `buildWeddingTitleCardDocument` for intro/outro text and duration; test BGM request metadata contains theme, duration and `source:"wedding-workbench"`.

- [ ] **Step 2: Verify RED**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-packaging-runtime.test.ts
```

- [ ] **Step 3: Implement title card rasterization**

Use browser Canvas at the selected output dimensions, draw a dark cinematic background and the Chinese title text, call `uploadImage`, create `CanvasNodeData` image nodes, and persist both nodes and `titleCardNodeIds`.

- [ ] **Step 4: Implement generated/uploaded BGM**

Generated BGM uses `runBackendGenerationTask({ mode: "audio" })` and `consumeGenerationTaskNode`. Upload uses `uploadMediaFile(file, "audio")`. Persist `bgmNodeId` and the audio node.

- [ ] **Step 5: Run tests and commit**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-packaging-runtime.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
git add web/src/services/wedding-packaging-generation.ts web/src/lib/wedding/packaging-runtime.ts web/test/wedding-packaging-runtime.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 接入包装素材"
```

---

### Task 3: 包装制作工作区

**Files:**
- Create: `web/test/wedding-packaging-stage.test.tsx`
- Create: `web/src/pages/wedding/packaging-stage.tsx`
- Modify: `web/src/pages/wedding/workbench.tsx`
- Modify: `web/src/pages/wedding/wedding-workbench.css`
- Modify: `web/test/wedding-workbench.browser.test.ts`

**Interfaces:**
- Produces `PackagingStage` with intro/outro fields, BGM mode, title card previews, timeline strip, seekable video preview, build timeline action and stage confirmation.

- [ ] **Step 1: Write failing render test**

```ts
expect(html).toContain("片头文案");
expect(html).toContain("片尾文案");
expect(html).toContain("时间线预览");
expect(html).toContain("生成标题卡");
expect(html).toContain("确认包装阶段");
```

- [ ] **Step 2: Verify RED**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-packaging-stage.test.tsx
```

- [ ] **Step 3: Build packaging workspace**

The preview walks title card → selected videos in dynasty order → outro. Show BGM state, subtitle/transition controls and clip durations. Save settings before building timeline.

- [ ] **Step 4: Wire workbench actions**

Generate title cards, generate/upload BGM, build timeline from real assets, persist `project.timeline`, mark packaging ready, then allow explicit packaging approval.

- [ ] **Step 5: Verify and commit**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-packaging-stage.test.tsx test/wedding-workbench.browser.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
git add web/src/pages/wedding/packaging-stage.tsx web/src/pages/wedding/workbench.tsx web/src/pages/wedding/wedding-workbench.css web/test/wedding-packaging-stage.test.tsx web/test/wedding-workbench.browser.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增包装制作工作区"
```

---

### Task 4: 成片交付状态与服务端渲染

**Files:**
- Create: `web/test/wedding-delivery-runtime.test.ts`
- Create: `web/test/wedding-delivery.test.ts`
- Create: `web/src/lib/wedding/delivery-runtime.ts`
- Create: `web/src/services/wedding-delivery.ts`
- Modify: `web/src/lib/wedding/types.ts`
- Modify: `web/src/lib/wedding/production-state.ts`

**Interfaces:**
- Produces:
  - `WeddingDeliveryState`
  - `markWeddingDeliveryRendering(state, taskId, now)`
  - `recordWeddingDeliveryResult(state, result, now)`
  - `verifyWeddingDeliveryPlayable(state, metadata, now)`
  - `confirmWeddingDelivery(state, now)`
  - `renderWeddingDelivery(runtime): Promise<TimelineRenderResult>`
  - `weddingDeliveryManifest(production): string`

- [ ] **Step 1: Write failing delivery tests**

```ts
expect(rendering.delivery?.status).toBe("rendering");
expect(recordWeddingDeliveryResult(rendering, result, now).delivery).toMatchObject({
    status: "needs_review",
    resourceId: "resource:film",
});
expect(confirmWeddingDelivery(unverified, now)).toBe(unverified);
```

- [ ] **Step 2: Implement render through existing task**

Use `beginLocalExecutorSession`, `runOwnedTimelineRender`, and the current `project.timeline`. Persist task ID while rendering and result resource ID on completion.

- [ ] **Step 3: Implement playable verification**

`verifyWeddingDeliveryPlayable` requires positive `durationMs` and `videoWidth/videoHeight`. Persist `playableVerifiedAt`.

- [ ] **Step 4: Implement manifest**

Export JSON containing source photo nodes, dynasty video versions, prompt overrides, scene/video QC, title/BGM resources, render task ID and final resource ID.

- [ ] **Step 5: Verify and commit**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-delivery-runtime.test.ts test/wedding-delivery.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
git add web/src/lib/wedding/types.ts web/src/lib/wedding/production-state.ts web/src/lib/wedding/delivery-runtime.ts web/src/services/wedding-delivery.ts web/test/wedding-delivery-runtime.test.ts web/test/wedding-delivery.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 接入成片渲染"
```

---

### Task 5: 成片交付工作区

**Files:**
- Create: `web/test/wedding-delivery-stage.test.tsx`
- Create: `web/src/pages/wedding/delivery-stage.tsx`
- Modify: `web/src/pages/wedding/workbench.tsx`
- Modify: `web/src/pages/wedding/wedding-workbench.css`
- Modify: `web/test/wedding-workbench.browser.test.ts`

**Interfaces:**
- Produces `DeliveryStage` with render progress, MP4 player, playable verification, QC summary, download, manifest export and final confirmation.

- [ ] **Step 1: Write failing render test**

```ts
expect(html).toContain("正式成片");
expect(html).toContain("渲染成片");
expect(html).toContain("验证可播放");
expect(html).toContain("下载 MP4");
expect(html).toContain("导出工程清单");
```

- [ ] **Step 2: Implement UI**

Use `resourceFileUrl(resourceId)` for preview/download. Require `playableVerifiedAt` before final confirmation. Keep partial timeline and task IDs visible on render failure.

- [ ] **Step 3: Wire workbench**

Submit render, subscribe task progress, verify video metadata from the rendered resource, persist delivery state, download MP4 and export manifest.

- [ ] **Step 4: Verify and commit**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-delivery-stage.test.tsx test/wedding-workbench.browser.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
git add web/src/pages/wedding/delivery-stage.tsx web/src/pages/wedding/workbench.tsx web/src/pages/wedding/wedding-workbench.css web/test/wedding-delivery-stage.test.tsx web/test/wedding-workbench.browser.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增成片交付工作区"
```

---

### Task 6: 全流程验收

- [ ] Run all `wedding-*.test.*` files.
- [ ] Run `bun run typecheck`, `bun run lint` and `bun run build`.
- [ ] Run Playwright at `1440x960` and `390x900`; verify all seven stages are reachable and have no horizontal overflow.
- [ ] Update the spec status from “待实施” to “七阶段完整执行链已完成”.
- [ ] Commit verification docs.
