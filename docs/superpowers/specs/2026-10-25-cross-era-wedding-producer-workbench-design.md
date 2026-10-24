# 跨时空婚礼电影制作方工作台设计

日期：2026-10-25
状态：设计已整理，待用户审阅后进入实施计划
范围：VisiDeep Web 跨时空婚礼电影制作入口、制作方工作台、生产状态、视频提示词控制与现有生成链适配

## 目标

把现有通用工作流画布改造成只面向制作方的完整婚礼电影制作台。制作方可以在同一个项目中完成照片与设定、朝代规划、定妆照、场景首帧、视频导演、包装制作和成片交付，并始终看到当前阶段、失败原因、版本、提示词和实际生成参数。

工作台必须执行真实生成链路，不能把静态节点、模拟按钮或前端状态演示描述成已经生成。

## 非目标

- 不建设新人端、客户审片页、远程确认或客户权限。
- 不重写 Seedream、Seedance、上传、资源存储、生成任务和模型渠道适配体系。
- 不把通用画布继续做成男女主角自助向导。
- 不公开 API Key、渠道密钥或服务端私有凭据。
- 不在本轮修改漫剧工作室或漫剧老李流程。

## 现有依据

- 工作流模板来源：`scripts/build-cross-era-wedding-workflow-json.mjs`
- 画布模板转换：`web/src/lib/canvas/creation-template-workflow.ts`
- 后台私有提示词解析：`backend/internal/app/creation_prompt.go`
- 婚礼生成源：`../generated/smart-open-skills/timeless-wedding-volc`
- 画布项目持久化：`web/src/services/local-workspace-repository.ts`
- 生成任务与本地渲染：`web/src/services/api/`、`web/src/lib/timeline/`、`backend/internal/operations/canvas_render.go`

## 产品结构

跨时空婚礼模板创建后进入专用路由 `/wedding/:canvasId`。普通工作流模板继续进入 `/canvas/:id`。

工作台采用桌面制作布局：

- 左侧固定七阶段导航，显示阶段状态、完成数量和阻塞原因。
- 顶部显示项目名称、保存状态、预览整片和导出工作副本。
- 中央是当前阶段的主要工作区。
- 右侧是当前选中的资产或朝代的检查器，包含提示词、参数、参考素材、质检和版本。
- 画布仍作为底层素材与结果存储，不作为主操作界面。

窄屏下左栏折叠为顶部阶段选择器，中央和检查器纵向排列。页面不得出现横向滚动遮挡操作。

## 七阶段流程

### 1. 项目设定

输入：

- 新郎、新娘原始照片
- 称呼
- 爱情故事和必须保留的细节
- 画幅、目标分辨率、片头片尾、字幕默认值
- 照片使用授权确认

门控：

- 两张照片均完成上传和资源登记
- 授权确认已勾选
- 输出规格合法

变化影响：

- 更换人物照片会让所有朝代肖像、场景、视频和成片失效。
- 修改故事或输出规格会让相关场景、视频和成片失效。

### 2. 朝代规划

输入：

- 选择 2 至 5 个朝代
- 调整朝代播放顺序
- 查看每个朝代主题、故事概述、预计片段时长和连续性要求

门控：

- 朝代数量合法
- 顺序无重复
- 每个朝代的源资产和规格已加载

变化影响：

- 新增朝代只初始化该朝代资产。
- 删除朝代会让最终成片失效。
- 调整顺序只让最终成片失效。

### 3. 定妆照

每个朝代显示男方和女方定妆照：

- 原图与定妆照并排对照
- 人物身份、发型、服饰和识别特征检查
- 单人生成状态、失败原因和重试
- 当前选片和全部历史版本
- 批量生成或逐张生成

门控：

- 每个选中朝代的男女定妆照均已选定
- 每张定妆照均标记为人工确认

变化影响：

- 修改某朝代任一定妆照会让该朝代场景、视频和成片失效。

### 4. 场景首帧

每个朝代显示：

- 双人场景首帧
- 使用的新郎、新娘定妆照
- 场景提示词摘要
- 自动质检结果：双人是否齐全、男女是否像本人、服饰、朝代场景、多余人物、文字水印、构图
- 当前选片、历史版本、单朝重生成

门控：

