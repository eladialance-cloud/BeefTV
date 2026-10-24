# 跨时空婚礼电影制作方工作台基础与提示词控制 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付制作方专用工作台的第一条可用垂直链，包括生产状态、画布持久化、专用路由、七阶段导航，以及由后台真实解析的视频提示词预览和覆盖保存。

**Architecture:** 工作台数据挂在现有 `CanvasProject` 顶层 `weddingProduction`，继续使用画布 revision、自动保存和历史。专用页面 `/wedding/:canvasId` 复用画布项目与素材节点，只替换制作体验。视频默认提示词通过新增的制作方预览接口从服务端私有目录解析，制作方覆盖值保存在画布状态并在生成请求中传给现有提示词准入逻辑。

**Tech Stack:** React 19、TypeScript、React Router、Zustand、TanStack Query、Ant Design、Go 1.25、Gin、GORM、Bun test、Playwright。

**Spec:** `docs/superpowers/specs/2026-10-25-cross-era-wedding-producer-workbench-design.md`

## Global Constraints

- 工作台只服务制作方，不建设新人端、客户权限或远程审片。
- 素材、任务、模型渠道、上传、资源存储和时间线渲染继续使用现有实现。
- 页面不得出现假生成按钮，也不得把静态状态描述成真实模型结果。
- `promptOverride` 非空时作为最终视频提示词；为空时使用服务端解析的模板提示词。
- 提示词覆盖只允许来自已启用工作流的 `dynastyVideo` 节点，长度为 1 至 12000 个字符。
- 所有业务 API 必须通过 `web/src/services/api/request.ts` 的 `http`。
- `CanvasProject` 的 revision、自动保存、history 和冲突处理是唯一持久化权威。
- 不修改漫剧工作室，不引入新依赖，不读取或输出 API Key。
- 本计划只交付基础与提示词垂直链。肖像、场景、视频生成、包装和交付拆成后续计划，但本计划创建这些阶段的状态骨架。

## File Structure

```text
backend/internal/app/
  creation_prompt.go
  creation_prompt_test.go

backend/internal/hostedapi/
  creation_catalog.go
  creation_catalog_test.go

web/src/lib/wedding/
  types.ts
  production-state.ts
  template-state.ts
  prompt.ts
  routing.ts

web/src/pages/wedding/
  workbench.tsx
  wedding-workbench.css
  workbench-stage-rail.tsx
  workbench-top-bar.tsx
  video-prompt-inspector.tsx

web/src/services/api/
  creation-templates.ts

web/src/services/
  local-workspace-repository.ts
  local-workspace-sync.ts
  workspace-project-repository.ts
  workflow-template-canvas.ts

web/src/stores/canvas/
  use-canvas-store.ts

web/src/pages/home/
  index.tsx

web/src/lib/
  workspace-route-modules.ts

web/src/
  router.tsx

web/test/
  wedding-production-state.test.ts
  wedding-template-state.test.ts
  wedding-routing.test.ts
  wedding-prompt.test.ts
  wedding-workbench.browser.test.ts
```

---

### Task 1: 生产状态类型和纯状态机

**Files:**
- Create: `web/src/lib/wedding/types.ts`
- Create: `web/src/lib/wedding/production-state.ts`
- Create: `web/test/wedding-production-state.test.ts`

**Interfaces:**
- Consumes: existing `CanvasProject` and `CanvasNodeData` types.
- Produces:
  - `WeddingStageId`
  - `WeddingItemStatus`
  - `WeddingProductionState`
  - `createWeddingProductionState(input: CreateWeddingProductionInput): WeddingProductionState`
  - `approveWeddingStage(state, stage, now): WeddingProductionState`
  - `invalidateWeddingDynasty(state, dynastyId, from: "portrait" | "scene" | "video" | "prompt"): WeddingProductionState`
  - `weddingStageBlockers(state, stage): string[]`

- [ ] **Step 1: Write the failing state tests**

