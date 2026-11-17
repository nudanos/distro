param([string]$Kind)
$ErrorActionPreference = 'Stop'
$base = Split-Path -Parent $PSScriptRoot
$out = Join-Path $PSScriptRoot 'task3-guest-stage'
New-Item -ItemType Directory -Force -Path $out | Out-Null
foreach($name in @('vyatta-login','cli-sandbox','vyatta-debian-pam-configs-config')) {
    $repo = Join-Path $base $name
    $target = Join-Path $out $name
    New-Item -ItemType Directory -Force -Path $target | Out-Null
    git -C $repo ls-files --cached --others --exclude-standard | ForEach-Object {
        $source = Join-Path $repo $_
        if (Test-Path -LiteralPath $source -PathType Leaf) {
            $dest = Join-Path $target $_
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dest) | Out-Null
            $bytes = [IO.File]::ReadAllBytes($source)
            $utf8 = [Text.UTF8Encoding]::new($false)
            if ($bytes -notcontains 0) {
                [IO.File]::WriteAllText($dest,$utf8.GetString($bytes).Replace([string][char]13 + [char]10,[string][char]10),$utf8)
            } else {
                [IO.File]::WriteAllBytes($dest,$bytes)
            }
        }
    }
}
$bundle=Join-Path $PSScriptRoot "task3-$Kind.tar"
tar.exe -cf $bundle -C $out vyatta-login cli-sandbox vyatta-debian-pam-configs-config
if($LASTEXITCODE -ne 0){exit $LASTEXITCODE}
$r=Join-Path $base 'test-runtime'
scp.exe -i "$r\debian\id_ed25519" -P 2222 -o BatchMode=yes -o "UserKnownHostsFile=$r\debian\known_hosts" $bundle builder@127.0.0.1:/home/builder/
if($LASTEXITCODE -ne 0){exit $LASTEXITCODE}
& "$r\ssh-debian.ps1" "mkdir -p /home/builder/login-safety/$Kind && tar -xf /home/builder/task3-$Kind.tar -C /home/builder/login-safety/$Kind"
