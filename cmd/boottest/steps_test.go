package main

import (
	"testing"
)

// configd saves on commit; "save" then says so (text from a layer 3 transcript).
func TestSavedMatchesCommitSavesConfiguration(t *testing.T) {
	out := "save\r\n\r\n  'commit' saves configuration.  This command has no effect\r\n\r\n[edit]\r\r\nvyatta@node# "
	if !saved.MatchString(out) {
		t.Errorf("saved does not match %q", out)
	}
}

// Bytes copied from layer 3 run 19 (od -c of the serial transcript).
func TestVersionLineMatchesTheRealConsole(t *testing.T) {
	out := "\x1b[?2004hvyatta@node:~$ show version\r\n\x1b[?2004l\r\rVersion:      1.0-20261004.0334\x1b[m\r\nDescription:  NuDanOS 1.0~20261004\x1b[m\r\n"
	if !versionLine.MatchString(out) {
		t.Errorf("versionLine does not match %q", out)
	}
	if versionLine.MatchString("Version:      UNKNOWN\r\n") {
		t.Error("versionLine matches UNKNOWN")
	}
}