```ts
import { describe, expect, test } from "bun:test";
import {
    approveWeddingStage,
    createWeddingProductionState,
    invalidateWeddingDynasty,
    weddingStageBlockers,
} from "@/lib/wedding/production-state";

const input = {
    template: { id: "template-1", key: "cross-era-wedding-volc", version: "1.0.0" },
    source: {
        groomPhotoNodeId: "groom",
        bridePhotoNodeId: "bride",
        maleName: "林屿",
        femaleName: "周棠",
        story: "杭州西湖相识",
        consentConfirmed: true,
        ratio: "16:9",
        resolution: "720p",
    },
    dynasties: [{
        id: "tang",
        name: "唐",
        order: 0,
        portraitMaleNodeId: "tang-male",
        portraitFemaleNodeId: "tang-female",
        sceneNodeId: "tang-scene",
        videoNodeId: "tang-video",
    }],
    now: "2026-10-25T00:00:00.000Z",
};

describe("wedding production state", () => {
    test("creates seven stages and ordered dynasties", () => {
        const state = createWeddingProductionState(input);
        expect(Object.keys(state.stages)).toEqual([
            "setup", "dynasties", "portraits", "scenes", "videos", "packaging", "delivery",
        ]);
        expect(state.stages.setup.status).toBe("ready");
        expect(state.dynasties[0]).toMatchObject({ id: "tang", name: "唐", order: 0, statuses: { portrait: "ready" } });
    });

    test("prompt invalidation marks video and final stages stale", () => {
        const state = createWeddingProductionState(input);
        const changed = invalidateWeddingDynasty(state, "tang", "prompt");
        expect(changed.dynasties[0].statuses.video).toBe("stale");
        expect(changed.stages.videos.status).toBe("stale");
        expect(changed.stages.delivery.status).toBe("stale");
    });

    test("cannot approve delivery while media stages have blockers", () => {
        const state = createWeddingProductionState(input);
        expect(weddingStageBlockers(state, "delivery")).toContain("唐：视频尚未确认");
        const approved = approveWeddingStage(state, "delivery", "2026-10-25T01:00:00.000Z");
        expect(approved).toBe(state);
    });
});
```

- [ ] **Step 2: Run the tests to verify failure**

Run:

```powershell
cd D:\画布\BeefTV\web
bun test test/wedding-production-state.test.ts
```

Expected: FAIL because `@/lib/wedding/production-state` does not exist.

- [ ] **Step 3: Add the exact state types**

Create `web/src/lib/wedding/types.ts` with the types from the spec, including `WeddingSceneQc`, `WeddingVideoQc`, `WeddingDynastyState`, `WeddingPackagingState`, `WeddingActivityEvent`, and `WeddingProductionState`. Export `WEDDING_STAGE_IDS` as:

```ts
export const WEDDING_STAGE_IDS = [
    "setup",
    "dynasties",
    "portraits",
    "scenes",
    "videos",
    "packaging",
    "delivery",
] as const;
```

- [ ] **Step 4: Implement the minimal pure state machine**

In `production-state.ts`, create every stage with `not_started`, set `setup` and `dynasties` to `ready`, and set each new dynasty's portrait, scene, and video items to `ready`. Implement invalidation so prompt changes mark `video` and `delivery` stale, scene changes mark `video` and `delivery` stale, portrait changes mark scene, video, and delivery stale, and video changes mark delivery stale. Implement `weddingStageBlockers` from the dynasty and packaging state without reading React or stores.

- [ ] **Step 5: Run the tests and typecheck**

Run:

```powershell
cd D:\画布\BeefTV\web
bun test test/wedding-production-state.test.ts
bun run typecheck
```

Expected: all state tests PASS and TypeScript reports no errors.

- [ ] **Step 6: Commit**

```powershell
cd D:\画布\BeefTV
git add web/src/lib/wedding/types.ts web/src/lib/wedding/production-state.ts web/test/wedding-production-state.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增生产状态机"
```

---

### Task 2: 从模板初始化工作台状态并持久化到画布

**Files:**
- Create: `web/src/lib/wedding/template-state.ts`
- Create: `web/test/wedding-template-state.test.ts`
- Modify: `web/src/stores/canvas/use-canvas-store.ts`
- Modify: `web/src/services/local-workspace-repository.ts`
- Modify: `web/src/services/local-workspace-sync.ts`
- Modify: `web/src/services/workspace-project-repository.ts`
- Modify: `web/src/services/workflow-template-canvas.ts`

