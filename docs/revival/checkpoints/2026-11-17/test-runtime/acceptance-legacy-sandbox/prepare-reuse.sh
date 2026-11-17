#!/bin/bash
set -euo pipefail
sed 's|/home/nudanos/terminal-probe.log|/home/serialtwo/terminal-probe.log|' /home/nudanos/terminal-probe.sh >/home/serialtwo/terminal-probe.sh
chmod 755 /home/serialtwo/terminal-probe.sh
chown serialtwo:users /home/serialtwo/terminal-probe.sh
pid=$(systemctl show serial-getty@ttyS0 -p MainPID --value)
test "$(cat /proc/$pid/comm)" = agetty
ps -o pid,ppid,sid,tty,args -p "$pid"
mkdir -p /var/tmp/serial-fixed-trace
nohup timeout -s INT -k 5 90 env LD_LIBRARY_PATH=/var/tmp/legacy-trace/tool/usr/lib/x86_64-linux-gnu /var/tmp/legacy-trace/tool/usr/bin/strace -ff -o /var/tmp/serial-fixed-trace/trace -p "$pid" </dev/null >/var/tmp/serial-fixed-trace/tracer.log 2>&1 &
echo "BOUNDED_FIXED_TRACER=$!"
