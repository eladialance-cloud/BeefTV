# BeefTV 托管运行基座 Plan 3：资源存储端口切换（对象存储真正承载读写）

> **给执行者**：本计划承接 `2026-10-16-beeftv-p0-storage-config.md` 末尾的「下一步：资源存储端口切换」。
> 目标是把「新上传素材真正写进对象存储、并能读回、能删除」跑通，同时不破坏桌面形态。

## 背景：三处现有约束（大白话）

1. **门卫不认云存储**：`internal/asset/open.go` 的 `OpenOwnedRange` / `PrepareOwnedDelivery` 看到 `resource.Provider != "local"` 直接返回 `ResourceNotLocal()`。不放开这里，写进 OSS 的素材读不出来。
2. **端口形状太窄**：`asset.BlobStore` 是 `Write/Open/Delete/Exists`，只认 `objectKey`，不认「这条资源在哪个存储位置」。多存储路由需要先知道「去哪读」。
3. **写锁的钥匙是磁盘路径**：`internal/asset/service.go` 的 `writeSpace()` 靠类型断言 `*FileStore` 取串行化锁标识。文件到云上后这把钥匙失效，必须由「存储路由」自己给出稳定身份。

另：`model.Resource.StorageSettingID / Provider / Endpoint / Bucket` 字段已存在，但 `WriteObject` 目前把它们固定写成空串/`local`，切换时要按解析结果回填。

## 目标与非目标

**本计划交付**

- 新写入的素材落到「默认存储位置」：桌面 = 本地 `FileStore`；托管 = 后台配置的默认对象存储（无默认配置时回落本地磁盘）。
- 已落库的素材（含历史 `Provider=local`）按行上记录的 `StorageSettingID/Provider` 解析并读回、删除。
- 桌面形态行为完全不变，且 `TestDesktopDependencyBoundary`、`TestLocalSourceExcludesHostedContentVerticals` 继续通过。

**明确不在本计划内**

- 历史素材跨存储迁移（老对象留在原地，按记录读取）。
- 匿名公开下载链接在对象存储上的 CDN 直连 / 重定向（现走本机代理流式；S3 直连签名 URL 单列 Plan 4）。
- 多实例跨进程写锁（现锁为进程内，托管多副本下的分布式锁单列）。

## 架构决策

**新增 `asset.BlobResolver` 端口，`BlobStore` 形状保持不变。**

- `BlobStore` 仍是「某一个存储位置」的字节读写（`FileStore`、`S3BlobStore` 都不用改）。
- `asset.Service` 不再直接持有单个 store，而是持有 resolver：
  - 写入：`DefaultTarget()` 拿到落库身份 + store，把 `SettingID/Provider/Endpoint/Bucket` 写进资源行。
  - 读取/删除：用资源行上的字段还原 `BlobTarget`，`Resolve(target)` 拿回 store。
- `WriteSpace()` 由 resolver 提供稳定锁身份，跨 Service 实例共享：本地 = 规范化数据目录；对象存储 = `s3://endpoint/bucket/prefix`。

**桌面与托管的分流不引入新 import 边**：resolver 的实现分别在 `asset`（单位置）与 `storageconfig`（多位置）；`asset` 不 import `storageconfig`，`cmd/desktop` 依赖图不含 AWS SDK。

## Task 1: `asset` 单位置 resolver + 写锁身份外置

**Files:**
- Modify: `backend/internal/asset/ports.go`
- Modify: `backend/internal/asset/file_store.go`
- Create: `backend/internal/asset/blob_resolver.go`
- Test: `backend/internal/asset/blob_resolver_test.go`

**Interfaces:**

```go
type BlobTarget struct {
    SettingID string
    Provider  string // "local" | "s3" | ...
    Endpoint  string
    Bucket    string
}

type BlobResolver interface {
    DefaultTarget() (BlobTarget, error)
    Resolve(target BlobTarget) (BlobStore, error)
    WriteSpace() string
}
```

- `FileStore` 暴露 `WriteSpace() string`（现 `writeSpace()` 改名，行为不变）。
- `Dependencies` 新增 `Resolver BlobResolver`；`Blobs BlobStore` 保留为桌面简写，`Resolver == nil && Blobs != nil` 时包成 `SingleLocation(Blobs)`。
- `SingleLocation(store BlobStore) BlobResolver`：`DefaultTarget()` 返回 `{Provider:"local"}`；`Resolve` 对 `Provider` 为空或 `"local"` 放行、否则 `ResourceNotLocal()`；`WriteSpace()` 透传可命名 store。

