# 跨时空婚礼场景与视频执行链 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在独立婚礼制作台内交付真实可执行的场景首帧、自动 QC、视频生成、版本选择与逐朝代审核流程。

**Architecture:** 场景和视频继续映射到画布 `dynastyScene`、`dynastyVideo` 节点，通过 `runBackendGenerationTask`、任务订阅、统一资产物化和 `persistCanvasDocument` 写回。`weddingProduction` 只保存版本引用、QC 和审核状态，不复制任务系统。

**Tech Stack:** React 19、TypeScript、TanStack Query、Zustand、Bun test、Playwright、现有 Go 任务 API。

**Spec:** `docs/superpowers/specs/2026-10-25-cross-era-wedding-dedicated-production-workbench-design.md`

## Global Constraints

- `/wedding/:canvasId` 是唯一正常制作入口。
- 不展示模拟状态，不把页面摘要当作生成结果。
- 单个朝代失败不阻塞其他朝代。
- 每个阶段批量生成、逐朝代审核、人工确认后推进。
- 场景必须引用已确认的男女定妆照。
- 视频必须引用已确认的男女定妆照和场景首帧。
- 视频提示词覆盖必须通过 `creationTemplate.promptOverride` 进入服务端任务。
- 不复制新的任务、素材或模型调用系统。
- 所有新增生产行为先写失败测试，再写最小实现。

## File Structure

```text
web/src/lib/wedding/
  generation-runtime.ts
  scene-runtime.ts
  scene-qc.ts
  video-runtime.ts

web/src/services/
  wedding-scene-generation.ts
  wedding-video-generation.ts

web/src/pages/wedding/
  scene-stage.tsx
  video-stage.tsx
  wedding-workbench.css
  workbench.tsx

web/test/
  wedding-scene-runtime.test.ts
  wedding-scene-generation.test.ts
  wedding-scene-qc.test.ts
  wedding-scene-stage.test.tsx
  wedding-video-runtime.test.ts
  wedding-video-generation.test.ts
  wedding-video-stage.test.tsx
  wedding-workbench.browser.test.ts
```

---

### Task 1: 场景状态机和精确审核门控

**Files:**
- Create: `web/test/wedding-scene-runtime.test.ts`
- Create: `web/src/lib/wedding/scene-runtime.ts`
- Modify: `web/src/lib/wedding/types.ts`
- Modify: `web/src/lib/wedding/production-state.ts`

**Interfaces:**
- Produces:
  - `WeddingSceneVersion = { assetId; nodeId; storageKey?; previewUrl?; taskId?; createdAt }`
  - `weddingSceneJobs(production, dynastyId?): WeddingSceneJob[]`
  - `markWeddingSceneGenerating(state, job, now)`
  - `recordWeddingSceneVersion(state, job, version, now)`
  - `selectWeddingSceneVersion(state, dynastyId, assetId, now)`
  - `confirmWeddingScene(state, dynastyId, assetId, now)`
  - `failWeddingSceneGeneration(state, job, reason, now)`

- [x] **Step 1: Write failing scene runtime tests**

```ts
test("maps one scene job per dynasty", () => {
    expect(weddingSceneJobs(production)).toEqual([
        { nodeId: "scene-tang", dynastyId: "tang", kind: "scene" },
        { nodeId: "scene-ming", dynastyId: "ming", kind: "scene" },
    ]);
});

test("confirming a scene clears only that dynasty scene blocker", () => {
    const confirmed = confirmWeddingScene(state, "tang", "scene-asset", now);
    expect(confirmed.dynasties[0]?.sceneSelection).toBe("scene-asset");
    expect(weddingStageBlockers(confirmed, "scenes")).toContain("明：场景首帧尚未确认");
});
```

- [x] **Step 2: Run the test and verify RED**

Run:

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-scene-runtime.test.ts
```

Expected: FAIL because `scene-runtime.ts` does not exist.

- [x] **Step 3: Add scene state**

Add to `WeddingDynastyState`:

```ts
sceneVersions?: WeddingSceneVersion[];
```

`confirmWeddingScene` must set `sceneSelection`, set `statuses.scene = "ready"`, invalidate `statuses.video`, and mark `videos`, `packaging`, and `delivery` stale.

Change `weddingStageBlockers(state, "scenes")` to require:

```ts
const qcReady = dynasty.qc.scene?.verdict === "pass"
    || Boolean(dynasty.qc.overrides.scene?.reason.trim());
if (!dynasty.sceneSelection) blockers.push(`${dynasty.name}：场景首帧尚未确认`);
if (!qcReady) blockers.push(`${dynasty.name}：场景 QC 未通过且没有覆盖记录`);
```

- [x] **Step 4: Run tests and typecheck**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-scene-runtime.test.ts test/wedding-production-state.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
```

