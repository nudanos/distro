package scenario

import (
	"strings"
	"testing"
)

const bgpSummary = `IPv4 Unicast Summary:
BGP router identifier 1.1.1.1, local AS number 65001 vrf-id 0
BGP table version 7
RIB entries 5, using 960 bytes of memory
Peers 1, using 21 KiB of memory

Neighbor        V         AS   MsgRcvd   MsgSent   TblVer  InQ OutQ  Up/Down State/PfxRcd
10.0.12.2       4      65002        25        27        7    0    0 00:05:12            2
10.0.13.2       4      65003       812       830        7    0    0 1d02h03m  Established
`

const ospfNeighbor = `Neighbor ID     Pri State           Dead Time Address         Interface            RXmtL RqstL DBsmL
10.255.0.2        1 Full/DR           35.512s 10.0.12.2       dp0s3:10.0.12.1          0     0     0
`

const ipRoute = `Codes: K - kernel route, C - connected, S - static, O - OSPF, B - BGP
O>* 10.255.0.3/32 [110/20] via 10.0.12.2, dp0s3, weight 1, 00:03:07
C>* 10.0.12.0/24 is directly connected, dp0s3, 00:04:00
`

func TestNormalizeMasksUptime(t *testing.T) {
	got := Normalize("show ip bgp summary", bgpSummary)
	for _, gone := range []string{"00:05:12", "1d02h03m", "  25 ", "  812 ", "960 bytes", "21 KiB"} {
		if strings.Contains(got, gone) {
			t.Errorf("normalised output still contains %q:\n%s", gone, got)
		}
	}
	if Normalize("show ip bgp summary", bgpSummary) != got {
		t.Error("Normalize is not deterministic")
	}
}

// Review Focus 5: masking must not hide the facts a scenario is about.
func TestNormalizeKeepsRoutingFacts(t *testing.T) {
	cases := map[string][]string{
		"show ip bgp summary":   {"10.0.12.2", "65001", "65002", "65003", "Established", "1.1.1.1"},
		"show ip ospf neighbor": {"10.255.0.2", "Full/DR", "10.0.12.2", "dp0s3"},
		"show ip route":         {"10.255.0.3/32", "[110/20]", "via 10.0.12.2", "10.0.12.0/24", "dp0s3"},
	}
	inputs := map[string]string{"show ip bgp summary": bgpSummary, "show ip ospf neighbor": ospfNeighbor, "show ip route": ipRoute}
	for cmd, facts := range cases {
		got := Normalize(cmd, inputs[cmd])
		for _, f := range facts {
			if !strings.Contains(got, f) {
				t.Errorf("%s: normalising lost %q:\n%s", cmd, f, got)
			}
		}
	}
	if strings.Contains(Normalize("show ip ospf neighbor", ospfNeighbor), "35.512s") {
		t.Error("the OSPF dead timer was not masked")
	}
}

func TestDiffEmptyWhenEqual(t *testing.T) {
	if d := Diff("a\nb\n", "a\nb\n"); d != "" {
		t.Errorf("Diff of equal text = %q", d)
	}
}

func TestDiffShowsChangedLine(t *testing.T) {
	d := Diff("a\nb\nc\n", "a\nB\nc\n")
	for _, want := range []string{"--- 2105", "+++ nudanos", "-b", "+B"} {
		if !strings.Contains(d, want) {
			t.Errorf("diff lacks %q:\n%s", want, d)
		}
	}
	if strings.Contains(d, "-a") || strings.Contains(d, "+c") {
		t.Errorf("diff shows unchanged lines:\n%s", d)
	}
}

func TestCheckAcceptedPassesExactMatch(t *testing.T) {
	d := Diff("a\n", "b\n")
	if err := CheckAccepted(d, d); err != nil {
		t.Error(err)
	}
	if err := CheckAccepted("", ""); err != nil {
		t.Error(err)
	}
}

func TestCheckAcceptedFailsOnNewDifference(t *testing.T) {
	err := CheckAccepted(Diff("a\n", "b\n"), "")
	if err == nil || !strings.Contains(err.Error(), "+b") {
		t.Errorf("err = %v, want the unaccepted diff", err)
	}
}

// FRR prints neighbor uptimes as 14m41s or 1h02m03s, and show vrrp the
// time since the last transition as 38s; they change from run to run.
func TestNormalizeMasksCompactDurations(t *testing.T) {
	cases := map[string]string{
		"dp0s3             10     MASTER  no         no     38s         <none>\n":   "dp0s3 10 MASTER no no <time> <none>\n",
		"10.255.0.2        1 Full/Backup     14m41s            33.080s 10.0.12.2\n": "10.255.0.2 1 Full/Backup <time> <time> 10.0.12.2\n",
		"neighbor up 1h02m03s\n":       "neighbor up <time>\n",
		"area 10 cost 100 via dp0s3\n": "area 10 cost 100 via dp0s3\n",
	}
	for in, want := range cases {
		if got := Normalize("show vrrp", in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

// Column padding follows the width of values that are masked anyway
// ("38s" vs "18m41s"), so runs of spaces compare as one.
func TestNormalizeCollapsesPadding(t *testing.T) {
	a := Normalize("show vrrp", "dp0s3             10     MASTER  no         no     38s         <none>\n")
	b := Normalize("show vrrp", "dp0s3             10     MASTER  no         no     1m38s       <none>\n")
	if a != b {
		t.Errorf("padding differs:\n%q\n%q", a, b)
	}
	if want := "dp0s3 10 MASTER no no <time> <none>\n"; a != want {
		t.Errorf("got %q, want %q", a, want)
	}
}

// DANOS's own form of the BGP summary ("show protocols bgp all summary")
// carries the same run-to-run counters as "show ip bgp summary".
func TestNormalizeMasksDANOSBGPSummaryCounters(t *testing.T) {
	a := Normalize("show protocols bgp all summary", "201.1.1.4 4 200 18 20 0 0 0 00:05:01 0 1\n")
	b := Normalize("show protocols bgp all summary", "201.1.1.4 4 200 16 17 7 0 0 00:04:12 0 1\n")
	if a != b {
		t.Errorf("counters not masked:\n%q\n%q", a, b)
	}
}
