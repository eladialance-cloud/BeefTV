# 跨时空婚礼独立制作台基础执行链 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将独立制作台恢复为唯一制作入口，并交付可保存项目设定、规划朝代、批量生成和审核定妆照的真实执行链。

**Architecture:** `/wedding/:canvasId` 直接渲染制作台，画布不再承载制作台覆盖层。制作台通过 `weddingProduction` 保存流程状态，通过现有 CanvasNodeData、生成任务和资产物化链路执行真实肖像生成。场景、视频、包装和成片由下一阶段计划接入。

**Tech Stack:** React 19、TypeScript、React Router、TanStack Query、Zustand、Ant Design、Go 1.25、Bun test、Playwright。

**Spec:** `docs/superpowers/specs/2026-10-25-cross-era-wedding-dedicated-production-workbench-design.md`

## Global Constraints

- `/wedding/:canvasId` 是唯一正常制作入口。
- 画布只作为底层节点、素材、任务和版本存储。
- 画布中不得出现“制作台”按钮。
- 画布主区域不得渲染婚礼制作台覆盖层。
- 每阶段批量生成、逐朝代审核、人工确认后推进。
- 单个朝代失败不阻塞其他朝代。
- 不复制新的任务系统，继续使用 `runBackendGenerationTask`、任务订阅和资产物化。
- 不展示模拟状态或把页面摘要当成真实生成结果。

## File Structure

```text
web/src/pages/wedding/
  workbench.tsx
  workbench-runtime.tsx
  setup-stage.tsx
  dynasty-stage.tsx
  portrait-stage.tsx
  wedding-workbench.css

web/src/lib/wedding/
  production-state.ts
  portrait-runtime.ts
  setup-state.ts
  dynasty-state.ts
  template-state.ts

web/src/services/
  wedding-generation.ts

web/src/pages/canvas/
  project.tsx
  canvas-project-top-bar.tsx

web/test/
  wedding-setup-state.test.ts
  wedding-dynasty-state.test.ts
  wedding-portrait-runtime.test.ts
  wedding-workbench-runtime.browser.test.ts
```

---

### Task 1: 移除画布内制作台并恢复独立入口

**Files:**
- Modify: `web/src/lib/wedding/routing.ts`
- Modify: `web/src/pages/home/index.tsx`
- Modify: `web/src/pages/wedding/workbench.tsx`
- Modify: `web/src/pages/canvas/project.tsx`
- Modify: `web/src/pages/canvas/canvas-project-top-bar.tsx`
- Modify: `web/test/wedding-routing.test.ts`
- Modify: `web/test/wedding-workbench.browser.test.ts`

**Interfaces:**
- Produces: `weddingWorkbenchPathForTemplate()` returns `/wedding/:canvasId`.
- Removes: canvas `canvasView=wedding`, topbar workbench toggle and overlay.

- [ ] **Step 1: Write failing route test**

```ts
expect(weddingWorkbenchPathForTemplate(
    { itemKey: "cross-era-wedding-volc" },
    "canvas 1",
)).toBe("/wedding/canvas%201");
```

Run:

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-routing.test.ts
```

Expected: FAIL because the current path points to `/canvas/...?canvasView=wedding`.

- [ ] **Step 2: Restore dedicated route**

Change `weddingWorkbenchPathForTemplate` to return:

```ts
`/wedding/${encodeURIComponent(canvasId)}`
```

Change `WeddingWorkbenchPage` to render `WeddingWorkbenchSurface` directly instead of redirecting.

- [ ] **Step 3: Remove canvas overlay and toggle**

In `CanvasPage`, remove:

- `weddingWorkbenchOpen`
- `weddingProduction`
- `legacyWeddingProduction`
- `weddingWorkbenchEnabled`
- `toggleWeddingWorkbench`
- backfill effect
- `WeddingWorkbenchSurface`
- overlay DOM
- `hidden={weddingWorkbenchOpen}`
- toolbar conditions based on `weddingWorkbenchOpen`

In `CanvasTopBar`, remove wedding-workbench props and both toggle buttons.

- [ ] **Step 4: Add browser regression**

Assert `/canvas/c1` has no “制作台” button, while `/wedding/c1` renders the seven-stage workbench.

- [ ] **Step 5: Run tests and build**

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-routing.test.ts test/wedding-workbench.browser.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
```

- [x] **Step 6: Commit**

```powershell
cd D:\画布\BeefTV
git add web/src/lib/wedding/routing.ts web/src/pages/home/index.tsx web/src/pages/wedding/workbench.tsx web/src/pages/canvas/project.tsx web/src/pages/canvas/canvas-project-top-bar.tsx web/test/wedding-routing.test.ts web/test/wedding-workbench.browser.test.ts
git commit -m "refactor(wedding-workbench): 跨时空婚礼电影 - 恢复独立制作台入口"
```

