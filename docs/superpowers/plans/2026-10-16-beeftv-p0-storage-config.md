# BeefTV P0 对象存储配置与后台入口实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让管理员能在后台**添加**一条平台级对象存储配置（`storage_locations`），密钥加密落库、只回显掩码，并通过连通性测试后才可设为默认。

**Architecture:** 新增托管侧 `storage_locations` 模型与仓储；`internal/storageconfig` 承载校验、加密、掩码、默认唯一性与连通性测试；管理后台 HTTP 入口放在新包 `internal/hostedapi`，**只在 `ProfileServer` 注册**，桌面路由图保持不变（保住「Go linker 丢弃托管处理器」的性质）。

**Tech Stack:** Go 1.25、GORM、Gin、`internal/localcrypto`（AES-GCM）、`internal/model.AdminAuditEvent`（审计）、aws-sdk-go-v2（S3 兼容探针）。

**Spec:** `docs/superpowers/specs/2026-10-16-beeftv-web-credits-design.md` 第 6、7 节

## Global Constraints

- 本机工具链（所有 Go 命令都要带这套环境变量）：

```powershell
$env:GOTELEMETRY="off"; $env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"
$env:GOPATH="D:\画布\_gopath"; $env:GOMODCACHE="D:\画布\_gomodcache"; $env:GOCACHE="D:\画布\_gocache"
$env:GOTMPDIR="D:\画布\_gotmp"; $env:TMP="D:\画布\_gotmp"; $env:TEMP="D:\画布\_gotmp"
$env:CGO_ENABLED="1"; $env:CC="D:\画布\_tools\zig\zig-x86_64-windows-0.14.1\zig.exe cc"
$env:ZIG_GLOBAL_CACHE_DIR="D:\画布\_zigcache\global"; $env:ZIG_LOCAL_CACHE_DIR="D:\画布\_zigcache\local"
& "D:\画布\_tools\gotool\go\bin\go.exe" <args>
```

- 本机基线已知失败（**环境问题，不是代码缺陷，不要尝试修复**）：
  - SQLite 临时库清理失败：Windows 不允许删除仍被占用的文件。测试里必须在 `t.Cleanup` 里 `Close()` 掉 `sql.DB`。
  - 缺少打包产物 `plugin-packages/*.beeftv-plugin`，影响 `internal/protocol` 部分用例。
- **硬约束：`storage_locations` 属于托管表。** 绝不能加入 `LocalModels()`，否则 desktop 迁移会先建后删，`RequireLocalSchema` 也会被污染。托管模型走 `HostedModels()`。
- 所有 Go 代码默认 ASCII，业务中文文案可保留。
- 手工编辑用 `apply_patch`；不要 `git reset --hard`、不要 `git checkout --`。
- 提交信息格式：`<type>(<scope>): <业务模块> - <变更摘要>`。

---

### Task 1: 托管存储模型与托管迁移

**Files:**
- Create: `backend/internal/model/models_storage.go`
- Modify: `backend/internal/database/hosted_schema.go`
- Test: `backend/internal/database/hosted_schema_test.go`（追加）

**Interfaces:**
- Produces:
  - `model.StorageLocation`（表名 `storage_locations`）
  - `database.HostedModels() []any`（= `LocalModels()` + 托管模型）

- [ ] **Step 1: 写失败测试**

在 `hosted_schema_test.go` 追加：

```go
func TestMigrateHostedSchemaCreatesStorageLocations(t *testing.T) {
	db := openHostedTestDB(t, "storage-locations.db")
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	if !db.Migrator().HasTable("storage_locations") {
		t.Fatal("托管迁移必须建出 storage_locations")
	}
}
```

- [ ] **Step 2: 运行确认失败**

```powershell
& "D:\画布\_tools\gotool\go\bin\go.exe" test ./internal/database/ -run "TestMigrateHostedSchemaCreatesStorageLocations" -v
```

