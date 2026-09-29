package versions

import (
	"regexp"
	"testing"
)

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"2.4.3", "2.4.10", -1}, {"0.2.0", "0.1.1", 1}, {"1.2.5", "1_2_5", 0},
		{"5.9.5.2", "5.9.5", 1}, {"1.32", "1.31", 1}, {"2.0", "2.0.0", -1},
	} {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestLatestHonoursThePattern(t *testing.T) {
	// mstpd: 0.03-0.05 are ancient two-part tags that must not beat 0.2.0.
	mstpd := []string{"0.0.6", "0.0.9", "0.03", "0.04", "0.05", "0.1.0", "0.1.1", "0.2.0"}
	tag, v, ok := Latest(mstpd, regexp.MustCompile(`^(\d+\.\d+\.\d+)$`))
	if !ok || tag != "0.2.0" || v != "0.2.0" {
		t.Errorf("mstpd latest = %q %q %v", tag, v, ok)
	}
	ntp := []string{"NTPsec_1_2_4", "NTPsec_1_2_5", "NTPsec_1_2_5_rc1"}
	tag, v, ok = Latest(ntp, regexp.MustCompile(`^NTPsec_(\d+_\d+_\d+)$`))
	if !ok || tag != "NTPsec_1_2_5" || v != "1.2.5" {
		t.Errorf("ntpsec latest = %q %q %v", tag, v, ok)
	}
	if _, _, ok := Latest([]string{"foo"}, regexp.MustCompile(`^v(\d+)$`)); ok {
		t.Error("no matching tag should report ok=false")
	}
}
