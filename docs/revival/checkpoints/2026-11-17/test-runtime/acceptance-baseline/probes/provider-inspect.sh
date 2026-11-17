#!/bin/sh
set -u
date -u
command -v last || true
dpkg-query -W vyatta-login vyatta-system-login-v1-yang vyatta-op-show-system-login-v1-yang
/usr/sbin/faillock --dir /run/vyatta-faillock --user locktest
perl -I/opt/vyatta/share/perl5 -MVyatta::Login::Faillock=tally_counts -e 'my %c=tally_counts(); for my $u (sort keys %c) { print "$u $c{$u}\n" }'
timeout 30 /opt/vyatta/sbin/system-login-information --action=get-state >/tmp/login-state.json 2>/tmp/login-state.err
rc=$?
echo PROVIDER_EXIT_$rc
cat /tmp/login-state.err
python3 -c 'import json; d=json.load(open("/tmp/login-state.json")); print("PROVIDER_LOCKTEST",*[x for x in d.get("user",[]) if x.get("name")=="locktest"])' 2>&1 || true
grep -n 'system-login-information' /opt/vyatta/share/yang/modules/vyatta-system-login-v1.yang 2>&1 || true
echo PROVIDER_INSPECT_DONE