- 每个选中朝代均选定一张场景首帧
- 硬性质检项全部通过，或制作方明确覆盖失败项并填写原因

变化影响：

- 修改场景首帧会让该朝代视频和成片失效。

### 5. 视频导演

每个朝代显示三段时间分镜 `0-2s`、`2-5s`、`5-8s`。用户可以逐段编辑中文导演稿，但一次视频生成仍按当前朝代的完整时长提交。

右侧检查器包含：

- 最终英文模型提示词
- 恢复朝代默认提示词
- 生成参数：模型档位、时长、画幅、分辨率、seed、生成模式
- 参考素材：男女定妆照和场景首帧
- 预计耗时和积分
- 版本列表、版本对比、选定版本
- 自动质检和人工确认结果

门控：

- 每个选中朝代均选定一个视频版本
- 人工确认视频中的人物、动作、服饰、场景和声音

变化影响：

- 修改提示词会让当前朝代视频和成片失效。
- 重新生成会新增版本，不覆盖已有版本。

### 6. 包装制作

包含：

- 片头、片尾文案和时长
- 每个朝代的标题卡
- 转场、字幕和声音层
- 自动 BGM、上传 BGM 或无 BGM
- 各段视频的最终使用顺序

门控：

- 所有槽位均有有效素材或明确关闭
- BGM 已选定、上传或关闭

变化影响：

- 包装变化只让最终成片失效。

### 7. 成片交付

包含：

- 整片时间线预览
- 本地渲染或现有时间线渲染任务
- 整片质检
- 成片版本
- 下载 MP4
- 导出素材清单、模型参数、seed、提示词版本和质检记录

门控：

- 渲染成功
- 成片完整播放通过
- 制作方完成最终确认

## 状态模型

生产状态保存在画布文档顶层 `weddingProduction`。素材仍保存在画布节点和资源库，生产状态只保存阶段、引用、提示词覆盖、选片和版本关系。

```ts
type WeddingStageId =
    | "setup"
    | "dynasties"
    | "portraits"
    | "scenes"
    | "videos"
    | "packaging"
    | "delivery";

type WeddingItemStatus =
    | "not_started"
    | "ready"
    | "generating"
    | "needs_review"
    | "approved"
    | "failed"
    | "stale";

type WeddingSceneQc = {
    verdict: "pass" | "fail" | "error";
    couplePresent: boolean;
    faceMatchMale: boolean;
    faceMatchFemale: boolean;
    outfitMatch: boolean;
    sceneMatch: boolean;
    extraPerson: boolean;
    watermarkOrText: boolean;
    reason: string;
    suggestion: string;
    checkedAt: string;
};

type WeddingVideoQc = {
    verdict: "pass" | "fail" | "error" | "pending";
    faceStable: boolean;
    outfitStable: boolean;
    actionReadable: boolean;
    extraPerson: boolean;
    watermarkOrText: boolean;
    reason: string;
    checkedAt: string;
};

type WeddingDynastyState = {
    id: string;
    name: string;
    order: number;
    portraitMaleNodeId: string;
    portraitFemaleNodeId: string;
    sceneNodeId: string;
    videoNodeId: string;
    portraitSelection: { male?: string; female?: string };
    sceneSelection?: string;
    videoSelection?: string;
    promptOverride?: string;
    promptOverrideUpdatedAt?: string;
    statuses: Record<"portrait" | "scene" | "video", WeddingItemStatus>;
    qc: {
        scene?: WeddingSceneQc;
        video?: WeddingVideoQc;
        overrides: Partial<Record<"scene" | "video", {
            reason: string;
            acceptedAt: string;
        }>>;
    };
};

type WeddingPackagingState = {
    status: WeddingItemStatus;
    intro: { text: string; durationSeconds: number; enabled: boolean };
    outro: { text: string; durationSeconds: number; enabled: boolean };
    subtitlesEnabled: boolean;
    transitionSeconds: number;
    bgmMode: "generated" | "uploaded" | "none";
    bgmNodeId?: string;
    titleCardNodeIds: string[];
    timelineRevision?: number;
    outputNodeId?: string;
    approvedAt?: string;
};

type WeddingActivityEvent = {
    id: string;
    type:
        | "stage_changed"
        | "asset_generated"
        | "asset_selected"
        | "asset_approved"
        | "prompt_changed"
        | "asset_staled"
        | "render_finished"
        | "error";
    stage: WeddingStageId;
    dynastyId?: string;
    message: string;
    createdAt: string;
};

type WeddingProductionState = {
    schemaVersion: 1;
    template: { id: string; key: string; version: string };
    activeStage: WeddingStageId;
    stages: Record<WeddingStageId, {
        status: WeddingItemStatus;
        updatedAt: string;
        approvedAt?: string;
    }>;
    source: {
        groomPhotoNodeId: string;
        bridePhotoNodeId: string;
        maleName: string;
        femaleName: string;
        story: string;
        consentConfirmed: boolean;
        ratio: string;
        resolution: string;
    };
    dynasties: WeddingDynastyState[];
    packaging: WeddingPackagingState;
    activity: WeddingActivityEvent[];
};
```

