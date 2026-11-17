#!/bin/sh
set -u
opts='Dir::Etc::sourcelist=/etc/apt/sources.list.d/interim.list'
apt-get -s -o "$opts" -o Dir::Etc::sourceparts=- install vyatta-login=2.45 >/tmp/solver-host.log 2>&1
rc=$?
echo HOST_SOLVER_EXIT_$rc
grep -E '(^Inst (vyatta-login|vyatta-system-login-v1-yang|configd)|unmet dependencies|Unable to)' /tmp/solver-host.log || true
apt-get -s -o "$opts" -o Dir::Etc::sourceparts=- install vyatta-system-login-v1-yang=2.45 >/tmp/solver-yang.log 2>&1
rc=$?
echo YANG_SOLVER_EXIT_$rc
grep -E '(^Inst (vyatta-login|vyatta-system-login-v1-yang|configd)|unmet dependencies|Unable to)' /tmp/solver-yang.log || true
echo SOLVERS_COMPLETE
