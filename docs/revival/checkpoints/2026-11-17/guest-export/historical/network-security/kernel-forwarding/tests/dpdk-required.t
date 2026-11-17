#!/usr/bin/perl
# The op scripts of vplane-config (DPDK-only, not installable here) are
# replaced by dpdk-required, which says why the command cannot run.
use strict;
use warnings;
use File::Temp qw(tempdir);
use Test::More;

my @scripts = qw(nbr-res-flush.pl vplane-arp vplane-gre.pl vplane-ifconfig.pl
  vplane-monitor-cmd.pl vplane-monitor.pl vplane-nd.pl vplane-route.pl
  vplane-show-memory.pl vplane-statistics.pl);

open my $fh, '<', 'debian/vyatta-kernel-forwarding.links'
  or die "debian/vyatta-kernel-forwarding.links: $!";
my %links;
while (<$fh>) {
    my ( $src, $dst ) = split;
    $links{$dst} = $src if $dst;
}
for my $s (@scripts) {
    is( $links{"opt/vyatta/bin/$s"},
        'usr/lib/vyatta-kernel-forwarding/dpdk-required', "$s is linked" );
}

my $dir = tempdir( CLEANUP => 1 );
symlink( "$ENV{PWD}/dpdk-required", "$dir/vplane-route.pl" ) or die $!;
my $out = `"$dir/vplane-route.pl" --all 2>&1`;
isnt( $? >> 8, 0, 'exits non-zero' );
like( $out, qr/requires the DPDK dataplane; this system forwards in the Linux kernel/,
    'names the reason' );
done_testing();