**Interfaces:**
- Consumes: `WeddingProductionState`, `CanvasProject`, and the converted canvas nodes.
- Produces:
  - `buildWeddingProductionFromTemplate(template, variables, nodes, now): WeddingProductionState`
  - `CanvasProject.weddingProduction?: WeddingProductionState`
  - `InitialCanvasContent` accepts `weddingProduction`

- [ ] **Step 1: Write the failing template initialization test**

```ts
import { describe, expect, test } from "bun:test";
import { buildWeddingProductionFromTemplate } from "@/lib/wedding/template-state";

test("maps selected dynasty nodes into production state", () => {
    const state = buildWeddingProductionFromTemplate(
        { id: "template-1", itemKey: "cross-era-wedding-volc", name: "跨时空婚礼电影", payloadJson: "{}" },
        {
            maleName: "林屿",
            femaleName: "周棠",
            customDesc: "杭州西湖相识",
            dynasties: ["tang", "ming"],
            ratio: "16:9",
            resolution: "720p",
        },
        [
            { id: "groom", type: "image", metadata: { uploadField: "groomPhoto" } },
            { id: "bride", type: "image", metadata: { uploadField: "bridePhoto" } },
            { id: "male-tang", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyPortraitMale", creationTemplateDynastyId: "tang" } } },
            { id: "female-tang", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyPortraitFemale", creationTemplateDynastyId: "tang" } } },
            { id: "scene-tang", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyScene", creationTemplateDynastyId: "tang" } } },
            { id: "video-tang", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyVideo", creationTemplateDynastyId: "tang" } } },
        ],
        "2026-10-25T00:00:00.000Z",
    );
    expect(state.source.groomPhotoNodeId).toBe("groom");
    expect(state.dynasties).toHaveLength(1);
    expect(state.dynasties[0]).toMatchObject({
        id: "tang",
        name: "唐",
        portraitMaleNodeId: "male-tang",
        portraitFemaleNodeId: "female-tang",
        sceneNodeId: "scene-tang",
        videoNodeId: "video-tang",
    });
});
```

- [ ] **Step 2: Run the test to verify failure**

Run:

```powershell
cd D:\画布\BeefTV\web
bun test test/wedding-template-state.test.ts
```

Expected: FAIL because `template-state.ts` does not exist.

- [ ] **Step 3: Implement template state mapping**

`buildWeddingProductionFromTemplate` must:

- Read selected dynasty IDs in the order supplied by variables.
- Find nodes by `metadata.pluginData.creationTemplateDynamicRef` and `creationTemplateDynastyId`.
- Find source photos by `metadata.uploadField`.
- Throw a descriptive error when a required node is missing.
- Ignore selected dynasty IDs that have no public option because they are invalid.
- Build `template`, `source`, `dynasties`, empty packaging state, and the initial activity event.

- [ ] **Step 4: Extend canvas initialization and persistence types**

Add `weddingProduction?: WeddingProductionState` to `CanvasProject`. Add it to:

- `InitialCanvasContent`
- `LocalCanvasContent`
- `CanvasDocumentPersistPatch`
- `syncLocalCanvasSnapshot` patch
- `createWorkspaceCanvasProject` initial content type
- `createCanvasProjectWithRemoteSync` initial content type

In `createWorkspaceCanvasFromTemplate`, after `buildCanvasWorkflowFromTemplate`, call `buildWeddingProductionFromTemplate` only when `template.itemKey === "cross-era-wedding-volc"`, then pass `weddingProduction` in initial content.

- [ ] **Step 5: Add a persistence regression test**

Create `web/test/wedding-template-state.test.ts` with a second test that writes a production state through `persistCanvasDocument` using an isolated local canvas. Reset the store and persistence harness using the existing test helpers in `local-workspace-bootstrap.test.ts`. Reload the project and assert the exact prompt override and stage status survive.

- [ ] **Step 6: Run targeted tests and typecheck**

Run:

```powershell
cd D:\画布\BeefTV\web
bun test test/wedding-template-state.test.ts test/wedding-production-state.test.ts test/local-workspace-bootstrap.test.ts
bun run typecheck
```

