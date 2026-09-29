// Package aptrepo publishes build output as a signed apt repository.
package aptrepo

import (
	"strconv"

	"github.com/nudanos/distro/internal/engine"
)

// Spec is the (network-less) container run that indexes outRoot into repoDir
// and signs it with keyID from gnupgDir.
func Spec(image, outRoot, repoDir, gnupgDir, keyID string, uid, gid int) engine.RunSpec {
	return engine.RunSpec{Image: image,
		Mounts: []engine.Mount{
			{Host: outRoot, Container: "/pool", ReadOnly: true},
			{Host: repoDir, Container: "/repo"},
			{Host: gnupgDir, Container: "/gnupg", ReadOnly: true},
		},
		Env: map[string]string{"KEY_ID": keyID, "SUITE": "trixie",
			"HOST_UID": strconv.Itoa(uid), "HOST_GID": strconv.Itoa(gid)},
		Network: "none",
		Cmd:     []string{"/usr/local/bin/make-repo"}}
}
