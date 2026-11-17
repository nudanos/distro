#!/bin/sh
set -u
printf 'LAST_IDENTITY\n'
ls -l /usr/bin/last
readlink -f /usr/bin/last
dpkg-query -S /usr/bin/last 2>&1 || true
/usr/bin/last -R --time-format iso >/tmp/last.stdout 2>/tmp/last.stderr
rc=$?
echo LAST_EXIT_$rc
echo LAST_STDOUT
cat /tmp/last.stdout
echo LAST_STDERR
cat /tmp/last.stderr
echo LAST_INSPECT_DONE