Expected: all targeted tests PASS and TypeScript reports no errors.

- [ ] **Step 7: Commit**

```powershell
cd D:\画布\BeefTV
git add web/src/lib/wedding/template-state.ts web/test/wedding-template-state.test.ts web/src/stores/canvas/use-canvas-store.ts web/src/services/local-workspace-repository.ts web/src/services/local-workspace-sync.ts web/src/services/workspace-project-repository.ts web/src/services/workflow-template-canvas.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 持久化制作状态"
```

---

### Task 3: 专用路由和首页入口

**Files:**
- Create: `web/src/lib/wedding/routing.ts`
- Create: `web/src/pages/wedding/workbench.tsx`
- Create: `web/src/pages/wedding/wedding-workbench.css`
- Create: `web/test/wedding-routing.test.ts`
- Modify: `web/src/pages/home/index.tsx`
- Modify: `web/src/lib/workspace-route-modules.ts`
- Modify: `web/src/router.tsx`

**Interfaces:**
- Consumes: `PublicCreationTemplate` and created canvas ID.
- Produces:
  - `weddingWorkbenchPathForTemplate(template, canvasId): string`
  - route `/wedding/:canvasId`
  - `loadWeddingWorkbenchPage`

- [ ] **Step 1: Write the failing route test**

```ts
import { describe, expect, test } from "bun:test";
import { weddingWorkbenchPathForTemplate } from "@/lib/wedding/routing";

describe("wedding workbench routing", () => {
    test("cross-era wedding opens the producer workbench", () => {
        expect(weddingWorkbenchPathForTemplate({ itemKey: "cross-era-wedding-volc" }, "canvas 1"))
            .toBe("/wedding/canvas%201");
    });

    test("other workflows keep the generic canvas route", () => {
        expect(weddingWorkbenchPathForTemplate({ itemKey: "wedding-video-guided-wizard" }, "canvas-2"))
            .toBe("/canvas/canvas-2");
    });
});
```

- [ ] **Step 2: Run the test to verify failure**

Run:

```powershell
cd D:\画布\BeefTV\web
bun test test/wedding-routing.test.ts
```

Expected: FAIL because `routing.ts` does not exist.

- [ ] **Step 3: Implement the route selector and page**

Create the route helper:

```ts
export function weddingWorkbenchPathForTemplate(
    template: Pick<PublicCreationTemplate, "itemKey">,
    canvasId: string,
) {
    return template.itemKey === "cross-era-wedding-volc"
        ? `/wedding/${encodeURIComponent(canvasId)}`
        : `/canvas/${encodeURIComponent(canvasId)}`;
}
```

The first `workbench.tsx` render reads `canvasId`, finds the project in `useCanvasStore`, calls `openLocalCanvasProjectFromBackend(canvasId)` when the live project is absent, renders the project title and the seven-stage rail from the persisted `weddingProduction`, and shows a precise error when the project or production state is missing. It must not create replacement demo state.

- [ ] **Step 4: Wire the home entry and router**

In `HomePage.submitWorkflow`, replace the fixed `/canvas/${created.id}` navigation with `weddingWorkbenchPathForTemplate(setupTemplate, created.id)`.

Add `wedding: () => import("@/pages/wedding/workbench")` to `workspaceRouteLoaders`, export `loadWeddingWorkbenchPage`, and register:

```tsx
{ path: "/wedding/:canvasId", element: deferred(<WeddingWorkbenchPage />) }
```

- [ ] **Step 5: Add route and render tests**

Extend `wedding-routing.test.ts` with:

```ts
test("encodes canvas ids in the dedicated path", () => {
    expect(weddingWorkbenchPathForTemplate({ itemKey: "cross-era-wedding-volc" }, "a/b"))
        .toBe("/wedding/a%2Fb");
});
```

Add a server-render test in the same file using `renderToStaticMarkup` for the stage rail component, asserting all seven Chinese stage names appear in order.

- [ ] **Step 6: Run tests, typecheck and build**

Run:

```powershell
cd D:\画布\BeefTV\web
bun test test/wedding-routing.test.ts
bun run typecheck
bun run build
```

Expected: tests PASS, typecheck PASS and Vite build succeeds.

- [ ] **Step 7: Commit**

