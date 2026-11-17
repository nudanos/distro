& ssh.exe -i "$PSScriptRoot\debian\id_ed25519" -p 2222 -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -o "UserKnownHostsFile=$PSScriptRoot\debian\known_hosts" builder@127.0.0.1 @args
exit $LASTEXITCODE
