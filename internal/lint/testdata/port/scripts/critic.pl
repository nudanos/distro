#!/usr/bin/perl
use strict;
use warnings;

my $x = 1;
print "$x\n" if $x;
exit 0;

sub f {
    my ($y) = @_;
    $y++;
}