```powershell
cd D:\画布\BeefTV
git add web/src/lib/wedding/routing.ts web/test/wedding-routing.test.ts web/src/pages/wedding/workbench.tsx web/src/pages/wedding/wedding-workbench.css web/src/pages/home/index.tsx web/src/lib/workspace-route-modules.ts web/src/router.tsx
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增制作方专用入口"
```

---

### Task 4: 服务端提示词预览和覆盖准入

**Files:**
- Modify: `backend/internal/app/creation_prompt.go`
- Modify: `backend/internal/app/creation_prompt_test.go`
- Modify: `backend/internal/hostedapi/creation_catalog.go`
- Modify: `backend/internal/hostedapi/creation_catalog_test.go`

**Interfaces:**
- Consumes: existing private `CreationCatalogItem.PayloadJSON`.
- Produces:
  - `resolveCreationPromptPayload(payload, nodeRef, dynastyID, variables) (defaultPrompt string, settings map[string]any, error)`
  - `PreviewCreationPrompt(templateID, nodeRef, dynastyID string, variables map[string]any)`
  - `POST /api/creation-templates/:id/prompt-preview`
  - `creationTemplate.promptOverride`

- [ ] **Step 1: Write the failing Go prompt preview test**

Add to `creation_prompt_test.go`:

```go
func TestPreviewCreationPromptResolvesVideoPromptAndSettings(t *testing.T) {
	payload := map[string]any{
		"dynastyCatalog": []any{map[string]any{
			"id": "tang",
			"videoPrompt": "0-2s: lift the veil. 2-5s: share the cup. 5-8s: orbit.",
			"videoSettings": map[string]any{"duration": float64(8), "ratio": "16:9"},
		}},
		"canvas": map[string]any{"dynamicNodes": []any{map[string]any{
			"ref": "dynastyVideo",
			"metadata": map[string]any{
				"promptTemplate": "{{dynasty.videoPrompt}} Story: {{customDesc}}.",
				"allowProducerPromptOverride": true,
			},
		}}},
	}
	prompt, settings, err := resolveCreationPromptPayload(payload, "dynastyVideo", "tang", map[string]any{
		"customDesc": "西湖相识",
	})
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "0-2s: lift the veil. 2-5s: share the cup. 5-8s: orbit. Story: 西湖相识." {
		t.Fatalf("prompt = %q", prompt)
	}
	if settings["duration"] != float64(8) || settings["ratio"] != "16:9" {
		t.Fatalf("settings = %#v", settings)
	}
}
```

- [ ] **Step 2: Run the test to verify failure**

Run:

```powershell
cd D:\画布\BeefTV\backend
go test ./internal/app -run TestPreviewCreationPromptResolvesVideoPromptAndSettings -count=1
```

Expected: FAIL because `resolveCreationPromptPayload` is undefined.

- [ ] **Step 3: Extract prompt resolution into one pure helper**

Move the variable lookup, personal story injection, dynasty lookup and template replacement from `applyCreationPrompt` into:

```go
func resolveCreationPromptPayload(
	payload map[string]any,
	nodeRef string,
	dynastyID string,
	variables map[string]any,
) (string, map[string]any, error)
```

The helper returns the resolved prompt and the selected dynasty `videoSettings`. `applyCreationPrompt` must call this helper so preview and generation cannot drift.

- [ ] **Step 4: Add prompt override admission**

In `applyCreationPrompt`, read optional `reference["promptOverride"]`. If it is a string with trimmed content:

- Require the selected node metadata `allowProducerPromptOverride` to equal `true`.
- Reject overrides longer than 12000 runes.
- Use the override as the final prompt.
- Continue to redact `promptOverride` through the existing safe reference rewriting.

Add a test that a non-empty override is used and a portrait node rejects override.

- [ ] **Step 5: Add the HTTP preview route**

Add `PreviewCreationPrompt` to the hosted API service interface and route:

