// Package installtest runs the layer 2 install/remove/purge test (spec 8.2).
package installtest

import "github.com/nudanos/distro/internal/engine"

// Spec runs tests/integration/install-purge.sh in a clean debian:trixie
// container against the signed repo in repoDir.
func Spec(repoDir, testsDir string) engine.RunSpec {
	return engine.RunSpec{Image: "debian:trixie",
		Mounts: []engine.Mount{
			{Host: repoDir, Container: "/repo", ReadOnly: true},
			{Host: testsDir, Container: "/tests", ReadOnly: true},
		},
		Cmd: []string{"bash", "/tests/install-purge.sh"}}
}
