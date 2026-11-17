#!/bin/bash
set -u
mkdir -p /var/tmp/legacy-trace
cd /var/tmp/legacy-trace
if ! test -x tool/usr/bin/strace; then
  apt-get download strace
  dpkg-deb -x strace_*.deb tool
fi
sha256sum strace_*.deb
if ! test -e tool/usr/lib/x86_64-linux-gnu/libunwind-ptrace.so.0; then
  apt-get download libunwind8
  dpkg-deb -x libunwind8_*.deb tool
fi
export LD_LIBRARY_PATH=/var/tmp/legacy-trace/tool/usr/lib/x86_64-linux-gnu
dpkg-query -W login
tool/usr/bin/strace -ff -o login.trace /bin/login -f nudanos
printf 'LOGIN_DIAGNOSTIC_EXIT=%s\n' "$?"
for f in login.trace*; do
  echo "$f"
  tail -35 "$f"
done
echo LOGIN_TRACE_COMPLETE
