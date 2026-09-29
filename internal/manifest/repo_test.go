package manifest

import "testing"

// The committed manifest must always load; the pilot stays ready; every ready
// upstream entry carries full pins (Validate enforces the fields).
func TestRepoManifest(t *testing.T) {
	m, err := Load("../../manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ready := map[string]bool{}
	for _, k := range []Kind{Danos, Apt, Upstream} {
		for _, e := range m.Ready(k) {
			ready[e.Name] = true
		}
	}
	for _, n := range []string{"dh-yang", "dh-vci", "vyatta-util", "frr", "lintian-profile-vyatta"} {
		if !ready[n] {
			t.Errorf("%s should be ready", n)
		}
	}
}
