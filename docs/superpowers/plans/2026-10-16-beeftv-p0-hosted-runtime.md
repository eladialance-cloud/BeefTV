# BeefTV P0 托管运行基座实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 BeefTV 的 server profile 能跑在 PostgreSQL 上，并且启动时不再删除托管表，为多用户与积分体系打好数据基座。

**Architecture:** 在 `internal/database` 增加驱动规范化与 PostgreSQL 分支；把 schema 迁移按 Profile 分流，desktop 继续走本地 SQLite 迁移（含删表），server 走只做 AutoMigrate 的托管迁移路径；连接池按驱动设置不同上限。

**Tech Stack:** Go 1.25、GORM、`gorm.io/driver/postgres`、pgx、Gin、PostgreSQL、`fergusstrange/embedded-postgres`（仅测试）。

**Spec:** `docs/superpowers/specs/2026-10-16-beeftv-web-credits-design.md`

## Global Constraints

- 本机工具链（已验证可用，所有 Go 命令都要带这套环境变量）：

```powershell
$env:GOTELEMETRY="off"; $env:GOPROXY="https://goproxy.cn,direct"
$env:GOMODCACHE="D:\画布\_gomodcache"; $env:GOCACHE="D:\画布\_gocache"; $env:GOTMPDIR="D:\画布\_gotmp"
$env:CGO_ENABLED="1"; $env:CC="D:\画布\_tools\zig\zig-x86_64-windows-0.14.1\zig.exe cc"
& "D:\画布\_tools\gotool\go\bin\go.exe" <args>
```

- 本机基线已知失败（**环境问题，不是代码缺陷，不要尝试修复**）：
  - SQLite 临时库清理失败：Windows 不允许删除仍被占用的文件，报 `TempDir RemoveAll cleanup ... being used by another process`，影响 `internal/repository`、`internal/taskdelivery` 等包；Linux CI 无此问题。
  - 缺少打包产物：`plugin-packages/*.beeftv-plugin` 需要跑 `scripts/build-beeftv-release.sh` 才生成，缺失导致 `internal/protocol`、`internal/provider` 部分用例失败。
- 所有 Go 代码默认 ASCII，业务中文注释可保留。
- 手工编辑用 `apply_patch`；不要 `git reset --hard`、不要 `git checkout --`。
- 提交信息格式：`<type>(<scope>): <业务模块> - <变更摘要>`。
- 不要为旧 API 加兼容层；签名变化直接改所有调用点。

---

### Task 1: PostgreSQL 驱动、驱动规范化与连接池分流

**Files:**
- Modify: `backend/go.mod`、`backend/go.sum`
- Modify: `backend/internal/database/database.go`
- Modify: `backend/internal/bootstrap/runtime.go:94`
- Modify: `backend/internal/app/db_runtime_matrix_test.go:217`
- Modify: `backend/internal/database/local_pool_test.go:21`
- Modify: `backend/internal/conversation/service_test.go:54`、`:75`
- Test: `backend/internal/database/database_driver_test.go`（新建）

**Interfaces:**
- Consumes: 无
- Produces:
  - `database.DriverSQLite = "sqlite"`、`database.DriverPostgres = "postgres"`
  - `database.NormalizeDriver(driver string) string`
  - `database.IsSQLite(driver string) bool`
  - `database.ConfigurePool(db *gorm.DB, driver string) error`（**签名变更**，第二参数为驱动名）

- [ ] **Step 1: 引入驱动依赖**

```powershell
cd "D:\画布\BeefTV\backend"
& "D:\画布\_tools\gotool\go\bin\go.exe" get gorm.io/driver/postgres@latest
```

- [ ] **Step 2: 写失败测试**

新建 `backend/internal/database/database_driver_test.go`：

