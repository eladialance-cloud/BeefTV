param(
    [Parameter(Mandatory = $true)]
    [string]$SourceRoot,
    [Parameter(Mandatory = $true)]
    [string]$TargetRoot,
    [switch]$Copy,
    [switch]$VerifyOnly
)

$ErrorActionPreference = "Stop"

$expectedCommit = "6f16955c57053d77ea0ffbdbbf3a7985a3e0080a"
$baseTreeHash = "f975dc22f36e1972be137ff9c04615522958a7283afd07bda50e921f5525940f"
$patchId = "V7.4.26+beef-segment-assets.1"
$repoRoot = Split-Path -Parent $PSScriptRoot
$patchPath = Join-Path $repoRoot "build\rule-patches\v7.4.26-segment-assets.1.patch"
if (-not (Test-Path -LiteralPath $patchPath)) {
    throw "Rule patch missing: $patchPath"
}
$patchHash = (Get-FileHash -LiteralPath $patchPath -Algorithm SHA256).Hash.ToLowerInvariant()
$source = (Resolve-Path -LiteralPath $SourceRoot).Path
$sourceGitPath = $source -replace "\\", "/"
$actualCommit = (git -c "safe.directory=$sourceGitPath" -C $source rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $actualCommit -ne $expectedCommit) {
    throw "Unexpected source commit: $actualCommit (expected $expectedCommit)"
}
$allowedDirty = @(
    "short-drama-director/scripts/check_asset_blocks.py",
    "short-drama-director/scripts/compile_feed.py",
    "short-drama-director/scripts/sd_core.py"
)
$dirty = @(git -c "safe.directory=$sourceGitPath" -C $source status --short --porcelain)
foreach ($line in $dirty) {
    $relative = $line.Substring(3).Trim().Replace("\", "/")
    if ($allowedDirty -notcontains $relative) {
        throw "Unexpected source modification: $relative"
    }
}

$target = [System.IO.Path]::GetFullPath($TargetRoot)
if ($Copy -and -not $VerifyOnly) {
    if (Test-Path -LiteralPath $target) {
        throw "Target already exists; refusing to overwrite: $target"
    }
    New-Item -ItemType Directory -Path $target | Out-Null
    Get-ChildItem -LiteralPath $source -Force |
        Where-Object { $_.Name -ne ".git" } |
        ForEach-Object {
            Copy-Item -LiteralPath $_.FullName -Destination $target -Recurse -Force
        }
}

if (-not (Test-Path -LiteralPath $target)) {
    throw "Target does not exist: $target"
}

$sourceFiles = @(Get-ChildItem -LiteralPath $source -Recurse -File |
    Where-Object { $_.FullName -notmatch "\\\.git\\" } |
    ForEach-Object { $_.FullName.Substring($source.Length + 1).Replace("\", "/") })
[Array]::Sort($sourceFiles, [StringComparer]::Ordinal)
$targetFiles = @(Get-ChildItem -LiteralPath $target -Recurse -File |
    ForEach-Object { $_.FullName.Substring($target.Length + 1).Replace("\", "/") })
[Array]::Sort($targetFiles, [StringComparer]::Ordinal)

$missing = @($sourceFiles | Where-Object { $targetFiles -notcontains $_ })
$extra = @($targetFiles | Where-Object { $sourceFiles -notcontains $_ })
if ($missing.Count -gt 0 -or $extra.Count -gt 0) {
    throw "File list mismatch. missing=$($missing.Count) extra=$($extra.Count)"
}

$files = [ordered]@{}
$different = [System.Collections.Generic.List[string]]::new()
$manifestLines = [System.Collections.Generic.List[string]]::new()
foreach ($relativePath in $sourceFiles) {
    $sourceFile = Join-Path $source ($relativePath -replace "/", "\")
    $targetFile = Join-Path $target ($relativePath -replace "/", "\")
    $sourceHash = (Get-FileHash -LiteralPath $sourceFile -Algorithm SHA256).Hash.ToLowerInvariant()
    $targetHash = (Get-FileHash -LiteralPath $targetFile -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($sourceHash -ne $targetHash) {
        $different.Add($relativePath)
    }
    $files[$relativePath] = $sourceHash
    $manifestLines.Add("${relativePath}:$sourceHash")
}
if ($different.Count -gt 0) {
    throw "Rule content mismatch in $($different.Count) files: $($different -join ', ')"
}

$manifestText = [string]::Join("`n", $manifestLines) + "`n"
$sha = [System.Security.Cryptography.SHA256]::Create()
try {
    $treeHash = -join ($sha.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($manifestText)) | ForEach-Object { $_.ToString("x2") })
} finally {
    $sha.Dispose()
}

$manifestPath = Join-Path $repoRoot "backend\internal\manju\skilldata\manifests\v7.4.26-source.json"
$manifestDirectory = Split-Path -Parent $manifestPath
New-Item -ItemType Directory -Path $manifestDirectory -Force | Out-Null
$manifest = [ordered]@{
    version = "V7.4.26"
    upstreamCommit = $expectedCommit
    patchId = $patchId
    patchSHA256 = $patchHash
    baseTreeHash = $baseTreeHash
    patchedTreeHash = $treeHash
    commit = $expectedCommit
    fileCount = $sourceFiles.Count
    treeHash = $treeHash
    files = $files
}
$json = $manifest | ConvertTo-Json -Depth 4
[System.IO.File]::WriteAllText($manifestPath, $json + "`n", [System.Text.UTF8Encoding]::new($false))

"V7426_RULE_PARITY=PASS files=$($sourceFiles.Count) different=0"
"V7426_TREE_HASH=$treeHash"
"V7426_PATCH_ID=$patchId"
"V7426_PATCH_SHA256=$patchHash"
"V7426_MANIFEST=$manifestPath"
