package installtest

import "testing"

func TestSpecMountsRepoAndTestsReadOnly(t *testing.T) {
	s := Spec("/w/repo", "/d/tests/integration")
	if s.Image != "debian:trixie" || len(s.Mounts) != 2 || !s.Mounts[0].ReadOnly || !s.Mounts[1].ReadOnly {
		t.Fatalf("spec = %+v", s)
	}
	if s.Cmd[len(s.Cmd)-1] != "/tests/install-purge.sh" {
		t.Errorf("cmd = %v", s.Cmd)
	}
}
