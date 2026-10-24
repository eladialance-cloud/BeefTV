# 跨时空婚礼电影项目设定阶段 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在已完成的制作方工作台中交付可编辑、可保存、可确认并会精确失效下游结果的项目设定阶段。

**Architecture:** 项目设定继续读写 `CanvasProject.weddingProduction.source`，使用纯状态函数计算照片、授权、故事和输出规格变化的影响范围。页面组件只负责表单交互，保存统一经过画布文档持久化。

**Tech Stack:** React 19、TypeScript、Zustand、TanStack Query、Bun test、Playwright。

**Spec:** `docs/superpowers/specs/2026-10-25-cross-era-wedding-producer-workbench-design.md`

## Global Constraints

- 只服务制作方，不增加新人端。
- 不伪造照片上传或授权状态。
- 更换人物照片必须让所有朝代肖像、场景、视频和成片失效。
- 修改故事或输出规格必须让场景、视频和成片失效。
- 保存失败时不得显示“已保存”。
- 所有写路径使用 `persistCanvasDocument`。

## File Structure

```text
web/src/lib/wedding/
  production-state.ts
  setup-state.ts

web/src/pages/wedding/
  workbench.tsx
  setup-stage.tsx
  wedding-workbench.css

web/test/
  wedding-setup-state.test.ts
  wedding-setup-stage.browser.test.ts
  fixtures/wedding-setup-harness.tsx
```

---

### Task 1: 项目设定纯状态更新和失效

**Files:**
- Modify: `web/src/lib/wedding/production-state.ts`
- Create: `web/src/lib/wedding/setup-state.ts`
- Create: `web/test/wedding-setup-state.test.ts`

**Interfaces:**
- Consumes: `WeddingProductionState`.
- Produces:
  - `updateWeddingSource(state, patch, now): WeddingProductionState`
  - `approveWeddingSetup(state, now): WeddingProductionState`

- [ ] **Step 1: Write failing setup state tests**

```ts
import { expect, test } from "bun:test";
import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { approveWeddingSetup, updateWeddingSource } from "@/lib/wedding/setup-state";

const base = createWeddingProductionState({
    template: { id: "template-1", key: "cross-era-wedding-volc", version: "1" },
    source: {
        groomPhotoNodeId: "groom-old",
        bridePhotoNodeId: "bride-old",
        maleName: "林屿",
        femaleName: "周棠",
        story: "旧故事",
        consentConfirmed: true,
        ratio: "16:9",
        resolution: "720p",
    },
    dynasties: [{
        id: "tang",
        name: "唐",
        order: 0,
        portraitMaleNodeId: "male",
        portraitFemaleNodeId: "female",
        sceneNodeId: "scene",
        videoNodeId: "video",
    }],
    now: "2026-10-25T00:00:00.000Z",
});

test("changing a source photo invalidates every downstream asset", () => {
    const changed = updateWeddingSource(base, { groomPhotoNodeId: "groom-new" }, "2026-10-25T01:00:00.000Z");
    expect(changed.dynasties[0].statuses).toEqual({ portrait: "stale", scene: "stale", video: "stale" });
    expect(changed.stages.portraits.status).toBe("stale");
    expect(changed.stages.delivery.status).toBe("stale");
});

test("changing output settings keeps portraits but invalidates scenes and videos", () => {
    const changed = updateWeddingSource(base, { ratio: "9:16" }, "2026-10-25T01:00:00.000Z");
    expect(changed.dynasties[0].statuses.portrait).toBe("ready");
    expect(changed.dynasties[0].statuses.scene).toBe("stale");
    expect(changed.dynasties[0].statuses.video).toBe("stale");
});

test("setup cannot be approved without photos and consent", () => {
    const incomplete = updateWeddingSource(base, { groomPhotoNodeId: "", consentConfirmed: false }, "2026-10-25T01:00:00.000Z");
    expect(approveWeddingSetup(incomplete, "2026-10-25T02:00:00.000Z")).toBe(incomplete);
    expect(approveWeddingSetup(base, "2026-10-25T02:00:00.000Z").stages.setup.status).toBe("approved");
});
```

- [ ] **Step 2: Run the tests and verify failure**

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-setup-state.test.ts
```

Expected: FAIL because `setup-state.ts` does not exist.

- [ ] **Step 3: Implement setup state changes**

`updateWeddingSource` must:

- Merge the patch into `state.source`.
- On `groomPhotoNodeId` or `bridePhotoNodeId` change, mark portrait, scene and video stale for every dynasty.
- On `story`, `ratio` or `resolution` change, mark scene and video stale for every dynasty.
- Mark packaging and delivery stale for any change that invalidates scenes or videos.
- Append one activity event with the changed field names.
- Return the original state when every patch value equals the current value.

`approveWeddingSetup` must require both photo node IDs, `consentConfirmed === true`, and non-empty male and female names before writing `approved`.

- [ ] **Step 4: Run tests and typecheck**

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-setup-state.test.ts test/wedding-production-state.test.ts
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
cd D:\画布\BeefTV
git add web/src/lib/wedding/production-state.ts web/src/lib/wedding/setup-state.ts web/test/wedding-setup-state.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增项目设定状态"
```