Expected: FAIL，`storage_locations` 不存在。

- [ ] **Step 3: 实现**

`backend/internal/model/models_storage.go`：

```go
package model

import "time"

// StorageLocation is a platform-level object-storage configuration owned by the
// admin backend. Credentials are stored encrypted and never returned in clear.
type StorageLocation struct {
	ID              string    `json:"id" gorm:"primaryKey;size:36"`
	Name            string    `json:"name" gorm:"size:120;index"`
	Provider        string    `json:"provider" gorm:"size:32;index"`
	Endpoint        string    `json:"endpoint" gorm:"size:500"`
	Region          string    `json:"region" gorm:"size:120"`
	Bucket          string    `json:"bucket" gorm:"size:255"`
	AccessKeyID     string    `json:"accessKeyId" gorm:"size:255"`
	SecretAccessKey string    `json:"-" gorm:"type:text"`
	CDNBaseURL      string    `json:"cdnBaseUrl" gorm:"size:500"`
	Prefix          string    `json:"prefix" gorm:"size:255"`
	PathStyle       bool      `json:"pathStyle"`
	IsDefault       bool      `json:"isDefault" gorm:"index"`
	Enabled         bool      `json:"enabled" gorm:"index"`
	LastTestedAt    *time.Time `json:"lastTestedAt"`
	LastTestError   string    `json:"lastTestError" gorm:"type:text"`
	CreatedBy       string    `json:"createdBy" gorm:"size:36;index"`
	UpdatedBy       string    `json:"updatedBy" gorm:"size:36"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (StorageLocation) TableName() string { return "storage_locations" }