状态规则：

- 同一阶段内单个朝代失败不阻塞其他朝代。
- `stale` 表示上游已变化，必须重新生成或重新确认。
- 进入下一阶段前检查当前阶段必需资产，不满足时显示具体阻塞项。
- 制作方可以自由查看任意阶段，返工会直接更新受影响范围。
- 阶段状态从运行时结果计算，不能由前端静态写入 `approved`。

## 提示词控制

视频提示词分为两个字段：

- `defaultPrompt`：根据后台模板、朝代和项目变量生成的默认最终提示词。
- `promptOverride`：制作方直接编辑并保存的最终提示词。

最终提交规则：

- `promptOverride` 非空时，后台直接使用该文本。
- `promptOverride` 为空时，后台使用已解析的模板提示词。
- 恢复默认会清空当前朝代的 `promptOverride`。
- 修改提示词后，当前视频节点和最终成片标记为 `stale`。
- 默认提示词和编辑后的最终提示词均由制作方工作台可见。
- 任务日志和错误信息不得记录渠道密钥或认证 Header。

新增制作方提示词预览接口：

```text
POST /api/creation-templates/:id/prompt-preview
{
  "nodeRef": "dynastyVideo",
  "dynastyId": "tang",
  "variables": { ... }
}

Response:
{
  "defaultPrompt": "...",
  "nodeRef": "dynastyVideo",
  "dynastyId": "tang",
  "settings": {
    "duration": 8,
    "ratio": "16:9",
    "resolution": "720p"
  }
}
```

接口只接受已启用工作流模板，只返回请求节点和朝代的解析结果。提示词覆盖通过现有生成请求的 `input.metadata.creationTemplate.promptOverride` 传入。服务端只在节点允许覆盖时接受该字段，长度限制为 1 至 12000 个字符。

## 持久化

- `CanvasProject` 增加可选顶层字段 `weddingProduction`。
- `InitialCanvasContent`、画布文档持久化 patch 和本地同步 patch 增加 `weddingProduction`。
- 后端现有画布文档合并会保留未知顶层字段，不新增数据库表。
- `weddingProduction` 使用画布现有 revision、历史版本、自动保存和冲突处理。
- 每次提示词、选片、阶段和版本变化都通过现有画布持久化边界保存。
- 阶段状态与节点结果必须来自同一画布 revision，避免出现视频已生成但工作台仍显示旧状态的跨版本组合。

## 生成与包装

- 肖像、场景和视频继续使用现有画布节点生成任务。
- 批量生成复用现有任务提交、轮询、失败重试、资产物化和任务事件。
- 工作台不维护第二套任务执行器。
- 视频生成请求包含模板引用、朝代、变量和可选提示词覆盖。
- 预览档和正式档只在管理后台配置了对应视频模型时可选；未配置时只显示正式档。
- 包装阶段创建 `TimelineProject`，使用现有时间线本地渲染能力输出 MP4。
- 自动 BGM 使用现有音频生成能力生成一条与朝代情绪顺序相符的音乐；上传 BGM 使用资源库音频；无 BGM 时明确关闭声音层。

## API 与前端边界

新增以下业务 API：

- `POST /api/creation-templates/:id/prompt-preview`

复用以下现有能力：

- 工作流模板公开列表：`GET /api/creation-templates?kind=workflow`
- 画布项目创建、读取、提交、历史与恢复
- 资源上传和资源引用
- 画布图像、视频、音频生成任务
- 时间线编译和本地渲染
- 生成任务事件与资产物化

