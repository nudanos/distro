package lint

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

// suppression markers per language. lineStart markers count only at the
// start of a line (after indentation); the others anywhere outside a string.
var suppressionMarkers = map[string]struct{ anywhere, lineStart []string }{
	"go":     {anywhere: []string{"//lint:ignore", "//lint:file-ignore", "//nolint"}},
	"perl":   {anywhere: []string{"## no critic"}, lineStart: []string{"#<<<"}},
	"shell":  {anywhere: []string{"# shellcheck disable="}},
	"python": {anywhere: []string{"# noqa", "# pylint: disable"}},
	"c": {anywhere: []string{"// cppcheck-suppress", "/* cppcheck-suppress"},
		lineStart: []string{"#pragma GCC diagnostic ignored", "#pragma clang diagnostic ignored"}},
}

var reasonRe = regexp.MustCompile(`reason:\s*\S`)

// inString reports whether position i of line lies inside a "..." or '...'
// literal, by counting unescaped quotes before it.
func inString(line string, i int) bool {
	dq, sq := 0, 0
	for j := 0; j < i; j++ {
		switch line[j] {
		case '\\':
			j++
		case '"':
			dq++
		case '\'':
			sq++
		}
	}
	return dq%2 == 1 || sq%2 == 1
}

// Unexplained returns a finding for each suppression in src (a file of
// language lang, at path) whose line does not carry "reason:" and some text.
func Unexplained(lang, path string, src []byte) []Finding {
	m, ok := suppressionMarkers[lang]
	if !ok {
		return nil
	}
	var out []Finding
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		at := -1
		for _, mk := range m.lineStart {
			if strings.HasPrefix(strings.TrimLeft(line, " \t"), mk) {
				at = strings.Index(line, mk)
			}
		}
		for _, mk := range m.anywhere {
			if i := strings.Index(line, mk); i >= 0 && !inString(line, i) && (at < 0 || i < at) {
				at = i
			}
		}
		if at >= 0 && !reasonRe.MatchString(line[at:]) {
			out = append(out, Finding{Tool: "lint", File: path, Line: n, Rule: "suppression-without-reason",
				Message: "a suppression needs \"reason: <why>\" on the same line"})
		}
	}
	return out
}
