// Package versions compares upstream release versions and picks tags.
package versions

import (
	"regexp"
	"strconv"
	"strings"
)

func parts(v string) []string {
	return strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '_' })
}

// Compare orders versions component by component, numerically. A version
// with extra components is newer ("2.0.0" > "2.0"). A non-numeric component
// sorts before any number.
func Compare(a, b string) int {
	pa, pb := parts(a), parts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		if i >= len(pa) {
			return -1
		}
		if i >= len(pb) {
			return 1
		}
		na, ea := strconv.Atoi(pa[i])
		nb, eb := strconv.Atoi(pb[i])
		switch {
		case ea != nil && eb != nil:
			if c := strings.Compare(pa[i], pb[i]); c != 0 {
				return c
			}
		case ea != nil:
			return -1
		case eb != nil:
			return 1
		case na != nb:
			if na < nb {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Latest returns the newest tag matching pattern (group 1 is the version,
// with '_' separators normalised to '.').
func Latest(tags []string, pattern *regexp.Regexp) (tag, version string, ok bool) {
	for _, t := range tags {
		m := pattern.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		v := strings.ReplaceAll(m[1], "_", ".")
		if !ok || Compare(v, version) > 0 {
			tag, version, ok = t, v, true
		}
	}
	return tag, version, ok
}