- [x] **Step 5: Commit**

```powershell
cd D:\画布\BeefTV
git add web/src/lib/wedding/types.ts web/src/lib/wedding/production-state.ts web/src/lib/wedding/scene-runtime.ts web/test/wedding-scene-runtime.test.ts web/test/wedding-production-state.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增场景状态机"
```

---

### Task 2: 场景真实生成任务

**Files:**
- Create: `web/test/wedding-scene-generation.test.ts`
- Create: `web/src/services/wedding-scene-generation.ts`
- Modify: `web/src/lib/wedding/scene-runtime.ts`

**Interfaces:**
- Consumes:
  - `weddingSceneJobs`
  - 已确认的 `portraitVersionHistory.male/female`
- Produces:
  - `WeddingGenerationSummary`, moved from the portrait-only service into `generation-runtime.ts`
  - `generateWeddingScenes(runtime, dynastyId?, options?): Promise<WeddingGenerationSummary>`
  - `WeddingSceneGenerationRuntime = { projectId; production; config: AiConfig; signal?; onTaskUpdate? }`

- [x] **Step 1: Write failing task-boundary test**

```ts
expect(submitted).toMatchObject({
    projectId: "canvas-1",
    mode: "image",
    prompt: "唐 · 双人婚礼场景",
    referenceImages: [
        { id: "male-asset", storageKey: "resource:male" },
        { id: "female-asset", storageKey: "resource:female" },
    ],
    metadata: {
        source: "wedding-workbench",
        stage: "scenes",
        dynastyId: "tang",
        creationTemplate: {
            nodeRef: "dynastyScene",
            dynastyId: "tang",
        },
    },
});
```

- [x] **Step 2: Verify RED**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-scene-generation.test.ts
```

Expected: FAIL because `wedding-scene-generation.ts` does not exist.

- [x] **Step 3: Implement generation**

Map selected portrait asset IDs to `ReferenceImage` values. Reject generation when either portrait is missing:

```ts
if (!male || !female) throw new Error("请先确认男女定妆照");
```

Run one `canvas_image` task per dynasty with bounded concurrency `2`. Use `consumeGenerationTaskNode`, `applyGeneratedMediaResultMetadata`, `imageMetadata`, and `recordWeddingSceneVersion`, exactly following `wedding-generation.ts`.

Move the existing `WeddingPortraitGenerationSummary` shape into `web/src/lib/wedding/generation-runtime.ts` as `WeddingGenerationSummary`; keep a portrait alias so existing portrait callers continue to compile.

- [x] **Step 4: Verify GREEN**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-scene-runtime.test.ts test/wedding-scene-generation.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
```

- [x] **Step 5: Commit**

```powershell
git add web/src/lib/wedding/generation-runtime.ts web/src/services/wedding-scene-generation.ts web/src/services/wedding-generation.ts web/src/lib/wedding/scene-runtime.ts web/test/wedding-scene-generation.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 接入场景生成"
```

---

### Task 3: 场景自动 QC 与人工覆盖

**Files:**
- Create: `web/test/wedding-scene-qc.test.ts`
- Create: `web/src/lib/wedding/scene-qc.ts`
- Modify: `web/src/services/wedding-scene-generation.ts`

**Interfaces:**
- Produces:
  - `parseWeddingSceneQc(text, checkedAt): WeddingSceneQc`
  - `sceneQcPrompt(dynastyName): string`
  - `applyWeddingSceneQc(state, dynastyId, qc, now)`
  - `overrideWeddingSceneQc(state, dynastyId, reason, now)`

- [x] **Step 1: Write failing parser and override tests**

```ts
test("parses a strict scene QC payload", () => {
    expect(parseWeddingSceneQc(JSON.stringify({
        verdict: "fail",
        couplePresent: true,
        faceMatchMale: false,
        faceMatchFemale: true,
        outfitMatch: true,
        sceneMatch: true,
        extraPerson: false,
        watermarkOrText: false,
        reason: "新郎脸部漂移",
        suggestion: "提高男女肖像参考权重",
    }), now)).toMatchObject({ verdict: "fail", faceMatchMale: false });
});

test("scene override requires a concrete reason", () => {
    expect(overrideWeddingSceneQc(state, "tang", "", now)).toBe(state);
});
```

- [x] **Step 2: Verify RED**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-scene-qc.test.ts
```

Expected: FAIL because the module is missing.

- [x] **Step 3: Implement QC**

Build a JSON-only prompt containing the dynastic scene, selected portraits, and required boolean fields. Run a `mode: "text"` backend task with all three reference images and metadata:

```ts
{
    source: "wedding-workbench",
    stage: "scene-qc",
    dynastyId,
    responseFormat: "json",
}
```

Persist `qc.scene` with `applyWeddingSceneQc`. The UI must show parser/model errors as `verdict: "error"` and require an explicit override reason; never translate an error into pass.

- [x] **Step 4: Verify GREEN**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-scene-qc.test.ts test/wedding-scene-generation.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
```

