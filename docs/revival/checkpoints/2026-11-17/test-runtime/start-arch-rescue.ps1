$ErrorActionPreference = 'Stop'
$runtime = $PSScriptRoot
$guest = Join-Path $runtime 'arch'
$pidFile = Join-Path $guest 'rescue.pid'
if (Test-Path -LiteralPath $pidFile) {
    $vmPid = [int](Get-Content -LiteralPath $pidFile)
    if (Get-Process -Id $vmPid -ErrorAction SilentlyContinue) { throw "VM already running with PID $vmPid" }
}
$qemu = Join-Path $runtime 'tools\qemu\qemu-system-x86_64.exe'
$arguments = @(
    '-name', 'nudanos-arch-test', '-machine', 'q35', '-accel', 'tcg,thread=multi',
    '-drive', ('if=pflash,format=raw,readonly=on,file=' + (Join-Path $runtime 'tools\qemu\share\edk2-x86_64-code.fd')), '-drive', 'if=pflash,format=raw,file=rescue-vars.fd', '-cpu', 'max', '-smp', '4', '-m', '4096', '-display', 'none', '-monitor', 'none',
    '-serial', 'file:rescue-serial.log', '-no-reboot',
    '-drive', 'file=rescue.qcow2,format=qcow2,if=virtio',
    '-drive', 'file=seed.iso,format=raw,media=cdrom,readonly=on',
    '-netdev', 'user,id=net0,hostfwd=tcp:127.0.0.1:2224-:22',
    '-device', 'virtio-net-pci,netdev=net0', '-device', 'virtio-rng-pci'
)
$process = Start-Process -FilePath $qemu -ArgumentList $arguments -WorkingDirectory $guest -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $guest 'rescue.stdout.log') -RedirectStandardError (Join-Path $guest 'rescue.stderr.log')
$process.Id | Set-Content -LiteralPath $pidFile
Write-Output "Started Arch VM PID $($process.Id), SSH 127.0.0.1:2224"






