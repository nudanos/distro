param(
    [Parameter(Mandatory)][ValidatePattern('^[a-z0-9-]+$')][string]$Repo,
    [Parameter(Mandatory)][string]$Commands
)
$ErrorActionPreference = 'Stop'
$revivalRoot = Split-Path -Parent $PSScriptRoot
$runtimeDir = Join-Path $revivalRoot 'test-runtime'
$artifactPath = Join-Path $PSScriptRoot "$Repo-test.tar.gz"
$pythonPath = 'C:\Users\Jon\.cache\codex-runtimes\codex-primary-runtime\dependencies\python\python.exe'
& $pythonPath (Join-Path $PSScriptRoot 'export-worktree.py') (Join-Path $revivalRoot $Repo) $artifactPath
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$keyPath = Join-Path $runtimeDir 'debian\id_ed25519'
$knownHosts = Join-Path $runtimeDir 'debian\known_hosts'
$guestArchive = "/home/builder/$Repo-test.tar.gz"
& scp.exe -q -i $keyPath -P 2222 -o BatchMode=yes -o "UserKnownHostsFile=$knownHosts" $artifactPath "builder@127.0.0.1:$guestArchive"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$guestDirectory = "/home/builder/revival-checks/$Repo"
$guestScript = @"
set -e
mkdir -p $guestDirectory
cd $guestDirectory
tar -xzf $guestArchive
$Commands
"@
& (Join-Path $runtimeDir 'ssh-debian.ps1') $guestScript
exit $LASTEXITCODE
