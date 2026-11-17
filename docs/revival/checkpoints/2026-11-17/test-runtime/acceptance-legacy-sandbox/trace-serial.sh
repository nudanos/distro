#!/bin/bash
set -eu
target=$1
test "$(cat /proc/$target/comm)" = agetty
test "$(ps -o tty= -p "$target" | xargs)" = ttyS0
ps -o pid,ppid,pgid,sid,tty,args -p "$target"
mkdir -p /var/tmp/legacy-trace
nohup timeout -s INT -k 5 90 env LD_LIBRARY_PATH=/var/tmp/legacy-trace/tool/usr/lib/x86_64-linux-gnu /var/tmp/legacy-trace/tool/usr/bin/strace -ff -o /var/tmp/legacy-trace/serial.trace -p "$target" </dev/null >/var/tmp/legacy-trace/tracer.log 2>&1 &
echo "BOUNDED_TRACER_PID=$!"
