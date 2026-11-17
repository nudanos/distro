#!/bin/bash
set -uo pipefail
exec > >(tee /var/tmp/legacy-old-failure.log) 2>&1
date -u
systemctl status cli-sandbox@nudanos.service --no-pager -l
journalctl -b -u cli-sandbox@nudanos.service --no-pager -n 100
journalctl -b --no-pager -n 250 | grep -Ei 'pam|sandbox|login|nspawn|error|failed'
find /run/cli-sandbox -maxdepth 4 -type f -print -exec cat {} \;
find /var/lib/sandbox -maxdepth 2 -name cli_sandbox_init -o -name os-release
ls -ld /run/user/* /run/systemd/machines/* 2>/dev/null
echo OLD_FAILURE_DIAG_COMPLETE
