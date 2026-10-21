package lint

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// The marker lines distro-build and the Go ports' builds write into
// work/logs/<name>.log, so a log that predates them is recognised.
const (
	FlagsMarker       = "nudanos-lint-flags: wall-wextra"
	StaticcheckPrefix = "nudanos-staticcheck: "
	StaticcheckDone   = "nudanos-staticcheck: done"
)

var (
	ccWarning   = regexp.MustCompile(`^(.+?):(\d+):\d+: warning: (.*) \[(-W[^\]]+)\]$`)
	staticcheck = regexp.MustCompile(`^(.+?):(\d+):\d+: (.*) \(([A-Z]+\d+)\)$`)
)

// buildDir is the package source's path in the builder (dpkg's
// -ffile-prefix-map shows it): warnings may name files under it.
const buildDir = "/build/pkg/"

// FromBuildLog returns the findings in a package build log: compiler
// warnings for a port with C (langs["c"]) and staticcheck results for a port
// with Go. A C port's log without FlagsMarker predates the lint flags and is
// an error; a Go port's log without StaticcheckDone gives one
// "staticcheck-not-run" finding. A port with neither needs no log.
func FromBuildLog(port string, langs map[string]bool, log []byte) ([]Finding, error) {
	if !langs["c"] && !langs["go"] {
		return nil, nil
	}
	if langs["c"] && !bytes.HasPrefix(log, []byte(FlagsMarker+"\n")) {
		return nil, fmt.Errorf("%s: build log predates lint flags, rebuild %s", port, port)
	}
	var out []Finding
	seen := map[string]bool{}
	add := func(f Finding) {
		k := fmt.Sprintf("%s|%s:%d|%s", f.Tool, f.File, f.Line, f.Rule)
		if !seen[k] {
			seen[k] = true
			out = append(out, f)
		}
	}
	ranStaticcheck := false
	sc := bufio.NewScanner(bytes.NewReader(log))
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		l := strings.TrimRight(sc.Text(), "\r")
		if l == StaticcheckDone {
			ranStaticcheck = true
			continue
		}
		if rest, ok := strings.CutPrefix(l, StaticcheckPrefix); ok {
			if strings.HasPrefix(rest, "\t") {
				// related information for the finding above it
				continue
			}
			m := staticcheck.FindStringSubmatch(rest)
			if m == nil {
				// not a finding: staticcheck did not run or could not load
				// the packages, so the port was not checked
				return nil, fmt.Errorf("%s: staticcheck did not check the build: %s", port, rest)
			}
			add(Finding{Port: port, Tool: "staticcheck", File: strings.TrimPrefix(m[1], buildDir),
				Line: atoi(m[2]), Rule: m[4], Message: m[3]})
			continue
		}
		if !langs["c"] {
			continue
		}
		m := ccWarning.FindStringSubmatch(l)
		if m == nil || strings.HasPrefix(m[1], "/usr/") {
			continue
		}
		add(Finding{Port: port, Tool: "cc", File: strings.TrimPrefix(m[1], buildDir),
			Line: atoi(m[2]), Rule: m[4], Message: m[3]})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if langs["go"] && !ranStaticcheck {
		out = append(out, Finding{Port: port, Tool: "lint", File: "debian/rules", Rule: "staticcheck-not-run",
			Message: "the build does not run staticcheck (docs/porting.md)"})
	}
	return out, nil
}