```

`hosted_schema.go` 增加：

```go
// HostedModels 是 server/hosted 形态的结构：本地业务模型加托管侧控制面模型。
func HostedModels() []any {
	models := LocalModels()
	return append(models, &model.StorageLocation{})
}
```

并把 `MigrateHostedSchema` 里的 `LocalModels()` 换成 `HostedModels()`。

- [ ] **Step 4: 运行确认通过**

```powershell
& "D:\画布\_tools\gotool\go\bin\go.exe" test ./internal/database/ -run "TestMigrateHostedSchema|TestHostedMigration|TestLocalSchema" -v
```

Expected: PASS，且本地用例仍确认托管表被删除。

- [ ] **Step 5: 提交**

```bash
git add backend/internal/model/models_storage.go backend/internal/database/hosted_schema.go backend/internal/database/hosted_schema_test.go
git commit -m "feat(backend): 对象存储 - 新增托管存储配置模型与托管迁移"
```

---

### Task 2: 存储配置仓储

**Files:**
- Create: `backend/internal/repository/storage_location.go`
- Test: `backend/internal/repository/storage_location_test.go`

**Interfaces:**
- Consumes: `model.StorageLocation`（Task 1）
- Produces:
  - `(*Repository).StorageLocations() ([]model.StorageLocation, error)`
  - `(*Repository).StorageLocation(id string) (*model.StorageLocation, error)`
  - `(*Repository).CreateStorageLocation(*model.StorageLocation) error`
  - `(*Repository).UpdateStorageLocation(*model.StorageLocation) error`
  - `(*Repository).DeleteStorageLocation(id string) error`
  - `(*Repository).SetDefaultStorageLocation(id string, updatedAt time.Time) error`（同一事务内清掉其它默认）

- [ ] **Step 1: 写失败测试**：建库、建一条、设默认、再建一条设默认，断言只有一条默认；删除默认后无默认。
- [ ] **Step 2: 运行确认失败**（`undefined: StorageLocations`）。
- [ ] **Step 3: 实现**（`StorageLocations` 按 `is_default desc, created_at asc` 排序；`SetDefaultStorageLocation` 在事务里 `UPDATE storage_locations SET is_default = false` 再置目标为 true）。
- [ ] **Step 4: 运行确认通过**。
- [ ] **Step 5: 提交**：`feat(backend): 对象存储 - 新增存储配置仓储与默认唯一性`。

---

### Task 3: 存储配置服务（校验、加密、掩码、连通性）

**Files:**
- Create: `backend/internal/storageconfig/service.go`
- Create: `backend/internal/storageconfig/prober.go`
- Create: `backend/internal/storageconfig/s3prober.go`
- Test: `backend/internal/storageconfig/service_test.go`

**Interfaces:**
- Consumes: 仓储接口（Task 2 的方法签名）、`localcrypto.Encrypt/Decrypt`
- Produces:
  - `storageconfig.Repository`（仓储端口，便于测试替身）
  - `storageconfig.Prober`：`Probe(ctx context.Context, target Target) error`
  - `storageconfig.Service`：`List/Create/Update/Delete/SetDefault/TestConnection`
  - 输入输出类型 `Input`、`View`（`View` 不含明文密钥，只含 `SecretAccessKeyMasked`）

关键规则：

- `provider` 只接受 `s3`（S3 兼容，含 OSS/COS 的 S3 兼容端点）；
- `name`、`endpoint`、`bucket`、`accessKeyId`、`secretAccessKey` 必填；`prefix` 不能以 `/` 开头、不能含 `..`；
- 密钥用 `localcrypto.Encrypt(dataDir, secret)`，且 `Update` 时若密钥字段留空表示「保持不变」；
- `Delete` 禁止删除默认存储（先切换默认）；
- `SetDefault` 要求 `Enabled` 且 `LastTestedAt != nil` 且 `LastTestError == ""`，否则返回 `409`；
- `TestConnection` 成功写 `LastTestedAt`、清 `LastTestError`；失败写 `LastTestError` 并返回 `502`；
- 每次写操作追加 `AdminAuditEvent`（`target_type=storage_location`），元数据不含明文密钥。

- [ ] **Step 1: 写失败测试**：用内存替身仓储 + 假 prober 覆盖：创建即加密（读回仓储行 `SecretAccessKey` 带 `enc:v1:` 前缀）、`View` 不含明文、未测试不允许设默认、测试成功后可设默认、密钥留空更新保持原值。
- [ ] **Step 2: 运行确认失败**。
- [ ] **Step 3: 实现服务**。
- [ ] **Step 4: 运行确认通过**。
- [ ] **Step 5: 提交**：`feat(backend): 对象存储 - 新增存储配置服务与连通性校验`。

---

### Task 4: 管理后台 HTTP 入口（仅 server profile）

**Files:**
- Create: `backend/internal/hostedapi/storage.go`
- Modify: `backend/internal/bootstrap/runtime.go`
- Test: `backend/internal/hostedapi/storage_test.go`

**Interfaces:**
- Consumes: `storageconfig.Service`
- Produces: `hostedapi.RegisterStorageAdminRoutes(r *gin.RouterGroup, svc *storageconfig.Service, resolveActor func(*gin.Context) (*model.User, error), requireAdmin func(*model.User) error)`

路由（挂在 `/api/admin/storage-locations`）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `` | 列表（掩码） |
| POST | `` | 添加 |
| PUT | `/:id` | 编辑（密钥可留空） |
| DELETE | `/:id` | 删除（默认存储拒绝） |
| POST | `/:id/test` | 连通性测试 |
| POST | `/:id/default` | 设为默认 |

- [ ] **Step 1: 写失败测试**：用 `gin.New()` + 注入的假 actor（非管理员应 403；管理员可创建、列表不含明文密钥、未测试设默认返回 409）。
- [ ] **Step 2: 运行确认失败**。
- [ ] **Step 3: 实现**：路由层只做绑定与错误投影，复用 `app` 的错误类型；**不**在桌面路径注册。
- [ ] **Step 4: 在 runtime 中只在 `ProfileServer` 注册**，并确认 `go build ./...` 通过。
- [ ] **Step 5: 提交**：`feat(backend): 管理后台 - 新增对象存储配置增删改查与测试入口`。

---

### Task 5: S3 兼容探针与文档

**Files:**
- Create: `backend/internal/storageconfig/s3prober.go`（若 Task 3 未落地）
- Modify: `docs/content/docs/backend/backend-database.mdx` 或新增 `backend/storage.mdx`
- Modify: `backend/go.mod`、`backend/go.sum`

- [ ] **Step 1: `go get github.com/aws/aws-sdk-go-v2/service/s3@latest`**（含 `aws/credentials`、`aws`）。
- [ ] **Step 2: 实现 S3 探针**：`PutObject` 写一个 `connectivity-test` 小对象，`GetObject` 读回校验内容，`DeleteObject` 清理；`UsePathStyle` 由配置控制；自定义端点走 `BaseEndpoint`。
- [ ] **Step 3: 文档**：后台添加入口、字段含义、掩码与审计说明、未通过测试不得设为默认。
- [ ] **Step 4: `go build ./...` + 相关包测试通过，提交**：`feat(backend): 对象存储 - 新增 S3 兼容连通性探针与后台文档`。

---

## 自检结论

- **Spec 覆盖**：本计划覆盖设计文档第 6 节（对象存储配置入口）与第 7 节第 6 项（后台对象存储配置），不覆盖 S3 实际承载用户资源读写（那是「存储端口切换」的独立计划）。
- **明确不在本计划内**：把 `BlobStore` 从 `FileStore` 换成对象存储适配器、历史资源跨存储迁移、`user_oss_settings`（BYO 存储）。
- **已知未验证项**：本机无 Docker、内嵌 PostgreSQL 二进制崩溃，PostgreSQL 侧的迁移与探针只能在 Linux/CI 或真实对象存储上验证。

---

## 已交付：S3 字节后端（原计划外新增）

`internal/storageconfig/s3blobstore.go` 已实现 `asset.BlobStore` 的四个方法，用 `s3.PutObject/GetObject/DeleteObject/HeadObject` 对接 S3 兼容存储，并用进程内 S3 桩（`s3blobstore_test.go`）端到端验证了写入、前缀、Open 后的 Seek/Stat、删除与存在性。它已在 server 依赖图中、不在桌面依赖图。

## 下一步：资源存储端口切换（独立计划）

要把新资源真正写进对象存储，还需解决三处现有约束，都需要自己的实施计划：

1. **`Provider != "local"` 硬门槛**：`internal/asset/open.go` 的 `OpenOwnedRange` 和 `PrepareOwnedDelivery` 对非 `local` 资源直接返回 `ResourceNotLocal()`，切换前必须改成按存储位置解析后端。
2. **`BlobStore` 端口形状**：`Open(string) (*os.File, error)` 要求可 `Seek` 且可 `Stat`，所以 S3 实现必须把对象取回临时文件（Windows 无法 unlink-on-open，现已由 `PruneTempFiles` 兜底）。若要真正流式 Range 读取，需先把端口改宽为 `io.ReadSeekCloser` + `Stat`。
3. **写锁身份 `writeSpace()`**：`internal/asset/service.go` 通过类型断言 `*FileStore` 取串行化锁的标识；换成路由型 BlobStore 后这个标识会退化成 `service:%p`，失去跨 Service 实例的串行化保证，必须重新设计锁身份。

另需：`model.Resource.StorageSettingID` 已存在且带索引，但 `WriteObject` 目前把它固定写成空串，切换时要按默认存储回填；历史资源继续按记录的位置读取，跨存储迁移另行立项。

**已完成**：这三条已由 `2026-10-16-beeftv-p0-storage-routing.md`（Plan 3）落地——新增 `asset.BlobResolver` 端口，本地与托管分别由 `SingleLocation` 与 `storageconfig.Router` 实现，读写删全部改走解析器，`WriteObject` 按解析结果回填落库地点。
