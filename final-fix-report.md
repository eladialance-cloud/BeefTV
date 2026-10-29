# Final Fix Report

Branch: `manju/v7426-gate-hardening`

## Commits

- Implementation commit: `04bcc2d3e21a4ea4b1d7c313da94e5269f2f7a7b`
- Implementation tree: `58e1269dd48c9002e8fa6a88df4880416d45f594`
- Review package head before fixes: `551528e89fad816f7058d64f5b192a64213610d7`
- Merge base: `09dc6fddfba5877ecddb4dfba4c31e275fbdce88`

The implementation commit contains 244 explicit files. It includes the complete
V7.4.26 embedded tree and manifest, all manju source/tests used by the scoped
packages, the direct hosted/database/model dependencies needed for compilation,
and no build caches, generated binaries, debug cookies, or secrets.

## Fixes

- Enforced at most one project-level `ACTIVE` production run. Stage
  regeneration, artifact reactivation, and manual-correction reactivation now
  use `ActivateManjuProductionRun`, which retires sibling active runs to
  `SUPERSEDED` history in the same transaction.
- Bound P4 compilation, P4 canvas packaging, and P4.5 state reads to the
  `project_state` snapshot whose `artifact_id` equals the current locked P3
  artifact, instead of reading the latest project snapshot.
- Hardened B-class P0B validation so each required technical hazard needs a
  positive, non-negated `永不解锁` assertion. Negated or contradictory forms
  such as `物体穿身并非永不解锁` now fail.
- Added `hasDeliverables` to production-run views. P5 now offers completed runs
  and deliverable-backed `SUPERSEDED` runs in read-only history, while using the
  existing run-scoped deliverables API.
- Canonicalized all 170 embedded text files to LF and regenerated the source
  manifest/contract hashes:
  - `patchedTreeHash=3f505ee09eabb4d414cb93f1724014ed2aa277951eaaa275a3f605a30c4a8c4f`
  - `fileCount=170`
  - `baseTreeHash=f975dc22f36e1972be137ff9c04615522958a7283afd07bda50e921f5525940f`
  - `patchId=V7.4.26+beef-segment-assets.1`

## Tests

Focused regressions:

```text
go test ./internal/manju -run '^(TestP0BRejectsMalformedAndAuthorityViolations|TestRegenerateProductionStageRetiresSiblingActiveRun|TestRegenerateProductionArtifactRetiresSiblingActiveRun|TestP4ReadsProjectStateBoundToLockedP3Artifact|TestSupersededCompletedRunWithDeliverablesRemainsHistoryVisible|TestP4CanvasPackageUsesCurrentP3ImageLineage|TestP4RequiresReadyP45BeforeConfirmation|TestGenerateProductionPrevisUsesNewOperationKeyPerAttempt)$' -count=1
ok  infinite-canvas/backend/internal/manju
```

Full scoped Go suite in the working checkout:

```text
PYTHONDONTWRITEBYTECODE=1 go test ./internal/manju ./internal/hostedapi ./internal/canvas ./cmd/server -count=1
ok  infinite-canvas/backend/internal/manju    154.486s
ok  infinite-canvas/backend/internal/hostedapi 86.772s
ok  infinite-canvas/backend/internal/canvas     26.879s
ok  infinite-canvas/backend/cmd/server           7.032s
```

Frontend:

```text
bun test test/manju-stage-workspace.test.ts
17 pass, 0 fail

bun run build
tsc --noEmit && vite build
19470 modules transformed
built in 1m 41s
```

The frontend build was rerun outside the sandbox after Vite's Windows realpath
helper hit sandbox `spawn EPERM`. TypeScript and Vite both passed.

## Clean Archive

The exact implementation commit was exported with `git archive` to:

```text
D:\画布\BeefTV\.superpowers\sdd\2026-10-28-manju-v7426-gate-hardening\clean-check
```

Archive:

```text
D:\画布\BeefTV\.superpowers\sdd\2026-10-28-manju-v7426-gate-hardening\clean-check.tar
```

Required clean-check command:

```text
PYTHONDONTWRITEBYTECODE=1 go test ./internal/manju ./internal/hostedapi ./internal/canvas ./cmd/server -count=1
```

Clean-check result:

```text
ok  infinite-canvas/backend/internal/manju    154.970s
ok  infinite-canvas/backend/internal/hostedapi 83.673s
ok  infinite-canvas/backend/internal/canvas     22.379s
ok  infinite-canvas/backend/cmd/server           4.037s
```

This confirms the requested packages compile and test with no reliance on
untracked worktree files.

## Dependency Closure Follow-up

The first clean archive was scoped to the Go packages above and did not prove a
complete frontend build. A second exact-tree verification found and closed the
web dependency gap:

- Added the existing Manju page dependencies for artifact editing, Canvas
  tables, Manju packages, style references, model catalog, credits, account
  chrome, login routing, and project/canvas baseline modules.
- Made server schema selection follow `ProfileServer` even on SQLite so hosted
  account tables are migrated before `/register` or `/login`.
- Removed the PostgreSQL service from the primary local Compose file.
- Fixed Windows path matching, browser executable selection, and temporary
  SQLite process cleanup in the affected tests.
- Kept tests that require unavailable platform tools visible:
  `ffmpeg`-dependent browser suites and POSIX `rsync` release-boundary tests are
  skipped on this Windows host.

Exact staged-tree frontend result:

```text
bun run lint       passed
bun run typecheck  passed
bun run test       3694 pass, 43 skip, 0 fail
```

Follow-up commits:

- `a1bf8a9 fix(task): 独立任务 - 先解析私有创建提示词`
- `820c1ab refactor(credits): 桌面依赖 - 下沉微积分比例常量`

The task follow-up makes `CreateLocalTask` resolve a private creation prompt
before task-type validation. The credits follow-up removes the hosted credits
package from the desktop dependency graph while keeping existing
`credits.MicrocreditsPerCredit` callers source-compatible.

Latest HEAD backend compile gate:

```text
go test ./... -run '^$' -count=1  passed for every package
```

Targeted backend tests passed:

```text
internal/app       private prompt resolution and hosted model selection
internal/bootstrap desktop dependency boundary and hosted SQLite schema
internal/database  hosted microcredits migration
```

The latest HEAD frontend-to-backend SQLite archive integration also passes:

```text
test/canvas-archive-sqlite-restore.test.ts  3 pass, 0 fail
```

The full Go suite was also rerun. `internal/hostedapi`, `internal/manju`, and
`cmd/server` pass. The repository-wide run is not green on this Windows host:
379 tests fail, predominantly because SQLite files remain locked during
`TempDir` cleanup, POSIX `0600` permissions cannot be represented, symlink
creation requires elevated privilege, `/bin/sh` is absent, and external network
tests time out. These environment failures must be resolved or rerun in the
Ubuntu CI environment before claiming a repository-wide green result.

## Concerns

- The worktree still contains unrelated pre-existing changes outside this
  scoped commit. They were not reset, stashed, cleaned, or included.
- The clean scoped packages required a small pre-existing dependency closure
  outside `internal/manju`: creation-catalog app/repository/model/hosted
  wiring; invite/admin moderation models referenced by `HostedModels`; channel
  query helpers; hosted credit migration; and `MicrocreditsPerCredit`. Only
  files proven necessary for the scoped compile were committed.
- Python gate executions must keep `PYTHONDONTWRITEBYTECODE=1` for integrity
  checks. A generated `__pycache__` directory changes the embedded file count
  from 170 to 185; the generated cache was removed and was not committed.
