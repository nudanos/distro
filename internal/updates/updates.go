// Package updates reports manifest entries whose upstream has moved on.
package updates

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/nudanos/distro/internal/control"
	"github.com/nudanos/distro/internal/manifest"
	"github.com/nudanos/distro/internal/versions"
)

// Drift is one pinned version that is no longer the newest. Held is the
// manifest's reason when the newer release is deliberately not taken.
type Drift struct {
	Name, Current, Latest, Detail, Held string
}

// SplitHeld separates drift that needs attention from deliberate holds.
func SplitHeld(drift []Drift) (open, held []Drift) {
	for _, d := range drift {
		if d.Held != "" {
			held = append(held, d)
		} else {
			open = append(open, d)
		}
	}
	return open, held
}

// CheckUpstream compares e.Version with the newest tag matching e.TagPattern.
func CheckUpstream(e manifest.Entry, tags []string) (*Drift, error) {
	re, err := regexp.Compile(e.TagPattern)
	if err != nil {
		return nil, fmt.Errorf("%s: tag_pattern: %w", e.Name, err)
	}
	tag, v, ok := versions.Latest(tags, re)
	if !ok {
		return nil, fmt.Errorf("%s: no tag matches %s", e.Name, e.TagPattern)
	}
	if versions.Compare(v, e.Version) <= 0 {
		return nil, nil
	}
	return &Drift{Name: e.Name, Current: e.Version, Latest: v, Detail: "tag " + tag, Held: e.Hold}, nil
}

// CheckApt compares each pinned package with the version the repository
// currently serves (third-party repos carry only their current release).
func CheckApt(ctx context.Context, e manifest.Entry, client *http.Client) ([]Drift, error) {
	f := strings.Fields(e.Source)
	if len(f) != 3 {
		return nil, fmt.Errorf("%s: source must be URL SUITE COMPONENT", e.Name)
	}
	url := fmt.Sprintf("%s/dists/%s/%s/binary-amd64/Packages.gz", strings.TrimSuffix(f[0], "/"), f[1], f[2])
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: GET %s: %s", e.Name, url, resp.Status)
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	served := map[string]string{}
	for _, p := range control.Parse(string(body)) {
		served[p["Package"]] = p["Version"]
	}
	var out []Drift
	names := make([]string, 0, len(e.Packages))
	for n := range e.Packages {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if v, ok := served[n]; ok && v != e.Packages[n] {
			out = append(out, Drift{Name: e.Name + "/" + n, Current: e.Packages[n], Latest: v, Detail: "apt " + f[0]})
		}
	}
	return out, nil
}

// CheckPackaging reports a packaging pin whose followed branch (head is the
// branch's current commit) has moved past it.
func CheckPackaging(e manifest.Entry, head string) *Drift {
	if e.PackagingBranch == "" || head == "" || head == e.PackagingRef {
		return nil
	}
	short := func(c string) string {
		if len(c) > 7 {
			return c[:7]
		}
		return c
	}
	return &Drift{Name: e.Name, Current: short(e.PackagingRef), Latest: short(head),
		Detail: "packaging " + e.PackagingBranch + " moved past the pin"}
}
