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

// Files written into bind mounts are chowned to OwnerIDs. Under rootless
// podman, container uid 0 already is the invoking user; chowning to the host
// uid would map to a subordinate uid the user cannot delete.
func TestOwnerIDs(t *testing.T) {
	if uid, gid := (Engine{Bin: "podman"}).OwnerIDs(501, 20); uid != 0 || gid != 0 {
		t.Errorf("podman OwnerIDs = %d:%d, want 0:0", uid, gid)
	}
	if uid, gid := (Engine{Bin: "docker"}).OwnerIDs(501, 20); uid != 501 || gid != 20 {
		t.Errorf("docker OwnerIDs = %d:%d, want 501:20", uid, gid)
	}
}

func TestBuildArgsLabelsTheImage(t *testing.T) {
	got := Engine{Bin: "docker"}.BuildArgs("builder", "nudanos/builder:trixie", map[string]string{"org.nudanos.builder-hash": "abc"})
	want := []string{"build", "--pull", "--label", "org.nudanos.builder-hash=abc", "-t", "nudanos/builder:trixie", "builder"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildArgs = %v, want %v", got, want)
	}
}

func TestLabelArgs(t *testing.T) {
	got := Engine{Bin: "podman"}.LabelArgs("img", "org.nudanos.builder-hash")
	want := []string{"image", "inspect", "--format", `{{ index .Config.Labels "org.nudanos.builder-hash" }}`, "img"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LabelArgs = %v, want %v", got, want)
	}
}

func TestRunArgsPrivilegedAndDevices(t *testing.T) {
	got := Engine{Bin: "docker"}.RunArgs(RunSpec{Image: "img", Privileged: true, Devices: []string{"/dev/kvm"}})
	want := []string{"run", "--rm", "--privileged", "--device", "/dev/kvm", "img"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunArgs = %v, want %v", got, want)
	}
}