- [x] **Step 1: 写失败测试**：`SingleLocation` 的默认目标、非本地 provider 拒绝、`WriteSpace` 与 store 别名一致；`Service` 用 `Dependencies{Blobs: ...}`（不传 Resolver）时 `writeSpace()` 仍等于 `FileStore.WriteSpace()`。
- [x] **Step 2: 运行确认失败**。
- [x] **Step 3: 实现**。
- [x] **Step 4: 运行确认通过**（`go test ./internal/asset/`）。
- [x] **Step 5: 提交**：`feat(backend): 资源存储 - 新增单位置 Blob 解析端口与写锁身份外置`。

## Task 2: `asset.Service` 全面走 resolver

**Files:**
- Modify: `backend/internal/asset/service.go`（字段 + `writeSpace()` + `validateListedResource`）
- Modify: `backend/internal/asset/store.go`（`WriteObject` 落库身份）
- Modify: `backend/internal/asset/open.go`（按 target 解析 store，放开 `provider != local` 硬门槛）
- Modify: `backend/internal/asset/delete.go`（`DeleteLocalObject` 按 target 解析）
- Modify: `backend/internal/asset/recover.go`（`Exists` 按 target 解析）

**规则**

- `localStorage == true`（桌面）时**保留**「非本地资源一律拒绝」的既有闸门与报错文案。
- `localStorage == false`（托管）时，按行上 target 解析；`Provider` 为空视为 `local`（兼容历史行）。
- `WriteObject`：`DefaultTarget()` → 落库 `SettingID/Provider/Endpoint/Bucket` → `Resolve(target).Write(objectKey, body)`。
- `RetryOwned` 里「provider 变了就换 objectKey」的历史分支按 target 判断，语义不变。

- [x] **Step 1: 写失败测试**：用内存假 resolver + 假 store 覆盖「托管下 provider=s3 的行可读回」「桌面下同一行被拒」「写入把 storageSettingId/provider 落库」。
- [x] **Step 2: 运行确认失败**。
- [x] **Step 3: 实现**。
- [x] **Step 4: 运行全部 `internal/asset/` 测试确认不回归**。
- [x] **Step 5: 提交**：`feat(backend): 资源存储 - 读写删按存储位置路由`。

## Task 3: `storageconfig` 多位置路由 resolver

**Files:**
- Create: `backend/internal/storageconfig/router.go`
- Test: `backend/internal/storageconfig/router_test.go`

**行为**

- `NewRouter(repo Repository, dataDir string, local BlobStoreFactory) *Router`：
  - `DefaultTarget()`：取 `is_default=true` 且 `enabled=true` 且已通过测试的配置 → 解密密钥 → 返回 S3 target；没有可用默认配置时回落本地 target（`Provider:"local"`）。
  - `Resolve(target)`：`Provider` 为空/`local` → 本地 store；否则按 `SettingID` 载入配置、解密、按缓存返回/新建 `S3BlobStore`。
  - 禁用或缺失的配置返回明确错误，**不静默回落**（避免把 A 桶的对象从 B 桶读）。
  - `WriteSpace()`：本地回落时为规范化数据目录；对象存储时为 `s3://endpoint/bucket/prefix`。
- store 按 `SettingID + UpdatedAt` 缓存，配置更新后失效。

- [x] **Step 1: 写失败测试**：无默认配置回落本地；有默认配置走 S3 目标并带上 endpoint/bucket；disabled 配置解析报错；同一配置两次 Resolve 复用实例；配置更新后重建。
- [x] **Step 2: 运行确认失败**。
- [x] **Step 3: 实现**。
- [x] **Step 4: 运行确认通过**。
- [x] **Step 5: 提交**：`feat(backend): 对象存储 - 新增多存储位置路由解析`。

## Task 4: 组装到托管入口

**Files:**
- Modify: `backend/internal/app/resource.go` 或新增 `backend/internal/app/resource_resolver.go`
- Modify: `backend/cmd/server/main.go`

- `app.Service` 暴露 `SetResourceBlobResolver(localasset.BlobResolver) error`：仅在组装期调用，替换 `s.assets`。
- `cmd/server` 的 `registerHostedRoutes` 里构造 `storageconfig.NewRouter(...)` 并注入。
- 桌面路径不调用该 setter，行为与依赖图不变。

- [x] **Step 1: 写失败测试**：注入后 `Service` 的资源写入走注入的 store（用假 resolver）。
- [x] **Step 2: 运行确认失败**。
- [x] **Step 3: 实现**。
- [x] **Step 4: `go build ./...` + `go test ./internal/app/ ./internal/bootstrap/` 通过**。
- [x] **Step 5: 提交**：`feat(backend): 托管入口 - 注入多存储路由解析`。