```go
r.POST("/creation-templates/:id/prompt-preview", func(c *gin.Context) {
    var input struct {
        NodeRef string `json:"nodeRef"`
        DynastyID string `json:"dynastyId"`
        Variables map[string]any `json:"variables"`
    }
    if err := c.ShouldBindJSON(&input); err != nil {
        writeError(c, kernel.BadAuthRequest("提示词预览参数格式错误"))
        return
    }
    result, err := svc.PreviewCreationPrompt(c.Param("id"), input.NodeRef, input.DynastyID, input.Variables)
    if err != nil {
        writeError(c, err)
        return
    }
    writeOK(c, result)
})
```

The handler response contains `defaultPrompt`, `nodeRef`, `dynastyId`, and `settings`.

- [ ] **Step 6: Add handler tests and run the backend package**

Add tests for:

- Valid `dynastyVideo` preview.
- Missing template.
- Disabled template.
- Missing dynasty.
- Non-workflow template.

Run:

```powershell
cd D:\画布\BeefTV\backend
go test ./internal/app ./internal/hostedapi -run 'CreationPrompt|CreationCatalog' -count=1
```

Expected: all selected tests PASS.

- [ ] **Step 7: Commit**

```powershell
cd D:\画布\BeefTV
git add backend/internal/app/creation_prompt.go backend/internal/app/creation_prompt_test.go backend/internal/hostedapi/creation_catalog.go backend/internal/hostedapi/creation_catalog_test.go
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 支持视频提示词预览与覆盖"
```

---

### Task 5: 前端提示词 API 和覆盖状态

**Files:**
- Create: `web/src/lib/wedding/prompt.ts`
- Create: `web/test/wedding-prompt.test.ts`
- Modify: `web/src/services/api/creation-templates.ts`

**Interfaces:**
- Consumes: `POST /api/creation-templates/:id/prompt-preview` and `WeddingProductionState`.
- Produces:
  - `previewCreationPrompt(templateId, input, signal?)`
  - `weddingVideoPrompt(state, dynastyId, defaultPrompt): string`
  - `setWeddingPromptOverride(state, dynastyId, prompt, now): WeddingProductionState`
  - `clearWeddingPromptOverride(state, dynastyId, now): WeddingProductionState`

- [ ] **Step 1: Write the failing prompt state tests**

```ts
import { describe, expect, test } from "bun:test";
import {
    clearWeddingPromptOverride,
    setWeddingPromptOverride,
    weddingVideoPrompt,
} from "@/lib/wedding/prompt";
import { createWeddingProductionState } from "@/lib/wedding/production-state";

const state = createWeddingProductionState({
    template: { id: "t", key: "cross-era-wedding-volc", version: "1" },
    source: {
        groomPhotoNodeId: "g", bridePhotoNodeId: "b", maleName: "男", femaleName: "女",
        story: "", consentConfirmed: true, ratio: "16:9", resolution: "720p",
    },
    dynasties: [{ id: "tang", name: "唐", order: 0, portraitMaleNodeId: "pm", portraitFemaleNodeId: "pf", sceneNodeId: "s", videoNodeId: "v" }],
    now: "2026-10-25T00:00:00.000Z",
});

test("override wins over the template prompt and invalidates video", () => {
    const changed = setWeddingPromptOverride(state, "tang", "edited prompt", "2026-10-25T01:00:00.000Z");
    expect(weddingVideoPrompt(changed, "tang", "default prompt")).toBe("edited prompt");
    expect(changed.dynasties[0].statuses.video).toBe("stale");
});

test("clearing override returns to default prompt", () => {
    const edited = setWeddingPromptOverride(state, "tang", "edited", "2026-10-25T01:00:00.000Z");
    const cleared = clearWeddingPromptOverride(edited, "tang", "2026-10-25T02:00:00.000Z");
    expect(weddingVideoPrompt(cleared, "tang", "default prompt")).toBe("default prompt");
});
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```powershell
cd D:\画布\BeefTV\web
bun test test/wedding-prompt.test.ts
```

Expected: FAIL because `prompt.ts` does not exist.

- [ ] **Step 3: Implement prompt state and API**

Add the API function:

```ts
export type CreationPromptPreview = {
    defaultPrompt: string;
    nodeRef: string;
    dynastyId: string;
    settings: Record<string, unknown>;
};

