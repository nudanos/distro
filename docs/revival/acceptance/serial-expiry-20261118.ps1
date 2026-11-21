param([Parameter(Mandatory)][string]$Fixture)

$ErrorActionPreference = 'Stop'
$Queue = Join-Path $Fixture 'apt-queue.txt'
$Log = Join-Path $Fixture 'apt-upgrade.log'
$Current = [IO.File]::ReadAllText((Join-Path $Fixture 'lockcheck.private'))
$NextPath = Join-Path $Fixture 'lockcheck-renewed.private'
if (Test-Path -LiteralPath $NextPath) {
    throw 'Use a fresh expired-account fixture'
}
$Next = 'NuD-' + [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(18)) + '-b7'
[IO.File]::WriteAllText($NextPath, $Next)

function Submit([string]$Text) {
    Start-Sleep -Milliseconds 1250
    $script:Cursor = (Get-Content -LiteralPath $Log).Count
    [IO.File]::AppendAllText($Queue, $Text + "`n")
}

function Wait-Prompt([string]$Pattern, [string]$Label, [int]$Seconds = 15) {
    $Deadline = (Get-Date).AddSeconds($Seconds)
    do {
        $Recent = (Get-Content -LiteralPath $Log | Select-Object -Skip $script:Cursor) -join "`n"
        if ($Recent -match $Pattern) { Write-Output $Label; return }
        if ($Recent -match 'Login incorrect|RPC failure|Authentication token manipulation error') {
            throw "PAM rejected the expiry conversation before $Label"
        }
        Start-Sleep -Milliseconds 300
    } while ((Get-Date) -lt $Deadline)
    throw "Timed out waiting for $Label"
}

Submit 'exit'
Wait-Prompt 'node login:\s*$' 'Serial getty ready'
Submit 'lockcheck'
Wait-Prompt '(?m)^Password:\s*$' 'Serial login password prompt'
Submit $Current
Wait-Prompt '(?im)^(?:Current(?: UNIX)? password|Old password):\s*$' 'Expired-password current prompt'
Submit $Current
Wait-Prompt '(?im)^New password:\s*$' 'Expired-password new prompt'
Submit $Next
Wait-Prompt '(?im)^Retype new password:\s*$' 'Expired-password confirmation prompt'
Submit $Next
Wait-Prompt 'lockcheck@[^:]+:~\$\s*$' 'Expired-password serial renewal reached an authenticated shell' 45
