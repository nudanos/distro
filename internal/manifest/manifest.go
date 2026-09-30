// Package manifest loads and validates distro/manifest.yaml.
package manifest

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// Kind says where a package comes from.
type Kind string

const (
	Danos     Kind = "danos"     // our repo on its trixie branch, own debian/ packaging
	Upstream  Kind = "upstream"  // latest upstream release built with Debian's packaging
	Apt       Kind = "apt"       // mirrored at pinned versions from a third-party apt repo
	Debian    Kind = "debian"    // Debian 13's own package, not built by us
	Drop      Kind = "drop"      // not carried forward
	Reference Kind = "reference" // kept for history, not a package we build
)

var milestones = map[string]bool{"1.0": true, "1.1": true, "1.5": true, "2": true, "later": true, "none": true}

// AuditEntry records the decision on one DANOS patch to an upstream package.
type AuditEntry struct {
	Patch   string `yaml:"patch"`
	Verdict string `yaml:"verdict"` // kept | upstreamed | dropped
	Note    string `yaml:"note,omitempty"`
}

// Entry is one package or repository in the manifest.
type Entry struct {
	Name         string       `yaml:"name"`
	Kind         Kind         `yaml:"kind"`
	Milestone    string       `yaml:"milestone"`
	Ready        bool         `yaml:"ready,omitempty"`
	Repo         string       `yaml:"repo,omitempty"`
	Ref          string       `yaml:"ref,omitempty"`
	Upstream     string       `yaml:"upstream,omitempty"`
	Track        string       `yaml:"track,omitempty"`
	Packaging    string       `yaml:"packaging,omitempty"`
	Patches      string       `yaml:"patches,omitempty"`
	Version      string       `yaml:"version,omitempty"`       // upstream: upstream version built
	Tag          string       `yaml:"tag,omitempty"`           // upstream: git tag of Version
	TagPattern   string       `yaml:"tag_pattern,omitempty"`   // upstream: regexp, group 1 = version
	PackagingRef string       `yaml:"packaging_ref,omitempty"` // upstream: branch or tag of packaging
	Audit        []AuditEntry `yaml:"audit,omitempty"`
	Exclude      []string     `yaml:"exclude,omitempty"`       // upstream: globs absent from the release tarball
	Subdir       string       `yaml:"subdir,omitempty"`        // upstream: source root inside the upstream repo
	PackagingDir string       `yaml:"packaging_dir,omitempty"` // upstream: debian/ taken from this upstream path instead of packaging
	UnpackWaf    bool         `yaml:"unpack_waf,omitempty"`    // upstream: replace the self-extracting waf blob with its source
	Profiles     []string     `yaml:"profiles,omitempty"`      // Debian build profiles to build with (DEB_BUILD_PROFILES)

	Source   string            `yaml:"source,omitempty"`   // apt: "URL SUITE COMPONENT"
	Key      string            `yaml:"key,omitempty"`      // apt: signing key URL
	Packages map[string]string `yaml:"packages,omitempty"` // apt: binary package -> exact version
	Note     string            `yaml:"note,omitempty"`
}

// Manifest is the whole package list.
type Manifest struct {
	Packages []Entry `yaml:"packages"`
}

// Parse decodes YAML strictly (unknown fields are errors) and validates it.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Load reads and parses a manifest file.
func Load(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// Validate checks every entry and reports all problems at once.
func (m *Manifest) Validate() error {
	var errs []string
	seen := map[string]bool{}
	for i, e := range m.Packages {
		bad := func(msg string) { errs = append(errs, fmt.Sprintf("entry %d (%s): %s", i, e.Name, msg)) }
		if e.Name == "" {
			bad("missing name")
		}
		if seen[e.Name] {
			bad("duplicate name")
		}
		seen[e.Name] = true
		if !milestones[e.Milestone] {
			bad("milestone must be one of 1.0, 1.1, 1.5, 2, later, none")
		}
		switch e.Kind {
		case Danos:
			if e.Repo == "" || e.Ref == "" {
				bad("danos entries need repo and ref")
			}
		case Upstream:
			if e.Upstream == "" || (e.Packaging == "") == (e.PackagingDir == "") {
				bad("upstream entries need upstream and exactly one of packaging or packaging_dir")
			}
			if e.Track != "latest" && e.Track != "lts" && e.Track != "debian" {
				bad("track must be latest, lts or debian")
			}
			if e.TagPattern != "" {
				if re, err := regexp.Compile(e.TagPattern); err != nil {
					bad("tag_pattern: " + err.Error())
				} else if re.NumSubexp() != 1 {
					bad("tag_pattern needs exactly one capture group (the version)")
				}
			}
			if e.Ready && (e.Version == "" || e.Tag == "" || e.TagPattern == "" || (e.Packaging != "" && e.PackagingRef == "")) {
				bad("ready upstream entries need version, tag, tag_pattern and (with packaging) packaging_ref")
			}

		case Apt:
			if len(strings.Fields(e.Source)) != 3 || e.Key == "" || len(e.Packages) == 0 {
				bad(`apt entries need source "URL SUITE COMPONENT", key and packages`)
			}
		case Debian, Drop, Reference:
			if e.Ready {
				bad(string(e.Kind) + " entries cannot be ready")
			}
		default:
			bad(fmt.Sprintf("unknown kind %q", e.Kind))
		}
		for _, a := range e.Audit {
			if a.Verdict != "kept" && a.Verdict != "upstreamed" && a.Verdict != "dropped" {
				bad(fmt.Sprintf("audit %s: verdict must be kept, upstreamed or dropped", a.Patch))
			}
		}

	}
	if len(errs) > 0 {
		return fmt.Errorf("manifest invalid:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// Ready returns the entries of kind k that are marked ready, in manifest order.
func (m *Manifest) Ready(k Kind) []Entry {
	var out []Entry
	for _, e := range m.Packages {
		if e.Kind == k && e.Ready {
			out = append(out, e)
		}
	}
	return out
}