export function previewCreationPrompt(
    templateId: string,
    input: { nodeRef: "dynastyVideo"; dynastyId: string; variables: Record<string, unknown> },
    signal?: AbortSignal,
) {
    return http.post<CreationPromptPreview>(
        `/creation-templates/${encodeURIComponent(templateId)}/prompt-preview`,
        input,
        { signal },
    );
}
```

Implement prompt state functions through `invalidateWeddingDynasty(..., "prompt")`.

- [ ] **Step 4: Run tests and typecheck**

Run:

```powershell
cd D:\画布\BeefTV\web
bun test test/wedding-prompt.test.ts test/wedding-production-state.test.ts
bun run typecheck
```

Expected: tests PASS and typecheck PASS.

- [ ] **Step 5: Commit**

```powershell
cd D:\画布\BeefTV
git add web/src/lib/wedding/prompt.ts web/test/wedding-prompt.test.ts web/src/services/api/creation-templates.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 接入提示词预览状态"
```

---

### Task 6: 七阶段工作台壳与视频提示词检查器

**Files:**
- Modify: `web/src/pages/wedding/workbench.tsx`
- Modify: `web/src/pages/wedding/wedding-workbench.css`
- Create: `web/src/pages/wedding/workbench-stage-rail.tsx`
- Create: `web/src/pages/wedding/workbench-top-bar.tsx`
- Create: `web/src/pages/wedding/video-prompt-inspector.tsx`

**Interfaces:**
- Consumes: `WeddingProductionState`, `previewCreationPrompt`, prompt state functions, `persistCanvasDocument`.
- Produces: usable desktop shell with seven stages and a functional video prompt inspector for the selected dynasty.

- [ ] **Step 1: Add a server-render component test**

Create `web/test/wedding-workbench-render.test.tsx`:

```tsx
import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { WorkbenchStageRail } from "@/pages/wedding/workbench-stage-rail";
import { createWeddingProductionState } from "@/lib/wedding/production-state";

test("renders all seven producer stages", () => {
    const state = createWeddingProductionState({
        template: { id: "t", key: "cross-era-wedding-volc", version: "1" },
        source: {
            groomPhotoNodeId: "g", bridePhotoNodeId: "b", maleName: "男", femaleName: "女",
            story: "", consentConfirmed: true, ratio: "16:9", resolution: "720p",
        },
        dynasties: [{ id: "tang", name: "唐", order: 0, portraitMaleNodeId: "pm", portraitFemaleNodeId: "pf", sceneNodeId: "s", videoNodeId: "v" }],
        now: "2026-10-25T00:00:00.000Z",
    });
    const html = renderToStaticMarkup(<WorkbenchStageRail state={state} activeStage="videos" onSelect={() => undefined} />);
    expect(html).toContain("项目设定");
    expect(html).toContain("视频导演");
    expect(html).toContain("成片交付");
});
```

- [ ] **Step 2: Run the test to verify failure**

Run:

```powershell
cd D:\画布\BeefTV\web
bun test test/wedding-workbench-render.test.tsx
```

Expected: FAIL because the component does not exist.

- [ ] **Step 3: Implement the stage rail and top bar**

Use `lucide-react` icons for completed, active, blocked and locked stages. Render the seven Chinese labels in the spec order. The rail must use `aria-current="step"` for the active stage and a keyboard-visible focus style.

The top bar shows project name, saved state from the canvas sync status service, a read-only stage state, and a link back to `/canvas/:id`.

- [ ] **Step 4: Implement the video prompt inspector**

The inspector:

- Receives `templateId`, `dynastyId`, variables, effective prompt and `onSave`.
- Loads `previewCreationPrompt` with TanStack Query.
- Shows loading, error and empty states.
- Shows the default prompt in a read-only comparison panel and the effective prompt in an editable textarea.
- Saves `promptOverride` through `setWeddingPromptOverride` and `persistCanvasDocument`.
- Restores default through `clearWeddingPromptOverride` and saves the new production state.
- Disables saving empty or over-12000-character prompts.
- Never renders a generate button in this plan.

- [ ] **Step 5: Compose the workbench page**

`workbench.tsx` loads the canvas project, reads `weddingProduction`, renders the rail and top bar, and shows the video inspector for the selected dynasty. Other stage content displays real state summaries from the production state, not fabricated generation data.

- [ ] **Step 6: Add browser coverage**

Create `web/test/wedding-workbench.browser.test.ts` using the existing Chromium harness. Assert:

- Route `/wedding/:id` opens the workbench.
- Seven stage labels are visible.
- Prompt preview loads.
- Editing and saving shows the new effective prompt after reload.
- Restore default removes the override.

Run:

```powershell
cd D:\画布\BeefTV\web
bun test test/wedding-workbench.browser.test.ts
```

Expected: all browser assertions PASS.

- [ ] **Step 7: Run build and lint**

Run:

```powershell
cd D:\画布\BeefTV\web
bun run typecheck
bun run build
bun run lint
```

Expected: typecheck, build and lint PASS.

- [ ] **Step 8: Commit**

```powershell
cd D:\画布\BeefTV
git add web/src/pages/wedding web/test/wedding-workbench-render.test.tsx web/test/wedding-workbench.browser.test.ts
git commit -m "feat(wedding-workbench): 跨时空婚礼电影 - 新增制作方工作台与提示词检查器"
```

---

### Task 7: 规格回归与阶段交付说明

**Files:**
- Modify: `docs/superpowers/specs/2026-10-25-cross-era-wedding-producer-workbench-design.md`
- Create: `docs/superpowers/plans/2026-10-25-cross-era-wedding-production-stages.md`

**Interfaces:**
- Consumes: completed foundation and prompt chain.
- Produces: explicit next plan for portrait, scene and video generation stages.

- [ ] **Step 1: Verify spec coverage for the foundation phase**

Check and record in the plan:

- `weddingProduction` persists through the canvas.
- Home cross-era card opens `/wedding/:canvasId`.
- Seven stages render from persisted state.
- Full video prompt preview loads from the backend.
- Prompt override saves, survives reload, and restores to default.

- [ ] **Step 2: Run the complete foundation test set**

Run:

```powershell
cd D:\画布\BeefTV\web
bun test test/wedding-production-state.test.ts test/wedding-template-state.test.ts test/wedding-routing.test.ts test/wedding-prompt.test.ts test/wedding-workbench-render.test.tsx test/wedding-workbench.browser.test.ts
bun run typecheck

