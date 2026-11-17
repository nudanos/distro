#!/bin/sh
set -u
date -u
dpkg-query -W vyatta-login vyatta-system-login-v1-yang vyatta-op-show-system-login-v1-yang 2>&1
/usr/sbin/faillock --dir /run/vyatta-faillock --user locktest
perl -I/opt/vyatta/share/perl5 -MVyatta::Login::Faillock=tally_counts -e 'my %c=tally_counts(); for my $u (sort keys %c) { print "$u $c{$u}\n" }'
echo TALLY_INSPECT_DONE
