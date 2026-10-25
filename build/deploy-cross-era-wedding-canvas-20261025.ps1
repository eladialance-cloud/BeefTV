param(
    [string]$Remote = "ubuntu@129.204.227.200",
    [string]$KeyPath = "",
    [string]$RemoteRepo = "/opt/visideep-manju"
)

$ErrorActionPreference = "Stop"

$archiveName = "visideep-cross-era-wedding-canvas-fix-20261025-143755.tar.gz"
$archive = Join-Path $PSScriptRoot $archiveName
$remoteArchive = "/tmp/$archiveName"

if (-not (Test-Path -LiteralPath $archive)) {
    throw "Deployment archive not found: $archive"
}
$actualHash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()

$scriptPath = Join-Path $PSScriptRoot "deploy-cross-era-wedding-canvas-20261025.sh"
if (-not (Test-Path -LiteralPath $scriptPath)) {
    throw "Remote deployment script not found: $scriptPath"
}
$remoteScript = "/tmp/deploy-cross-era-wedding-canvas-20261025.sh"

$scpArgs = @("-O", "-o", "ConnectTimeout=20", "-o", "ServerAliveInterval=15")
if ($KeyPath) {
    $scpArgs += @("-i", $KeyPath)
}
$scpArgs += @("--", $archive, $scriptPath, "${Remote}:/tmp/")
& scp.exe @scpArgs
if ($LASTEXITCODE -ne 0) {
    throw "Remote deployment script upload failed with exit code $LASTEXITCODE"
}

$sshArgs = @("-T", "-o", "ConnectTimeout=20", "-o", "ServerAliveInterval=15")
if ($KeyPath) {
    $sshArgs += @("-i", $KeyPath)
}
$sshArgs += @($Remote, "bash $remoteScript $remoteArchive $actualHash $RemoteRepo")
& ssh.exe @sshArgs
if ($LASTEXITCODE -ne 0) {
    throw "Remote deployment failed with exit code $LASTEXITCODE"
}
