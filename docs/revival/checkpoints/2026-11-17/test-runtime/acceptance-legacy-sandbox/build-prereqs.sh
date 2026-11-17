#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
curl -fsS http://10.0.2.2:18767/pinned-baseline.asc -o /usr/share/keyrings/legacy-baseline.asc
cat >/etc/apt/sources.list.d/legacy-build.list <<'EOF'
deb [signed-by=/usr/share/keyrings/legacy-baseline.asc] http://10.0.2.2:18767/baseline-repo trixie main
deb [signed-by=/usr/share/keyrings/debian-archive-keyring.gpg] http://deb.debian.org/debian trixie main
EOF
apt-get update
apt-get install -y build-essential debhelper dh-yang dh-python python3-all python3-setuptools python3-yaml shellcheck libsystemd-dev libpam0g-dev pkg-config
dpkg --audit
echo BUILD_PREREQUISITES_READY
