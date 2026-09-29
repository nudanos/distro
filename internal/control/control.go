// Package control parses Debian control files (deb822 paragraphs).
package control

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Paragraph maps field names to values. Continuation lines are joined with a space.
// Field names are case-insensitive in deb822, so keys are stored canonically
// (see Canonical): look fields up as "Build-Depends", "Source", "Package".
type Paragraph map[string]string

// Canonical returns a field name with each dash-separated word capitalised and
// the rest lower-cased: "build-depends-INDEP" -> "Build-Depends-Indep".
func Canonical(field string) string {
	words := strings.Split(strings.ToLower(field), "-")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, "-")
}

// Parse splits deb822 text into paragraphs. Lines starting with '#' are ignored.
func Parse(text string) []Paragraph {
	var out []Paragraph
	cur := Paragraph{}
	key := ""
	flush := func() {
		if len(cur) > 0 {
			out = append(out, cur)
		}
		cur, key = Paragraph{}, ""
	}
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.TrimSpace(line) == "":
			flush()
		case strings.HasPrefix(line, "#"):
		case (line[0] == ' ' || line[0] == '\t') && key != "":
			cur[key] += " " + strings.TrimSpace(line)
		default:
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			key = Canonical(strings.TrimSpace(k))
			cur[key] = strings.TrimSpace(v)
		}
	}
	flush()
	return out
}

var (
	reVersion = regexp.MustCompile(`\([^)]*\)`)
	reArch    = regexp.MustCompile(`\[[^\]]*\]`)
	reProfile = regexp.MustCompile(`<[^>]*>`)
)

// Relations parses a relationship field into AND-groups of OR-alternatives,
// keeping package names only. Version constraints, [arch] and <profile>
// restrictions, ":any"-style qualifiers and ${substvars} are dropped.
func Relations(field string) [][]string {
	var groups [][]string
	for _, group := range strings.Split(field, ",") {
		var alts []string
		for _, alt := range strings.Split(group, "|") {
			alt = reVersion.ReplaceAllString(alt, "")
			alt = reArch.ReplaceAllString(alt, "")
			alt = reProfile.ReplaceAllString(alt, "")
			alt = strings.TrimSpace(alt)
			if alt == "" || strings.HasPrefix(alt, "${") {
				continue
			}
			name, _, _ := strings.Cut(alt, ":")
			alts = append(alts, name)
		}
		if len(alts) > 0 {
			groups = append(groups, alts)
		}
	}
	return groups
}

// Source summarises one source package's control file.
type Source struct {
	Name         string
	BuildDepends [][]string // Build-Depends, -Indep and -Arch combined
	Binaries     []string   // sorted
	Provides     []string   // sorted
}

// ParseSource reads the source paragraph and binary paragraphs of a control file.
func ParseSource(text string) (*Source, error) {
	ps := Parse(text)
	if len(ps) == 0 || ps[0]["Source"] == "" {
		return nil, errors.New("no Source paragraph")
	}
	s := &Source{Name: ps[0]["Source"]}
	for _, f := range []string{"Build-Depends", "Build-Depends-Indep", "Build-Depends-Arch"} {
		s.BuildDepends = append(s.BuildDepends, Relations(ps[0][f])...)
	}
	for _, p := range ps[1:] {
		if n := p["Package"]; n != "" {
			s.Binaries = append(s.Binaries, n)
		}
		for _, g := range Relations(p["Provides"]) {
			s.Provides = append(s.Provides, g...)
		}
	}
	sort.Strings(s.Binaries)
	sort.Strings(s.Provides)
	return s, nil
}

// ReadSource parses the control file at path (normally <src>/debian/control).
func ReadSource(path string) (*Source, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s, err := ParseSource(string(b))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}