---

### Task 2: 制作台运行时和阶段容器

**Files:**
- Create: `web/src/pages/wedding/workbench-runtime.tsx`
- Modify: `web/src/pages/wedding/workbench.tsx`
- Modify: `web/src/pages/wedding/wedding-workbench.css`
- Create: `web/test/wedding-workbench-runtime.test.tsx`

**Interfaces:**
- Produces:
  - `useWeddingWorkbenchRuntime(canvasId): { project, production, activeStage, setActiveStage, saveProduction, saveBusy, feedback }`
  - stage component contract `{ production, runtime }`

- [ ] **Step 1: Write failing runtime test**

Render the workbench with a seeded canvas that lacks `weddingProduction`; assert it derives state from existing nodes and displays setup.

- [ ] **Step 2: Implement runtime hook**

The hook must:

- Load the canvas project when absent.
- Use `project.weddingProduction` when present.
- Backfill from `buildWeddingProductionFromCanvasNodes`.
- Persist the backfilled state with `persistCanvasDocument`.
- Expose `saveProduction(next)`.
- Expose save feedback and busy state.

- [ ] **Step 3: Replace direct page state**

Move project loading, production state, active stage and prompt feedback out of `workbench.tsx` into the runtime hook. Stage components receive the runtime.

- [ ] **Step 4: Render real stage router**

```tsx
switch (activeStage) {
    case "setup": return <SetupStage production={production} runtime={runtime} />;
    case "dynasties": return <DynastyStage production={production} runtime={runtime} />;
    case "portraits": return <PortraitStage production={production} runtime={runtime} />;
    default: return <StageUnavailable stage={activeStage} />;
}
```

`StageUnavailable` may explain that the stage is not enabled yet, but it must not show fake actions.

- [ ] **Step 5: Run test and typecheck, commit**

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-workbench-runtime.test.tsx
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck

cd D:\画布\BeefTV
git add web/src/pages/wedding/workbench-runtime.tsx web/src/pages/wedding/workbench.tsx web/src/pages/wedding/wedding-workbench.css web/test/wedding-workbench-runtime.test.tsx
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增制作台运行时"
```

---

### Task 3: 项目设定真实表单和阶段确认

**Files:**
- Create: `web/src/lib/wedding/setup-state.ts`
- Create: `web/src/pages/wedding/setup-stage.tsx`
- Create: `web/test/wedding-setup-state.test.ts`
- Modify: `web/src/lib/wedding/production-state.ts`

**Interfaces:**
- Produces:
  - `updateWeddingSource(production, patch, now)`
  - `approveWeddingSetup(production, now)`
  - `SetupStage`

- [ ] **Step 1: Write failing invalidation tests**

Test photo change and output-spec change invalidation from the approved spec.

- [ ] **Step 2: Implement state functions**

Implement exact invalidation and stage blockers. Reuse node IDs from `CanvasProject`.

- [ ] **Step 3: Build setup form**

Render photo summaries, names, story, ratio, resolution, consent and save/confirm actions. Upload replacement uses the existing resource upload service.

- [ ] **Step 4: Wire save and stage approval**

Save calls `runtime.saveProduction(next)`. Confirmation first saves, then calls `approveWeddingSetup`.

- [ ] **Step 5: Run tests and commit**

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-setup-state.test.ts test/wedding-workbench-runtime.test.tsx
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck

cd D:\画布\BeefTV
git add web/src/lib/wedding/setup-state.ts web/src/pages/wedding/setup-stage.tsx web/test/wedding-setup-state.test.ts web/src/lib/wedding/production-state.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增项目设定阶段"
```

---

### Task 4: 朝代规划和节点补齐

**Files:**
- Create: `web/src/lib/wedding/dynasty-state.ts`
- Create: `web/src/pages/wedding/dynasty-stage.tsx`
- Create: `web/test/wedding-dynasty-state.test.ts`
- Modify: `web/src/pages/wedding/workbench.tsx`

**Interfaces:**
- Produces:
  - `setWeddingDynasties(production, orderedIds, now)`
  - `missingWeddingDynastyNodes(production)`
  - `DynastyStage`

For the first implementation, ordering and removing selected dynasties are supported. Adding a dynasty creates the four required nodes from safe template definitions in `template-state.ts`.

- [ ] **Step 1: Test ordering, removal and add**

Assert order persistence, downstream invalidation and exact node creation.

- [ ] **Step 2: Implement pure state changes**