## Task 5: 端到端回归（进程内 S3 桩）

**Files:**
- Test: `backend/internal/app/resource_storage_route_test.go`

- 用 `s3blobstore_test.go` 已有的进程内 S3 桩思路，构造「S3 默认位置」的 router，走完整链路：上传 → 行上落 `provider=s3` 与 `storageSettingId` → Range 读回字节一致 → 删除后对象消失。

- [x] **Step 1: 写失败测试**。
- [x] **Step 2: 运行确认失败**。
- [x] **Step 3: 补齐实现缺口**。
- [x] **Step 4: `go test ./internal/asset/ ./internal/storageconfig/ ./internal/app/` 全绿**。
  - 实际：新增用例全绿；`internal/asset` 在本机仍保留两类**环境性**失败（Windows SQLite 临时库句柄未释放导致 `TempDir RemoveAll cleanup`，以及无 symlink 创建特权导致 `TestAliasedFileStoresShareWriteLock`），与本改动无关。
- [x] **Step 5: 提交**：`test(backend): 资源存储 - 补齐对象存储读写删端到端回归`。

## 自检结论

- **Spec 覆盖**：覆盖设计文档「对象存储配置入口」之后真正承载用户资源读写的落地部分。
- **已知未验证项**：本机无 Docker / 无真实对象存储，跨进程、真实 OSS 权限与分片上传只能在真实环境验证；PostgreSQL 侧迁移维持 Linux/CI 验证。

---

## 已交付（2026-10-16）

改动落在 `internal/asset`、`internal/storageconfig`、`internal/app`、`internal/bootstrap`、`cmd/server`，共新增/修改约 20 个文件。

**端口**：`asset.BlobTarget` + `asset.BlobResolver`（`DefaultTarget / Resolve / LocalOnly / WriteSpace`）。`BlobStore` 形状未变，`FileStore` 只把 `writeSpace()` 改名导出为 `WriteSpace()`。

**本地形态**：`SingleLocation(store)` 包装单个 `FileStore`。它 `Resolve` 非 local 目标时返回 `ResourceNotLocal()`，所以「桌面工作区绝不能读到云端行」这条旧规则由解析器统一承担，不再散落在读写删各处。

**托管形态**：`storageconfig.Router` 实现同一端口。默认写入走后台配置的默认存储（未配置时回落本机）；已落库的行按 `StorageSettingId/Provider/Endpoint/Bucket` 解析回原存储。默认存储被停用或未通过测试时**报错而不是静默回落**，避免悄悄把服务器磁盘写满。

**读写删三路放开**：`OpenOwnedRange`、`PrepareOwnedDelivery`、`deleteTrustedObject`、`objectPresent`、`WriteObject`、`validateListedResource` 全部改为经解析器取存储；`WriteObject` 现在把落库地点写进资源行。

**注入点**：`bootstrap.Config.HostedBlobResolver` 由 `cmd/server` 提供，`bootstrap` 在构造 `app.Service` 之前注入，保证 `localapp` 拿到的资源域已经是路由形态。桌面入口不设该钩子，`TestDesktopDependencyBoundary` 与 `TestLocalSourceExcludesHostedContentVerticals` 均通过。

**测试**：`internal/asset/blob_resolver_test.go`（单位置语义 / 写锁身份 / 路由写读删 + Range）、`internal/storageconfig/router_test.go`（回落、默认目标、不可用默认、缓存与失效、未知/停用拒绝）、`internal/app/resource_storage_route_test.go`（注入后字节真进外部存储、桌面仍拒绝云端行）。

## 本计划仍未覆盖（下一步）

1. **浏览器出口的 CDN 直连**：现在托管资源仍经本机 `/resources/:id/file` 代理（S3 后端会先落到临时文件）。`PrepareOwnedDelivery` 还没有返回签名直连 `RedirectURL`，大文件带宽成本高。
2. **`BlobStore.Open` 的端口形状**：仍是 `*os.File`，所以 S3 读取必经「下载到临时文件」。真正流式 Range 需把端口改宽为 `io.ReadSeekCloser` + `Stat`。
3. **跨进程写锁**：`WriteSpace()` 只是进程内锁表身份。托管多副本部署下同一 uploadKey 的并发写需要分布式锁或数据库唯一约束兜底。
4. **历史资源跨存储迁移**：老对象留在原存储，按记录读取；批量搬迁尚未立项。