```go
package database

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNormalizeDriverFallsBackToSQLite(t *testing.T) {
	if got := NormalizeDriver(""); got != DriverSQLite {
		t.Fatalf("空驱动应回退 sqlite，得到 %q", got)
	}
	if got := NormalizeDriver(" PostgreSQL "); got != DriverPostgres {
		t.Fatalf("驱动名应去空格并小写，得到 %q", got)
	}
	if !IsSQLite("") || IsSQLite("postgres") {
		t.Fatalf("IsSQLite 判定错误")
	}
}

func TestOpenPostgresRequiresDSN(t *testing.T) {
	if _, err := Open(Config{Driver: DriverPostgres}); err == nil {
		t.Fatal("postgres 缺少 DSN 时必须报错")
	}
}

func TestConfigurePoolSplitsByDriver(t *testing.T) {
	raw, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pool.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := ConfigurePool(raw, DriverSQLite); err != nil {
		t.Fatalf("sqlite 连接池配置失败: %v", err)
	}
	sqlDB, err := raw.DB()
	if err != nil {
		t.Fatalf("取底层连接失败: %v", err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("sqlite 必须单写连接，得到 %d", got)
	}
	if err := ConfigurePool(raw, DriverPostgres); err != nil {
		t.Fatalf("postgres 连接池配置失败: %v", err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 25 {
		t.Fatalf("postgres 连接池上限应为 25，得到 %d", got)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```powershell
& "D:\画布\_tools\gotool\go\bin\go.exe" test ./internal/database/ -run "TestNormalizeDriverFallsBackToSQLite|TestOpenPostgresRequiresDSN|TestConfigurePoolSplitsByDriver" -v
```

Expected: 编译失败，`undefined: NormalizeDriver`（`ConfigurePool` 参数个数不符）。

- [ ] **Step 4: 实现**

`backend/internal/database/database.go` 全文替换为：

```go
package database

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

type Config struct {
	Driver  string
	DSN     string
	DataDir string
}

// NormalizeDriver 统一驱动名，空值按 sqlite 处理。
func NormalizeDriver(driver string) string {
	value := strings.ToLower(strings.TrimSpace(driver))
	if value == "" {
		return DriverSQLite
	}
	return value
}

// IsSQLite 判断是否走本地单文件库。
func IsSQLite(driver string) bool {
	return NormalizeDriver(driver) == DriverSQLite
}

// Open 按驱动建立连接。sqlite 使用本地文件，postgres 必须显式提供 DSN。
func Open(config Config) (*gorm.DB, error) {
	switch driver := NormalizeDriver(config.Driver); driver {
	case DriverSQLite:
		dsn := strings.TrimSpace(config.DSN)
		if dsn == "" {
			if err := os.MkdirAll(config.DataDir, 0o755); err != nil {
				return nil, err
			}
			dsn = config.DataDir + "/open_ai_canvas.db?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on&_synchronous=NORMAL"
		}
		return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	case DriverPostgres:
		dsn := strings.TrimSpace(config.DSN)
		if dsn == "" {
			return nil, fmt.Errorf("postgres 需要 DATABASE_URL")
		}
		return gorm.Open(postgres.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("不支持的数据库驱动：%s", driver)
	}
}

// ConfigurePool 按驱动设置连接池。sqlite 必须串行写；postgres 面向并发 web 用户，
// 连接数要有上限，避免长耗时生成请求打满服务端连接。
func ConfigurePool(db *gorm.DB, driver string) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if IsSQLite(driver) {
		// SQLite serializes writers. A single shared connection avoids intermittent
		// SQLITE_BUSY failures under concurrent autosave/task updates while WAL
		// still keeps reads cheap for this single-process desktop application.
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		return nil
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return nil
}
```

- [ ] **Step 5: 更新全部调用点**

`backend/internal/bootstrap/runtime.go` 第 94 行：

```go
	if err := database.ConfigurePool(db, cfg.DatabaseDriver); err != nil {
```

其余三处测试文件的调用改为传入驱动名：`database.ConfigurePool(db)` → `database.ConfigurePool(db, "")`（测试均使用 sqlite）。

- [ ] **Step 6: 运行测试确认通过**

```powershell
& "D:\画布\_tools\gotool\go\bin\go.exe" test ./internal/database/ -run "TestNormalizeDriverFallsBackToSQLite|TestOpenPostgresRequiresDSN|TestConfigurePoolSplitsByDriver" -v
```

Expected: 三个用例 PASS。

- [ ] **Step 7: 全仓编译与受影响包测试**

```powershell
& "D:\画布\_tools\gotool\go\bin\go.exe" build ./...
& "D:\画布\_tools\gotool\go\bin\go.exe" test ./internal/database/... ./internal/conversation/...
```

Expected: `build` 无输出；两个包 OK。

- [ ] **Step 8: 提交**

```bash
git add backend/go.mod backend/go.sum backend/internal/database/database.go backend/internal/database/database_driver_test.go backend/internal/bootstrap/runtime.go backend/internal/app/db_runtime_matrix_test.go backend/internal/database/local_pool_test.go backend/internal/conversation/service_test.go
git commit -m "feat(backend): 数据库驱动 - 新增 PostgreSQL 支持与连接池按驱动分流"
```

---

### Task 2: Profile 分流的 schema 迁移

**Files:**
- Create: `backend/internal/database/hosted_schema.go`
- Create: `backend/internal/database/hosted_schema_test.go`
- Modify: `backend/internal/bootstrap/runtime.go:96-105`

**Interfaces:**
- Consumes: `NormalizeDriver`、`IsSQLite`（Task 1）
- Produces:
  - `database.MigrateHostedSchema(db *gorm.DB) error`
  - `database.RequireHostedSchema(db *gorm.DB) error`

**背景（必须理解，否则会写错）**：`MigrateLocalSchema` 会调用 `dropHostedTables()`，把 `credit_accounts`、`payment_orders`、`storage_locations`、`users` 等上游托管表全部 `DROP`。server profile 目前也走这条路径，所以任何托管表在 server 启动时都会被删掉。

- [ ] **Step 1: 写失败测试**

新建 `backend/internal/database/hosted_schema_test.go`：

```go
package database

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 托管迁移必须保留托管表：这是与本地迁移最关键的行为差异。
func TestMigrateHostedSchemaKeepsHostedTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "hosted.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := db.Exec("CREATE TABLE credit_accounts (id TEXT PRIMARY KEY, balance INTEGER)").Error; err != nil {
		t.Fatalf("建托管表失败: %v", err)
	}
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	if !db.Migrator().HasTable("credit_accounts") {
		t.Fatal("托管迁移不得删除 credit_accounts")
	}
}

func TestMigrateHostedSchemaIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "hosted-twice.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("重复迁移必须幂等: %v", err)
	}
}

func TestRequireHostedSchemaFailsOnEmptyDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "empty.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := RequireHostedSchema(db); err == nil {
		t.Fatal("空库必须报缺少核心表")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
& "D:\画布\_tools\gotool\go\bin\go.exe" test ./internal/database/ -run "Hosted" -v
```

Expected: 编译失败，`undefined: MigrateHostedSchema`。

- [ ] **Step 3: 实现托管迁移**

新建 `backend/internal/database/hosted_schema.go`：

```go
package database

import (
	"fmt"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// MigrateHostedSchema 是 server/hosted 形态的结构入口。它只做 GORM AutoMigrate，
// 不执行本地形态的 SQLite 结构修复，也绝不删除托管表。
func MigrateHostedSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(LocalModels()...); err != nil {
		return fmt.Errorf("托管结构迁移失败: %w", err)
	}
	return nil
}

// RequireHostedSchema 在关闭自动迁移时做最小可用性校验：核心表必须存在。
func RequireHostedSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Project{}) {
		return fmt.Errorf("托管数据库缺少核心表 projects，请先执行迁移")
	}
	return nil
}
```

- [ ] **Step 4: 按 Profile 分流**

`backend/internal/bootstrap/runtime.go` 的迁移块改为：

```go
	if cfg.AutoMigrate {
		if database.IsSQLite(cfg.DatabaseDriver) {
			err = database.MigrateLocalSchema(db)
		} else {
			err = database.MigrateHostedSchema(db)
		}
	} else {
		if database.IsSQLite(cfg.DatabaseDriver) {
			err = database.RequireLocalSchema(db)
		} else {
			err = database.RequireHostedSchema(db)
		}
	}