---

### Task 2: 项目设定表单

**Files:**
- Create: `web/src/pages/wedding/setup-stage.tsx`
- Modify: `web/src/pages/wedding/workbench.tsx`
- Modify: `web/src/pages/wedding/wedding-workbench.css`

**Interfaces:**
- Consumes: `WeddingProductionState`, `updateWeddingSource`, `approveWeddingSetup`, `persistCanvasDocument`.
- Produces: functional setup stage with save and approval.

- [ ] **Step 1: Add server-render coverage**

Create `web/test/wedding-setup-stage.test.tsx`:

```tsx
import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { SetupStage } from "@/pages/wedding/setup-stage";
import { createWeddingProductionState } from "@/lib/wedding/production-state";

test("setup stage renders source, story, output and consent controls", () => {
    const state = createWeddingProductionState({
        template: { id: "t", key: "cross-era-wedding-volc", version: "1" },
        source: {
            groomPhotoNodeId: "g", bridePhotoNodeId: "b", maleName: "男", femaleName: "女",
            story: "", consentConfirmed: false, ratio: "16:9", resolution: "720p",
        },
        dynasties: [],
        now: "2026-10-25T00:00:00.000Z",
    });
    const html = renderToStaticMarkup(<SetupStage state={state} saving={false} onSave={async () => undefined} onApprove={async () => undefined} />);
    expect(html).toContain("新郎称呼");
    expect(html).toContain("爱情故事");
    expect(html).toContain("照片使用授权");
    expect(html).toContain("确认项目设定");
});
```

Run:

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-setup-stage.test.tsx
```

Expected: FAIL because `SetupStage` does not exist.

- [ ] **Step 2: Implement the setup form**

The component renders:

- Male and female names.
- Story textarea.
- Ratio segmented control for `16:9` and `9:16`.
- Resolution segmented control for `480p`, `720p` and `1080p`.
- Consent checkbox.
- Existing groom and bride resource summaries with links to the underlying canvas.
- Save button and confirm button.

The component keeps draft values local. Save calls `onSave(patch)`. Confirm first saves the current draft, then calls `onApprove`.

- [ ] **Step 3: Wire persistence in workbench**

In `workbench.tsx`, when `activeStage === "setup"`, render `SetupStage`.

`onSave` computes `updateWeddingSource` and awaits:

```ts
await persistCanvasDocument(project.id, { weddingProduction: next });
```

`onApprove` calls `approveWeddingSetup` and persists it only when the returned state changes. Show a precise success or failure message.

- [ ] **Step 4: Run tests, typecheck and lint**

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-setup-stage.test.tsx test/wedding-setup-state.test.ts test/wedding-workbench-render.test.tsx
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run typecheck
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe run lint
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
cd D:\画布\BeefTV
git add web/src/pages/wedding/setup-stage.tsx web/src/pages/wedding/workbench.tsx web/src/pages/wedding/wedding-workbench.css web/test/wedding-setup-stage.test.tsx
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增项目设定表单"
```

---

### Task 3: 浏览器持久化验证

**Files:**
- Create: `web/test/fixtures/wedding-setup-harness.tsx`
- Create: `web/test/wedding-setup-stage.browser.test.ts`

**Interfaces:**
- Consumes: actual workbench page, canvas API, prompt API and commit API.
- Produces: proof that setup edits and approval survive reload.

- [ ] **Step 1: Create the setup browser harness**

Render `/wedding/c1` with `MemoryRouter`, `QueryClientProvider` and the real workbench page. Return incomplete setup state from `/api/canvas-projects/c1`.

- [ ] **Step 2: Write the browser test**

The test must:

1. Open the setup stage.
2. Edit both names, story, ratio and resolution.
3. Check consent.
4. Save and reload.
5. Assert the values survive.
6. Confirm the stage and assert its status becomes approved after reload.
7. Change the groom photo node ID through the API-backed fixture and assert downstream stages become stale.
8. Verify `document.documentElement.scrollWidth <= window.innerWidth` at 1440px and 390px.

- [ ] **Step 3: Run the browser test**

```powershell
cd D:\画布\BeefTV\web
D:\画布\_tools\bun\node_modules\bun\bin\bun.exe test test/wedding-setup-stage.browser.test.ts
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
cd D:\画布\BeefTV
git add web/test/fixtures/wedding-setup-harness.tsx web/test/wedding-setup-stage.browser.test.ts
git commit -m "test(wedding-workbench): 跨时空婚礼电影 - 验证项目设定持久化"
```

## Self-Review

- Setup changes use exact downstream invalidation rules from the spec.
- Save and approval both use the existing canvas document persistence boundary.
- No generation or preview data is fabricated.
- The plan defines the functions and props used by each task in the order they are introduced.
