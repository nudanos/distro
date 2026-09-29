package aptmirror

import (
	"reflect"
	"testing"

	"github.com/nudanos/distro/internal/engine"
	"github.com/nudanos/distro/internal/manifest"
)

var frr = manifest.Entry{Name: "frr", Kind: manifest.Apt,
	Source: "https://deb.frrouting.org/frr trixie frr-stable", Key: "https://deb.frrouting.org/frr/keys.gpg",
	Packages: map[string]string{"libyang3": "3.13.6-1~deb13u1", "frr": "10.7.1-0~deb13u1"}}

func TestSpec(t *testing.T) {
	got := Spec("img", frr, "/w/out/frr", 501, 20)
	want := engine.RunSpec{Image: "img",
		Mounts: []engine.Mount{{Host: "/w/out/frr", Container: "/out"}},
		Env: map[string]string{"SOURCE": frr.Source, "KEY_URL": frr.Key,
			"PINS": "frr=10.7.1-0~deb13u1 libyang3=3.13.6-1~deb13u1", "HOST_UID": "501", "HOST_GID": "20"},
		Cmd: []string{"/usr/local/bin/mirror-apt"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Spec =\n %+v\nwant\n %+v", got, want)
	}
}

func TestKeyChangesWithPins(t *testing.T) {
	k1 := Key(frr)
	bumped := frr
	bumped.Packages = map[string]string{"libyang3": "3.13.6-1~deb13u1", "frr": "10.7.2-0~deb13u1"}
	if Key(bumped) == k1 {
		t.Error("pin change did not change key")
	}
	if Key(frr) != k1 {
		t.Error("key not stable")
	}
}