```

- [ ] **Step 5: 运行测试确认通过**

```powershell
& "D:\画布\_tools\gotool\go\bin\go.exe" test ./internal/database/ -run "Hosted" -v
& "D:\画布\_tools\gotool\go\bin\go.exe" test ./internal/database/...
```

Expected: 新用例通过；`internal/database` 包整体 OK。

- [ ] **Step 6: 确认没有回归掉本地行为**

```powershell
& "D:\画布\_tools\gotool\go\bin\go.exe" test ./internal/bootstrap/ -run "TestDesktopSchemaExcludesHostedTables" -v
```

Expected: PASS（desktop 继续删托管表，行为不变）。

- [ ] **Step 7: 提交**

```bash
git add backend/internal/database/hosted_schema.go backend/internal/database/hosted_schema_test.go backend/internal/bootstrap/runtime.go
git commit -m "feat(backend): 数据库迁移 - 按 Profile 分流并阻止托管表被删除"
```

---

### Task 3: PostgreSQL 集成测试

**Files:**
- Modify: `backend/go.mod`、`backend/go.sum`
- Create: `backend/internal/database/postgres_integration_test.go`

**Interfaces:**
- Consumes: `MigrateHostedSchema`、`ConfigurePool`、`Open`（Task 1、2）
- Produces: 无导出接口，仅测试。

**说明**：本机没有 Docker，也没有本机 PostgreSQL，因此用 `fergusstrange/embedded-postgres` 在测试内拉起临时实例；二进制从 Maven Central 下载（已验证 `repo1.maven.org` 可达）。该测试用环境变量 `BEEFTV_TEST_POSTGRES=1` 显式开启，默认跳过，避免影响日常开发与 CI。

- [ ] **Step 1: 引入测试依赖并写失败测试**

```powershell
cd "D:\画布\BeefTV\backend"
& "D:\画布\_tools\gotool\go\bin\go.exe" get -t github.com/fergusstrange/embedded-postgres@latest
```

新建 `backend/internal/database/postgres_integration_test.go`：

```go
package database

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

func freePort(t *testing.T) uint32 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("取空闲端口失败: %v", err)
	}
	defer listener.Close()
	return uint32(listener.Addr().(*net.TCPAddr).Port)
}

