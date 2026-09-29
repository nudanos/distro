package engine

import (
	"reflect"
	"testing"
)

func TestRunArgs(t *testing.T) {
	got := Engine{Bin: "docker"}.RunArgs(RunSpec{
		Image:   "nudanos/builder:trixie",
		Mounts:  []Mount{{Host: "/w/src/a", Container: "/src", ReadOnly: true}, {Host: "/w/out/a", Container: "/out"}},
		Env:     map[string]string{"PKG": "a", "HOST_UID": "501"},
		Workdir: "/build",
		Network: "none",
		Cmd:     []string{"/usr/local/bin/build-package"},
	})
	want := []string{"run", "--rm", "--network", "none",
		"-v", "/w/src/a:/src:ro", "-v", "/w/out/a:/out",
		"-e", "HOST_UID=501", "-e", "PKG=a",
		"-w", "/build", "nudanos/builder:trixie", "/usr/local/bin/build-package"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunArgs =\n %v\nwant\n %v", got, want)
	}
}
