package manifest

import "testing"

// The committed manifest must always load, and the pilot must be what is ready.
func TestRepoManifest(t *testing.T) {
	m, err := Load("../../manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ready := map[string]bool{}
	for _, k := range []Kind{Danos, Apt} {
		for _, e := range m.Ready(k) {
			ready[e.Name] = true
		}
	}
	for _, n := range []string{"dh-yang", "dh-vci", "vyatta-util", "frr"} {
		if !ready[n] {
			t.Errorf("%s should be ready", n)
		}
	}
	if len(ready) != 4 {
		t.Errorf("ready entries = %v, want only the pilot", ready)
	}
}