// 真实 PostgreSQL 上验证：驱动可用、托管迁移不删表、连接池按驱动分流。
// 需要 BEEFTV_TEST_POSTGRES=1 才运行。
func TestPostgresHostedSchemaMigration(t *testing.T) {
	if os.Getenv("BEEFTV_TEST_POSTGRES") != "1" {
		t.Skip("设置 BEEFTV_TEST_POSTGRES=1 才运行 PostgreSQL 集成测试")
	}
	port := freePort(t)
	instance := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(port).
		Database("beeftv_test").
		Username("beeftv").
		Password("beeftv").
		RuntimePath(t.TempDir()).
		StartTimeout(90 * time.Second))
	if err := instance.Start(); err != nil {
		t.Fatalf("启动内嵌 PostgreSQL 失败: %v", err)
	}
	defer func() { _ = instance.Stop() }()

	dsn := fmt.Sprintf("host=127.0.0.1 port=%d user=beeftv password=beeftv dbname=beeftv_test sslmode=disable", port)
	db, err := Open(Config{Driver: DriverPostgres, DSN: dsn})
	if err != nil {
		t.Fatalf("连接 PostgreSQL 失败: %v", err)
	}
	if err := ConfigurePool(db, DriverPostgres); err != nil {
		t.Fatalf("配置连接池失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取底层连接失败: %v", err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 25 {
		t.Fatalf("postgres 连接池上限应为 25，得到 %d", got)
	}
	if err := db.Exec("CREATE TABLE credit_accounts (id TEXT PRIMARY KEY, balance BIGINT)").Error; err != nil {
		t.Fatalf("建托管表失败: %v", err)
	}
	if err := MigrateHostedSchema(db); err != nil {
		t.Fatalf("托管迁移失败: %v", err)
	}
	if !db.Migrator().HasTable("credit_accounts") {
		t.Fatal("托管迁移不得删除 credit_accounts")
	}
	if !db.Migrator().HasTable("projects") {
		t.Fatal("托管迁移必须建出 projects 表")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
$env:BEEFTV_TEST_POSTGRES="1"
& "D:\画布\_tools\gotool\go\bin\go.exe" test ./internal/database/ -run "TestPostgresHostedSchemaMigration" -v
```

Expected: 编译失败或运行失败（`MigrateHostedSchema` 尚未在 Postgres 上验证过、依赖可能未补齐）。

- [ ] **Step 3: 让测试通过**

按报错修正；本任务预期不需要改产品代码，若 `AutoMigrate` 在 Postgres 上因模型定义不兼容而失败，按报错最小修正模型标签（例如把不支持的列类型换成跨库类型），不要改动 SQLite 行为。

- [ ] **Step 4: 运行测试确认通过**

```powershell
$env:BEEFTV_TEST_POSTGRES="1"
& "D:\画布\_tools\gotool\go\bin\go.exe" test ./internal/database/ -run "TestPostgresHostedSchemaMigration" -v
```

Expected: PASS。

- [ ] **Step 5: 确认默认跳过后不拖慢常规测试**

```powershell
Remove-Item Env:\BEEFTV_TEST_POSTGRES -ErrorAction SilentlyContinue
& "D:\画布\_tools\gotool\go\bin\go.exe" test ./internal/database/ -run "TestPostgresHostedSchemaMigration" -v
```

Expected: `--- SKIP`。

- [ ] **Step 6: 提交**

```bash
git add backend/go.mod backend/go.sum backend/internal/database/postgres_integration_test.go
git commit -m "test(backend): 数据库迁移 - 增加 PostgreSQL 集成测试"
```

---

### Task 4: 部署编排与文档

**Files:**
- Modify: `docker-compose.yml`
- Modify: `docker-compose.local.yml`（若同样需要 postgres，按 Task 说明判断）
- Modify: `docs/content/docs/backend/backend-database.mdx`
- Modify: `docs/local-first-architecture.md`

**Interfaces:**
- Consumes: Task 1、2 的驱动与迁移行为
- Produces: 无代码接口，交付可启动的 PostgreSQL 部署形态与文档。

- [ ] **Step 1: 给 compose 增加 postgres 服务**

`docker-compose.yml` 的 `backend` 服务补上数据库连接与依赖，并新增 `postgres` 服务：

```yaml
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_DB: ${CANVAS_POSTGRES_DB:-beeftv}
      POSTGRES_USER: ${CANVAS_POSTGRES_USER:-beeftv}
      POSTGRES_PASSWORD: ${CANVAS_POSTGRES_PASSWORD:-beeftv}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${CANVAS_POSTGRES_USER:-beeftv} -d ${CANVAS_POSTGRES_DB:-beeftv}"]
      interval: 10s
      timeout: 5s
      retries: 10
    restart: unless-stopped
```

`backend` 服务的 `environment` 增加：

```yaml
      DATABASE_URL: ${DATABASE_URL:-postgres://beeftv:beeftv@postgres:5432/beeftv?sslmode=disable}
```

`backend` 的 `depends_on` 增加 `postgres: {condition: service_healthy}`，`volumes` 段增加 `postgres-data:`。

- [ ] **Step 2: 补文档**

`docs/content/docs/backend/backend-database.mdx` 增加 PostgreSQL 小节：驱动名 `postgres`、`DATABASE_URL` 必填、server profile 走托管迁移（不删托管表）、desktop 仍走本地 SQLite 迁移。`docs/local-first-architecture.md` 增加一句：server/hosted 形态由 `CANVAS_DATABASE_DRIVER=postgres` 启用，桌面形态不受影响。

- [ ] **Step 3: 验证配置可解析**

```powershell
docker compose -f "D:\画布\BeefTV\docker-compose.yml" config
```

Expected: 输出解析后的配置，无语法错误。**本机没有 Docker，此项需在有 Docker 的机器上执行并在交付说明中如实标注未验证。**

- [ ] **Step 4: 提交**

```bash
git add docker-compose.yml docs/content/docs/backend/backend-database.mdx docs/local-first-architecture.md
git commit -m "build(deploy): 数据库 - 增加 PostgreSQL 服务与托管部署文档"
```

---

## 自检结论

- **Spec 覆盖**：本计划只覆盖设计文档 P0 中的「PostgreSQL 驱动与连接池」「Profile 分流的 schema 迁移」「部署」三项。P0 的「对象存储端口与后台添加入口」是独立子系统，另开计划（Plan 2），不并入本计划。
- **占位符扫描**：无 TBD / TODO；所有代码步骤含可直接落盘的完整代码。
- **类型一致性**：`ConfigurePool(db, driver)` 在 Task 1 定义，Task 2、3 只通过 `IsSQLite` / `MigrateHostedSchema` 使用，签名一致；`MigrateHostedSchema` / `RequireHostedSchema` 在 Task 2 定义并在 Task 2、3 使用。
- **已知未验证项**：Task 4 Step 3 需要 Docker，本机不可执行，已在步骤中标注。
