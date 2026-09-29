package aptrepo

import (
	"reflect"
	"testing"

	"github.com/nudanos/distro/internal/engine"
)

func TestSpec(t *testing.T) {
	got := Spec("img", "/w/out", "/w/repo", "/h/gnupg", "ABCD", 501, 20)
	want := engine.RunSpec{Image: "img",
		Mounts: []engine.Mount{{Host: "/w/out", Container: "/pool", ReadOnly: true},
			{Host: "/w/repo", Container: "/repo"}, {Host: "/h/gnupg", Container: "/gnupg", ReadOnly: true}},
		Env:     map[string]string{"KEY_ID": "ABCD", "SUITE": "trixie", "HOST_UID": "501", "HOST_GID": "20"},
		Network: "none",
		Cmd:     []string{"/usr/local/bin/make-repo"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Spec =\n %+v\nwant\n %+v", got, want)
	}
}