Use existing dynasty catalog and node IDs; never duplicate a dynasty ID.

- [ ] **Step 3: Build the reorderable dynasty list**

Provide up/down controls, remove, add from remaining catalog entries and a five-entry limit.

- [ ] **Step 4: Run tests and commit**

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-dynasty-state.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck

cd D:\画布\BeefTV
git add web/src/lib/wedding/dynasty-state.ts web/src/pages/wedding/dynasty-stage.tsx web/test/wedding-dynasty-state.test.ts web/src/pages/wedding/workbench.tsx
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增朝代规划阶段"
```

---

### Task 5: 定妆照批量生成和审核

**Files:**
- Create: `web/src/services/wedding-generation.ts`
- Create: `web/src/lib/wedding/portrait-runtime.ts`
- Create: `web/src/pages/wedding/portrait-stage.tsx`
- Create: `web/test/wedding-portrait-runtime.test.ts`
- Modify: `web/src/pages/wedding/workbench.tsx`

**Interfaces:**
- Produces:
  - `weddingPortraitJobs(production): WeddingGenerationJob[]`
  - `generateWeddingPortraits(runtime, dynastyId?): Promise<void>`
  - `PortraitStage`

Each job contains:

```ts
type WeddingGenerationJob = {
    nodeId: string;
    dynastyId: string;
    kind: "portrait";
    promptSourceNodeId: string;
};
```

- [x] **Step 1: Test job mapping**

Assert two jobs are created per dynasty, with correct male and female node IDs.

- [x] **Step 2: Implement generation service**

Use the selected effective image model and call `runBackendGenerationTask` with:

- `mode: "image"`
- existing portrait node metadata
- source photo as reference image
- `metadata.creationTemplate` from node plugin data
- project ID

Materialize the returned image into the node and persist it with `persistCanvasDocument`.

- [x] **Step 3: Build portrait board**

Show male/female pairs per dynasty, status, preview, error and retry. Batch button processes missing/failed portraits with bounded concurrency.

- [x] **Step 4: Implement selection and approval**

Selecting a result writes the selected asset ID. Confirming the stage requires both genders approved for every selected dynasty.

- [x] **Step 5: Run tests, typecheck and browser verification**

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-portrait-runtime.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-workbench-runtime.browser.test.ts
```

- [ ] **Step 6: Commit**

```powershell
cd D:\画布\BeefTV
git add web/src/services/wedding-generation.ts web/src/lib/wedding/portrait-runtime.ts web/src/pages/wedding/portrait-stage.tsx web/test/wedding-portrait-runtime.test.ts web/src/pages/wedding/workbench.tsx
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 接入定妆照生成"
```

---

### Task 6: 第一阶段验收和后续计划

**Files:**
- Modify: `docs/superpowers/specs/2026-10-25-cross-era-wedding-dedicated-production-workbench-design.md`
- Create: `docs/superpowers/plans/2026-10-25-cross-era-wedding-scenes-video.md`

**Interfaces:**
- Consumes: completed dedicated workbench through portrait stage.
- Produces: implementation plan for scenes, video, packaging and delivery.

- [x] **Step 1: Verify first-stage coverage**

Verify:

- `/wedding/:canvasId` is the only entry.
- Canvas has no workbench toggle.
- Setup saves and invalidates correctly.
- Dynasties can be ordered, added and removed.
- Portraits call real tasks and materialize assets.

- [x] **Step 2: Run first-stage suite**

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-routing.test.ts test/wedding-workbench-runtime.test.tsx test/wedding-setup-state.test.ts test/wedding-dynasty-state.test.ts test/wedding-portrait-runtime.test.ts test/wedding-workbench-runtime.browser.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run lint
```

- [x] **Step 3: Write the scenes/video plan**

Create the next plan with explicit tasks for:

- scene generation and QC
- video prompt editing and generation
- version comparison and stage approval
- failure isolation and retry

- [ ] **Step 4: Commit**

```powershell
cd D:\画布\BeefTV
git add docs/superpowers/specs/2026-10-25-cross-era-wedding-dedicated-production-workbench-design.md docs/superpowers/plans/2026-10-25-cross-era-wedding-scenes-video.md
git commit -m "docs(wedding-workbench): 跨时空婚礼电影 - 记录第一阶段验收"
```

## Self-Review

- Spec coverage: this plan implements dedicated entry, setup, dynasty planning and portrait production. Scene, video, packaging and delivery are explicitly assigned to the next plan in Task 6.
- Placeholder scan: all tasks identify exact files, interfaces, test commands and commit messages.
- Type consistency: stage components consume `weddingProduction` and the runtime hook. Generation jobs reference real node IDs.
