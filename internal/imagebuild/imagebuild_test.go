package imagebuild

import "testing"

func TestSpecIsPrivilegedWithReadOnlyInputs(t *testing.T) {
	s := Spec("nudanos/builder:trixie", "/w/repo", "/d/image", "/w/image", "1.0~20261001", "1790000000", 501, 20)
	if !s.Privileged {
		t.Error("live-build needs a privileged container")
	}
	ro := map[string]bool{}
	for _, m := range s.Mounts {
		ro[m.Container] = m.ReadOnly
	}
	if !ro["/repo"] || !ro["/image"] || ro["/out"] {
		t.Errorf("mounts = %+v (want /repo and /image read-only, /out writable)", s.Mounts)
	}
	if s.Env["NUDANOS_VERSION"] != "1.0~20261001" || s.Env["SOURCE_DATE_EPOCH"] != "1790000000" {
		t.Errorf("env = %v", s.Env)
	}
}

func TestSpecRunsMakeImageFromTheConfigMount(t *testing.T) {
	s := Spec("nudanos/builder:trixie", "/w/repo", "/d/image", "/w/image", "1.0~20261001", "1790000000", 501, 20)
	if len(s.Cmd) != 2 || s.Cmd[0] != "bash" || s.Cmd[1] != "/image/make-image.sh" {
		t.Errorf("cmd = %v (want bash /image/make-image.sh, so the builder image stays unchanged)", s.Cmd)
	}
}