- [x] **Step 5: Commit**

```powershell
git add web/src/lib/wedding/scene-qc.ts web/src/services/wedding-scene-generation.ts web/test/wedding-scene-qc.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增场景 QC"
```

---

### Task 4: 场景首帧工作区

**Files:**
- Create: `web/test/wedding-scene-stage.test.tsx`
- Create: `web/src/pages/wedding/scene-stage.tsx`
- Modify: `web/src/pages/wedding/workbench.tsx`
- Modify: `web/src/pages/wedding/wedding-workbench.css`
- Modify: `web/test/wedding-workbench.browser.test.ts`

**Interfaces:**
- Consumes all scene runtime and generation actions.
- Produces `SceneStage` with:

```ts
{
    production: WeddingProductionState;
    nodes: CanvasNodeData[];
    tasks: Record<string, GenerationTask>;
    saving: boolean;
    feedback: string;
    onGenerateAll(): Promise<void>;
    onGenerateDynasty(dynastyId: string): Promise<void>;
    onRunQc(dynastyId: string): Promise<void>;
    onSelectVersion(dynastyId: string, assetId: string): Promise<void>;
    onConfirm(dynastyId: string, assetId: string): Promise<void>;
    onOverrideQc(dynastyId: string, reason: string): Promise<void>;
    onApprove(): Promise<void>;
}
```

- [ ] **Step 1: Write failing static render test**

```ts
expect(html).toContain("双人婚礼场景");
expect(html).toContain("自动 QC");
expect(html).toContain("生成缺失场景");
expect(html).toContain("确认场景阶段");
```

- [ ] **Step 2: Verify RED**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-scene-stage.test.tsx
```

Expected: FAIL because `scene-stage.tsx` does not exist.

- [ ] **Step 3: Build scene workspace**

Each dynasty shows selected male/female portrait references, the scene preview, generation progress, versions, QC booleans, QC reason/suggestion, and an override textarea. Primary actions call the real scene service from `workbench.tsx`; no local fake result.

- [ ] **Step 4: Verify GREEN and browser layout**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-scene-stage.test.tsx test/wedding-workbench.browser.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
```

The browser test must verify no horizontal overflow at `1440x960` and `390x900`.

- [ ] **Step 5: Commit**

```powershell
git add web/src/pages/wedding/scene-stage.tsx web/src/pages/wedding/workbench.tsx web/src/pages/wedding/wedding-workbench.css web/test/wedding-scene-stage.test.tsx web/test/wedding-workbench.browser.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增场景首帧工作区"
```

---

### Task 5: 视频状态机和真实视频任务

**Files:**
- Create: `web/test/wedding-video-runtime.test.ts`
- Create: `web/test/wedding-video-generation.test.ts`
- Create: `web/src/lib/wedding/video-runtime.ts`
- Create: `web/src/services/wedding-video-generation.ts`
- Modify: `web/src/lib/wedding/types.ts`
- Modify: `web/src/lib/wedding/production-state.ts`

**Interfaces:**
- Produces:
  - `WeddingVideoVersion`
  - `weddingVideoJobs(production, dynastyId?): WeddingVideoJob[]`
  - `markWeddingVideoGenerating`
  - `recordWeddingVideoVersion`
  - `selectWeddingVideoVersion`
  - `confirmWeddingVideo`
  - `failWeddingVideoGeneration`
  - `generateWeddingVideos(runtime, dynastyId?, options?)`

- [ ] **Step 1: Write failing video mapping and task tests**

```ts
expect(weddingVideoJobs(production)).toEqual([
    { nodeId: "video-tang", dynastyId: "tang", kind: "video" },
    { nodeId: "video-ming", dynastyId: "ming", kind: "video" },
]);

expect(submitted).toMatchObject({
    mode: "video",
    referenceImages: [
        { id: "male-asset" },
        { id: "female-asset" },
        { id: "scene-asset" },
    ],
    metadata: {
        videoEditOperation: "reference_to_video",
        creationTemplate: {
            nodeRef: "dynastyVideo",
            dynastyId: "tang",
            promptOverride: "PRODUCER EDITED PROMPT",
        },
    },
});
```

