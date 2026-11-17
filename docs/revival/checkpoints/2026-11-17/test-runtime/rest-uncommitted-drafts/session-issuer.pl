#!/usr/bin/perl
# SPDX-License-Identifier: GPL-2.0-only
use strict;
use warnings;
use JSON;
use Fcntl;
# This installed configuration binding is unused by SessionKey's issuer. Keep
# App::* real, without requiring the unrelated config engine in this unit test.
BEGIN { $INC{'Vyatta/Config.pm'} = __FILE__; }
require './lib/Vyatta/WebGUI2/SessionKey.pm';

my $case = shift // die 'case required';
mkdir '/run/gui', 0700 or die $!;
sub issue {
    my ($uid) = @_;
    my $pid = fork();
    die $! unless defined $pid;
    if (!$pid) {
        open STDOUT, '>', '/run/result' or die $!;
        if ($uid) {
            POSIX::setgid($uid) == 0 or die $!;
            POSIX::setuid($uid) == 0 or die $!;
        }
        get_key('root', 'service-user', '127.0.0.1');
        die 'issuer returned without exit';
    }
    waitpid($pid, 0) == $pid or die $!;
    return $?;
}
sub contents {
    my ($path) = @_;
    open my $fh, '<', $path or die "$path: $!";
    local $/;
    return <$fh>;
}
sub write_file {
    my ($path, $data, $mode) = @_;
    open my $fh, '>', $path or die $!;
    print $fh $data or die $!;
    close $fh or die $!;
    chmod $mode, $path or die $!;
}
if ($case eq 'valid') {
    issue(0) == 0 or die 'root issuance failed';
    my $result = decode_json(contents('/run/result'));
    die 'wrong user/token' unless $result->{session}->{user} eq 'root'
        && $result->{session}->{key} =~ /^[0-9A-F]{16}$/;
    my @s = stat '/run/gui/session';
    die 'session must be root-owned 0600' unless $s[4] == 0 && ($s[2] & 0777) == 0600;
    my $key = $result->{session}->{key};
    issue(0) == 0 or die 'reuse failed';
    die 'legacy session reuse changed' unless decode_json(contents('/run/result'))->{session}->{key} eq $key;
} elsif ($case eq 'unprivileged') {
    chmod 0777, '/run/gui' or die $!;
    issue(65534) != 0 or die 'unprivileged caller issued root session';
    die 'unprivileged caller created authority' if -e '/run/gui/session';
} elsif ($case eq 'unsafe-directory') {
    chmod 0777, '/run/gui' or die $!;
    issue(0) != 0 or die 'accepted unsafe directory';
    die 'created state in unsafe directory' if -e '/run/gui/session';
} elsif ($case eq 'symlink') {
    write_file('/run/victim', 'unchanged', 0600);
    symlink '/run/victim', '/run/gui/session_tmp' or die $!;
    issue(0) != 0 or die 'accepted temporary symlink';
    die 'clobbered symlink target' unless contents('/run/victim') eq 'unchanged';
} elsif ($case eq 'lock') {
    write_file('/run/gui/session.lck', '', 0600);
    issue(0) != 0 or die 'ignored existing lock';
    die 'removed another issuer lock' unless -e '/run/gui/session.lck';
} else { die "unknown case $case"; }
print "session-issuer: $case OK\n";
