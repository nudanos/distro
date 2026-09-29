package control

import (
	"reflect"
	"testing"
)

const sample = `# leading comment
Source: vyatta-util
Section: net
Build-Depends: debhelper-compat (= 13),
 liburiparser-dev (>= 0.9) [amd64 arm64],
 check <!nocheck>,
 python3:any | python3-minimal,
 ${misc:Depends}
Build-Depends-Indep: dh-yang

Package: vyatta-util
Architecture: any
Depends: libvyatta-util1 (= ${binary:Version}), ${shlibs:Depends}
Description: utilities
 Multi-line description
 continues here.

Package: libvyatta-util1
Provides: libvyatta-util (= 1.0), vyatta-validate
`

func TestParseJoinsContinuationsAndSkipsComments(t *testing.T) {
	ps := Parse(sample)
	if len(ps) != 3 {
		t.Fatalf("got %d paragraphs, want 3", len(ps))
	}
	if ps[0]["Source"] != "vyatta-util" {
		t.Errorf("Source = %q", ps[0]["Source"])
	}
	if got := ps[1]["Description"]; got != "utilities Multi-line description continues here." {
		t.Errorf("Description = %q", got)
	}
}

func TestRelationsStripsEverythingButNames(t *testing.T) {
	got := Relations("debhelper-compat (= 13), libfoo-dev (>= 1.0) [amd64] <!nocheck>, a | b:any, ${misc:Depends},, check <!nocheck>")
	want := [][]string{{"debhelper-compat"}, {"libfoo-dev"}, {"a", "b"}, {"check"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Relations = %v, want %v", got, want)
	}
}

func TestParseSource(t *testing.T) {
	s, err := ParseSource(sample)
	if err != nil {
		t.Fatal(err)
	}
	wantBD := [][]string{{"debhelper-compat"}, {"liburiparser-dev"}, {"check"}, {"python3", "python3-minimal"}, {"dh-yang"}}
	if !reflect.DeepEqual(s.BuildDepends, wantBD) {
		t.Errorf("BuildDepends = %v, want %v", s.BuildDepends, wantBD)
	}
	if !reflect.DeepEqual(s.Binaries, []string{"libvyatta-util1", "vyatta-util"}) {
		t.Errorf("Binaries = %v", s.Binaries)
	}
	if !reflect.DeepEqual(s.Provides, []string{"libvyatta-util", "vyatta-validate"}) {
		t.Errorf("Provides = %v", s.Provides)
	}
}

func TestParseSourceRejectsMissingSource(t *testing.T) {
	if _, err := ParseSource("Package: x\n"); err == nil {
		t.Error("want error for control file without Source paragraph")
	}
}
