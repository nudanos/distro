$ErrorActionPreference = 'Stop'
$runtime = $PSScriptRoot
$guest = Join-Path $runtime 'debian'
$pidFile = Join-Path $guest 'qemu.pid'
if (Test-Path -LiteralPath $pidFile) {
    $vmPid = [int](Get-Content -LiteralPath $pidFile)
    if (Get-Process -Id $vmPid -ErrorAction SilentlyContinue) { throw "VM already running with PID $vmPid" }
}
$qemu = Join-Path $runtime 'tools\qemu\qemu-system-x86_64.exe'
$arguments = @(
    '-name', 'nudanos-debian-test', '-machine', 'q35', '-accel', 'whpx',
    '-cpu', 'qemu64,-svm', '-smp', '4', '-m', '4096', '-display', 'none', '-monitor', 'none',
    '-serial', 'file:serial.log', '-no-reboot',
    '-drive', 'file=system.qcow2,format=qcow2,if=virtio',
    '-drive', 'file=seed.iso,format=raw,media=cdrom,readonly=on',
    '-netdev', 'user,id=net0,hostfwd=tcp:127.0.0.1:2222-:22',
    '-device', 'virtio-net-pci,netdev=net0', '-device', 'virtio-rng-pci'
)
$process = Start-Process -FilePath $qemu -ArgumentList $arguments -WorkingDirectory $guest -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $guest 'qemu.stdout.log') -RedirectStandardError (Join-Path $guest 'qemu.stderr.log')
$process.Id | Set-Content -LiteralPath $pidFile
Write-Output "Started Debian VM PID $($process.Id), SSH 127.0.0.1:2222"

