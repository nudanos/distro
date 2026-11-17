#!/bin/sh
set -u
date -u
systemctl show -p MainPID -p ActiveState configd
perl -I/opt/vyatta/share/perl5 -MVyatta::Configd -MJSON -e 'my $c=Vyatta::Configd::Client->new(); for my $db (undef,$Vyatta::Configd::RUNNING) { my $o=defined($db) ? $c->tree_get_full_hash("system login user-info",{database=>$db}) : $c->tree_get_full_hash("system login user-info"); print defined($db)?"RUNNING ":"AUTO ",encode_json($o),"\n"; }' 2>&1
echo CONFIGD_JOURNAL
journalctl -u configd -n 80 --no-pager 2>&1
echo TREE_DEBUG_DONE