前端新目录：

```text
web/src/pages/wedding/
  workbench.tsx
  wedding-workbench.css
  workbench-stage-rail.tsx
  workbench-top-bar.tsx
  setup-stage.tsx
  dynasty-stage.tsx
  portrait-stage.tsx
  scene-stage.tsx
  video-stage.tsx
  packaging-stage.tsx
  delivery-stage.tsx
  video-prompt-inspector.tsx
  version-strip.tsx

web/src/lib/wedding/
  types.ts
  production-state.ts
  production-state.test.ts
  template-state.ts
  template-state.test.ts
  prompt.ts
  prompt.test.ts
  generation.ts
  packaging.ts

web/src/services/api/
  wedding-workbench.ts
```

`workbench.tsx` 只负责页面组合和路由参数。阶段组件负责本阶段交互。`production-state.ts` 负责纯状态转换、门控和失效计算。API 模块只通过 `http` 调用后端。

## 视觉规范

- 采用安静的影视制作工作台风格，不使用营销页结构。
- 页面为全幅工作区，不使用大面积卡片堆叠。
- 中央工作区优先展示素材、分镜和操作。
- 右侧检查器固定用于提示词、参数、版本和质检。
- 按钮使用 Lucide 图标或明确的动作文字。
- 主要颜色为中性深色底色，琥珀色表示当前制作动作，青绿色表示已验证，红色只用于错误。
- 所有文本在 1280px 和窄屏下不得溢出，提示词区域可滚动，不遮挡生成操作。
- 尊重 `prefers-reduced-motion`，键盘焦点保留可见状态。

## 错误处理

- 上传失败显示具体阶段和文件，不写入半成品资源引用。
- 生成失败保留当前版本和历史，显示供应商错误分类和可重试动作。
- 提示词预览失败时禁止静默使用空白提示词。
- 提示词覆盖保存失败时不把本地草稿标记为已保存。
- 画布 revision 冲突时停止覆盖，展示重新加载入口。
- 未配置预览模型时禁用预览档并说明原因。
- FFmpeg 或渲染不可用时保留时间线，可重新提交渲染。
- 资源失效时标记对应节点和下游阶段为 `stale`，不自动替换素材。

## 测试

单元测试：

- 七阶段状态转换和门控
- 朝代增加、删除、排序
- 照片、肖像、场景、提示词、包装变化导致的精确失效范围
- 视频提示词默认值、覆盖、恢复和长度校验
- 模板变量展开和默认提示词预览

后端测试：

- 提示词预览接口拒绝不存在、停用或非工作流模板
- 提示词覆盖只在允许节点生效
- 覆盖不会进入持久化日志中的敏感 Header
- 现有画布文档合并保留 `weddingProduction`

前端测试：

- 跨时空婚礼模板创建后进入 `/wedding/:canvasId`
- 普通模板继续进入 `/canvas/:id`
- 视频导演阶段显示完整提示词并保存覆盖
- 失败、空态、加载态和 revision 冲突
- 版本切换和恢复默认

浏览器验证：

- 1440px、1280px 和窄屏三档截图
- 七阶段导航、提示词编辑、横向滚动与纵向滚动
- 不出现文本遮挡、按钮溢出或不可操作区域
- 真实任务入口能进入现有任务轮询和资产物化链路

构建验证：

- `cd web && bun run build`
- 相关 `bun test` 和 `bun run lint`
- `cd backend && go test ./...`

## 验收标准

- 制作方从首页跨时空婚礼卡片进入专用工作台。
- 七阶段可逐步推进，也可返回任意阶段返工。
- 制作方能查看和直接编辑每个朝代的最终英文视频提示词。
- 提示词覆盖真实进入生成请求，恢复默认后重新使用服务端模板。
- 更换上游资产只会让准确的下游范围失效。
- 肖像、场景和视频生成使用现有真实任务链，不出现假生成按钮。
- 包装阶段使用现有时间线渲染产出真实 MP4。
- 页面刷新、重新进入和 revision 冲突后状态可恢复。
- 不向客户端或任务记录泄漏 API Key、渠道密钥和认证 Header。
- 前端构建、专项测试、后端测试和浏览器验证通过。