cd D:\画布\BeefTV\backend
go test ./internal/app ./internal/hostedapi -run 'CreationPrompt|CreationCatalog' -count=1
```

Expected: all commands PASS.

- [ ] **Step 3: Write the next implementation plan**

Create `docs/superpowers/plans/2026-10-25-cross-era-wedding-production-stages.md` covering:

- Setup and dynasty editing.
- Portrait batch generation and selection.
- Scene generation and QC override.
- Video generation with prompt override and version comparison.

The next plan must reference the exact functions and types produced by this plan and use the same TDD and commit structure.

- [ ] **Step 4: Commit**

```powershell
cd D:\画布\BeefTV
git add docs/superpowers/specs/2026-10-25-cross-era-wedding-producer-workbench-design.md docs/superpowers/plans/2026-10-25-cross-era-wedding-production-stages.md
git commit -m "docs(wedding-workbench): 跨时空婚礼电影 - 记录基础阶段验收与后续计划"
```

## Self-Review

### Spec Coverage

This plan implements the spec sections:

- 产品结构
- 七阶段导航骨架
- 状态模型
- 提示词控制
- 画布持久化
- 专用路由
- 视频提示词预览 API
- 对应单元、后端和浏览器测试

The spec sections for real portrait, scene, video generation, packaging and delivery are intentionally assigned to the next plan because they depend on the state and prompt interfaces delivered here. The next plan file is a required deliverable of Task 7.

### Placeholder Scan

The plan contains no unresolved markers, unspecified error handling, or references to undefined functions. Every task names exact files, exported interfaces, test commands and commit messages.

### Type Consistency

- `WeddingProductionState.weddingProduction` is the only top-level production field.
- `promptOverride` is stored on each `WeddingDynastyState`.
- `setWeddingPromptOverride` and `clearWeddingPromptOverride` both return a complete new state.
- Backend field names are `nodeRef`, `dynastyId`, `promptOverride`, `defaultPrompt`, and `settings`.
- Route id parameter is `canvasId`.
