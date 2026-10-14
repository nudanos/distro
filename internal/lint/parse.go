package lint

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// lineParser turns one output line into a finding. skip reports noise that
// is neither a finding nor an error; err reports a file the tool could not
// check.
type lineParser func(line string) (f *Finding, skip bool, err error)

var (
	perlcriticLine = regexp.MustCompile(`^(.+?):(\d+):\d+:([A-Za-z0-9_]+(?:::[A-Za-z0-9_]+)*):(.*)$`)
	shellcheckLine = regexp.MustCompile(`^(.+?):(\d+):\d+: \w+: (.*) \[(SC\d+)\]$`)
	flake8Line     = regexp.MustCompile(`^(.+?):(\d+):\d+: ([A-Z]+\d+) (.*)$`)
	pylintLine     = regexp.MustCompile(`^(.+?):(\d+): \[([A-Z]\d+)\([^)]*\), [^\]]*\] (.*)$`)
	pylintNoise    = regexp.MustCompile(`^(\*+ Module |-+$|Your code has been rated)`)
	cppcheckLine   = regexp.MustCompile(`^(.+?):(\d+):([A-Za-z0-9_]+):(.*)$`)
)

// cppcheckErrors are the ids cppcheck uses when it could not analyse a file.
var cppcheckErrors = map[string]bool{"syntaxError": true, "internalError": true,
	"internalAstError": true, "cppcheckError": true, "unknownMacro": true}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func fileList(tool, msg string) lineParser {
	return func(l string) (*Finding, bool, error) {
		return &Finding{Tool: tool, File: l, Rule: tool, Message: msg}, false, nil
	}
}

var parsers = map[string]lineParser{
	// gofmt -l, and the runner's list of files perltidy would change
	"gofmt":    fileList("gofmt", "not gofmt-formatted"),
	"perltidy": fileList("perltidy", "not perltidy-formatted"),
	"perlcritic": func(l string) (*Finding, bool, error) {
		if strings.HasSuffix(l, " source OK") {
			return nil, true, nil
		}
		m := perlcriticLine.FindStringSubmatch(l)
		if m == nil {
			return nil, false, fmt.Errorf("perlcritic: %s", l)
		}
		return &Finding{Tool: "perlcritic", File: m[1], Line: atoi(m[2]), Rule: m[3], Message: m[4]}, false, nil
	},
	"shellcheck": func(l string) (*Finding, bool, error) {
		m := shellcheckLine.FindStringSubmatch(l)
		if m == nil {
			return nil, false, fmt.Errorf("shellcheck: %s", l)
		}
		return &Finding{Tool: "shellcheck", File: m[1], Line: atoi(m[2]), Rule: m[4], Message: m[3]}, false, nil
	},
	"flake8": func(l string) (*Finding, bool, error) {
		m := flake8Line.FindStringSubmatch(l)
		if m == nil {
			return nil, false, fmt.Errorf("flake8: %s", l)
		}
		if m[3] == "E999" { // flake8's syntax error: the file was not checked
			return nil, false, fmt.Errorf("flake8: %s", l)
		}
		return &Finding{Tool: "flake8", File: m[1], Line: atoi(m[2]), Rule: m[3], Message: m[4]}, false, nil
	},
	"pylint": func(l string) (*Finding, bool, error) {
		if pylintNoise.MatchString(l) {
			return nil, true, nil
		}
		m := pylintLine.FindStringSubmatch(l)
		if m == nil {
			return nil, false, fmt.Errorf("pylint: %s", l)
		}
		if m[3] == "E0001" || strings.HasPrefix(m[3], "F") { // syntax error, fatal
			return nil, false, fmt.Errorf("pylint: %s", l)
		}
		return &Finding{Tool: "pylint", File: m[1], Line: atoi(m[2]), Rule: m[3], Message: m[4]}, false, nil
	},
	"cppcheck": func(l string) (*Finding, bool, error) {
		m := cppcheckLine.FindStringSubmatch(l)
		if m == nil {
			return nil, false, fmt.Errorf("cppcheck: %s", l)
		}
		if cppcheckErrors[m[3]] {
			return nil, false, fmt.Errorf("cppcheck: %s", l)
		}
		return &Finding{Tool: "cppcheck", File: m[1], Line: atoi(m[2]), Rule: m[3], Message: m[4]}, false, nil
	},
}

// Parse reads one tool's output, in the format cmd/lint asks the tool for,
// into findings (Port left empty). A line it does not recognise, or one that
// says the tool could not check a file, is an error: an unchecked file must
// never count as a clean one.
func Parse(tool string, out []byte) ([]Finding, error) {
	p, ok := parsers[tool]
	if !ok {
		return nil, fmt.Errorf("lint: unknown tool %q", tool)
	}
	var fs []Finding
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		l := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}
		f, skip, err := p(l)
		if err != nil {
			return nil, err
		}
		if !skip {
			fs = append(fs, *f)
		}
	}
	return fs, sc.Err()
}