- [ ] **Step 2: Verify RED**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-video-runtime.test.ts test/wedding-video-generation.test.ts
```

Expected: FAIL because video runtime and service are missing.

- [ ] **Step 3: Implement state and generation**

Video generation requires a confirmed scene and both confirmed portraits. Map selected asset IDs to references, use the target video node's `seconds`, `size`, `vquality`, and `videoEditOperation`, and add:

```ts
creationTemplate: {
    ...creationTemplateGenerationMetadata(target.metadata).creationTemplate,
    promptOverride: production.dynasties.find((item) => item.id === dynastyId)?.promptOverride,
}
```

Use `consumeGenerationTaskNode`, persist the node, and append `WeddingVideoVersion`. A prompt change marks only that dynasty video and final delivery stale.

- [ ] **Step 4: Verify GREEN**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-video-runtime.test.ts test/wedding-video-generation.test.ts test/wedding-production-state.test.ts test/wedding-prompt.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
```

- [ ] **Step 5: Commit**

```powershell
git add web/src/lib/wedding/types.ts web/src/lib/wedding/production-state.ts web/src/lib/wedding/video-runtime.ts web/src/services/wedding-video-generation.ts web/test/wedding-video-runtime.test.ts web/test/wedding-video-generation.test.ts web/test/wedding-production-state.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 接入视频生成"
```

---

### Task 6: 视频导演工作区与版本审核

**Files:**
- Create: `web/test/wedding-video-stage.test.tsx`
- Create: `web/src/pages/wedding/video-stage.tsx`
- Modify: `web/src/pages/wedding/workbench.tsx`
- Modify: `web/src/pages/wedding/wedding-workbench.css`
- Modify: `web/test/wedding-workbench.browser.test.ts`

**Interfaces:**
- Consumes `VideoPromptInspector`, video generation service and video runtime.
- Produces `VideoStage` with real prompt save/restore, generation, playback, version selection, QC and confirmation.

- [ ] **Step 1: Write failing render test**

```ts
expect(html).toContain("最终英文视频提示词");
expect(html).toContain("生成缺失视频");
expect(html).toContain("历史版本");
expect(html).toContain("确认视频阶段");
```

- [ ] **Step 2: Verify RED**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-video-stage.test.tsx
```

Expected: FAIL because `video-stage.tsx` does not exist.

- [ ] **Step 3: Build video workspace**

Show the selected dynasty prompt editor, reference thumbnails, video settings, generation status, playable result, historical versions, video QC checks, and per-dynasty confirm action. The batch action generates only missing/failed videos; each dynasty exposes a retry action.

- [ ] **Step 4: Verify GREEN and browser**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-video-stage.test.tsx test/wedding-workbench.browser.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run lint
```

The browser test must switch to video stage, verify the prompt editor and generation action are present, and confirm no overflow at desktop and mobile sizes.

- [ ] **Step 5: Commit**

```powershell
git add web/src/pages/wedding/video-stage.tsx web/src/pages/wedding/workbench.tsx web/src/pages/wedding/wedding-workbench.css web/test/wedding-video-stage.test.tsx web/test/wedding-workbench.browser.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增视频导演工作区"
```

---

### Task 7: 第二阶段验收

**Files:**
- Modify: `docs/superpowers/specs/2026-10-25-cross-era-wedding-dedicated-production-workbench-design.md`
- Modify: `docs/superpowers/plans/2026-10-25-cross-era-wedding-dedicated-workbench.md`
- Modify: `docs/superpowers/plans/2026-10-25-cross-era-wedding-scenes-video.md`

**Interfaces:**
- Consumes all scene/video deliverables.
- Produces a verified staging point for packaging and final delivery.

- [ ] **Step 1: Run the full scene/video suite**

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-scene-runtime.test.ts test/wedding-scene-generation.test.ts test/wedding-scene-qc.test.ts test/wedding-scene-stage.test.tsx test/wedding-video-runtime.test.ts test/wedding-video-generation.test.ts test/wedding-video-stage.test.tsx test/wedding-workbench.browser.test.ts
```

- [ ] **Step 2: Run build and lint**

```powershell
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run lint
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run build
```

- [ ] **Step 3: Record verified behavior**

Document exact verified behaviors and any environment-only failures in the spec. Do not mark packaging or delivery complete.

- [ ] **Step 4: Commit**

```powershell
git add docs/superpowers/specs/2026-10-25-cross-era-wedding-dedicated-production-workbench-design.md docs/superpowers/plans/2026-10-25-cross-era-wedding-dedicated-workbench.md docs/superpowers/plans/2026-10-25-cross-era-wedding-scenes-video.md
git commit -m "docs(wedding-workbench): 跨时空婚礼电影 - 记录场景视频验收"
```

## Self-Review

- Spec coverage: scene generation, portrait-referenced scene frames, automatic QC and override, video prompt editing, real video tasks, version selection, per-dynasty retry and confirmation are covered.
- Placeholder scan: each task contains exact files, test commands, state/interface names and commit boundaries.
- Type consistency: task job types and stage actions use the existing `WeddingProductionState`, `CanvasNodeData`, `GenerationTask`, and shared `WeddingGenerationSummary`.
