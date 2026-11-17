& ssh.exe -i "$PSScriptRoot\arch\id_ed25519" -p 2224 -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -o "UserKnownHostsFile=$PSScriptRoot\arch\rescue-known_hosts" builder@127.0.0.1 @args
exit $LASTEXITCODE


