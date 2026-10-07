package scenario

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

var (
	timeFields = []*regexp.Regexp{
		regexp.MustCompile(`\b\d+w\d+d\d+h\b`),
		regexp.MustCompile(`\b\d+d\d+h\d+m\b`),
		regexp.MustCompile(`\b\d{1,2}:\d{2}:\d{2}\b`),
		regexp.MustCompile(`\b\d+\.\d+s\b`),          // OSPF dead timer
		regexp.MustCompile(`\b(\d+h)?(\d+m)?\d+s\b`), // FRR uptime 14m41s, show vrrp 38s
	}
	memoryUse   = regexp.MustCompile(`using \d+ (bytes|KiB|MiB) of memory`)
	tableVer    = regexp.MustCompile(`(BGP table version( is)?) \d+`)
	anyNumber   = regexp.MustCompile(`\b\d+\b`)
	trailingSpc = regexp.MustCompile(`[ \t]+\n`)
	innerPad    = regexp.MustCompile(`(\S)[ \t]{2,}`)
)

// Normalize replaces the parts of a show command's output that change from
// run to run (uptimes, timers, counters, memory use) with placeholders, and
// leaves everything a scenario is about (prefixes, AS numbers, next hops,
// states) alone.
func Normalize(command, output string) string {
	out := output
	for _, re := range timeFields {
		out = re.ReplaceAllString(out, "<time>")
	}
	out = memoryUse.ReplaceAllString(out, "using <n> of memory")
	out = tableVer.ReplaceAllString(out, "$1 <n>")
	switch {
	case strings.HasPrefix(command, "show ip bgp summary"), strings.HasPrefix(command, "show bgp summary"):
		out = maskBGPCounters(out)
	case strings.HasPrefix(command, "show dataplane"), strings.Contains(command, "counters"), strings.Contains(command, "statistics"):
		out = maskCounters(out)
	}
	if !strings.HasSuffix(out, "\n") && out != "" {
		out += "\n"
	}
	// column padding follows the width of values masked above; leading
	// indentation (FRR's nesting) is kept
	out = innerPad.ReplaceAllString(out, "$1 ")
	return trailingSpc.ReplaceAllString(out, "\n")
}

// maskBGPCounters masks MsgRcvd, MsgSent and TblVer on neighbor lines
// (Neighbor V AS MsgRcvd MsgSent TblVer InQ OutQ Up/Down State/PfxRcd).
func maskBGPCounters(out string) string {
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		f := strings.Fields(l)
		if len(f) >= 6 && net.ParseIP(f[0]) != nil {
			f[3], f[4], f[5] = "<n>", "<n>", "<n>"
			lines[i] = strings.Join(f, " ")
		}
	}
	return strings.Join(lines, "\n")
}

// maskCounters masks every bare number on lines that name an interface.
func maskCounters(out string) string {
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		if f := strings.Fields(l); len(f) > 1 && strings.HasPrefix(f[0], "dp0") {
			lines[i] = f[0] + anyNumber.ReplaceAllString(strings.TrimPrefix(l, f[0]), "<n>")
		}
	}
	return strings.Join(lines, "\n")
}

// Diff lists the lines that differ between the 2105 reference (want) and
// NuDanOS's output (got), in order, "-" for 2105 and "+" for NuDanOS; it is
// "" when they are equal. No context lines, so an accepted.diff only names
// what really differs.
func Diff(want, got string) string {
	if want == got {
		return ""
	}
	a, b := strings.Split(want, "\n"), strings.Split(got, "\n")
	// longest common subsequence table
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var sb strings.Builder
	sb.WriteString("--- 2105\n+++ nudanos\n")
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			i++
			j++
		case j < len(b) && (i == len(a) || lcs[i][j+1] >= lcs[i+1][j]):
			sb.WriteString("+" + b[j] + "\n")
			j++
		default:
			sb.WriteString("-" + a[i] + "\n")
			i++
		}
	}
	return sb.String()
}

// CheckAccepted passes when diff is exactly the reviewed accepted.diff
// (both may be empty).
func CheckAccepted(diff, accepted string) error {
	if diff == accepted {
		return nil
	}
	return fmt.Errorf("show output differs from 2105 beyond accepted.diff:\n%s", diff)
}
