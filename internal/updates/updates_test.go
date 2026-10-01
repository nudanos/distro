package updates

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nudanos/distro/internal/manifest"
)

func TestCheckUpstream(t *testing.T) {
	e := manifest.Entry{Name: "mstpd", Version: "0.1.1", TagPattern: `^(\d+\.\d+\.\d+)$`}
	d, err := CheckUpstream(e, []string{"0.05", "0.1.1", "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if d == nil || d.Latest != "0.2.0" || d.Current != "0.1.1" {
		t.Errorf("drift = %+v", d)
	}
	e.Version = "0.2.0"
	if d, _ := CheckUpstream(e, []string{"0.05", "0.2.0"}); d != nil {
		t.Errorf("up to date but drift = %+v", d)
	}
	if _, err := CheckUpstream(manifest.Entry{Name: "x", Version: "1", TagPattern: `^v(\d+)$`}, []string{"nope"}); err == nil {
		t.Error("no matching tags should be an error")
	}
}

func TestCheckApt(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write([]byte("Package: frr\nVersion: 10.7.2-0~deb13u1\n\nPackage: libyang3\nVersion: 3.13.6-1~deb13u1\n"))
	zw.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/frr/dists/trixie/frr-stable/binary-amd64/Packages.gz" {
			http.NotFound(w, r)
			return
		}
		w.Write(buf.Bytes())
	}))
	defer srv.Close()
	e := manifest.Entry{Name: "frr", Source: srv.URL + "/frr trixie frr-stable",
		Packages: map[string]string{"frr": "10.7.1-0~deb13u1", "libyang3": "3.13.6-1~deb13u1"}}
	ds, err := CheckApt(context.Background(), e, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].Name != "frr/frr" || ds[0].Latest != "10.7.2-0~deb13u1" {
		t.Errorf("drift = %+v", ds)
	}
}

// A deliberate hold (pam_tacplus: 1.6.2+ install a libtac.h that includes
// gnulib headers they do not ship) still reports the newer release, marked
// held, so check-updates can list it without failing.
func TestCheckUpstreamHeld(t *testing.T) {
	e := manifest.Entry{Name: "pam_tacplus", Version: "1.6.1", TagPattern: `^v(\d+\.\d+\.\d+)$`,
		Hold: "libtac.h needs unshipped gnulib headers"}
	d, err := CheckUpstream(e, []string{"v1.6.1", "v1.7.0"})
	if err != nil {
		t.Fatal(err)
	}
	if d == nil || d.Latest != "1.7.0" || d.Held != e.Hold {
		t.Errorf("drift = %+v, want a held drift to 1.7.0", d)
	}
	if open, held := SplitHeld([]Drift{*d, {Name: "x", Current: "1", Latest: "2"}}); len(open) != 1 || len(held) != 1 {
		t.Errorf("SplitHeld = %v, %v", open, held)
	}
}

// Packaging is pinned to a commit; check-updates reports when the branch the
// pin follows has moved on, so the pin does not go stale silently.
func TestCheckPackaging(t *testing.T) {
	pin := "1111111111111111111111111111111111111111"
	e := manifest.Entry{Name: "net-snmp", PackagingRef: pin, PackagingBranch: "master"}
	if d := CheckPackaging(e, pin); d != nil {
		t.Errorf("at the pin: drift %+v", d)
	}
	d := CheckPackaging(e, "2222222222222222222222222222222222222222")
	if d == nil || d.Name != "net-snmp" || d.Current != "1111111" || d.Latest != "2222222" ||
		!strings.Contains(d.Detail, "packaging master") {
		t.Errorf("moved branch: drift = %+v", d)
	}
	if d := CheckPackaging(manifest.Entry{Name: "x", PackagingRef: pin}, "3333333333333333333333333333333333333333"); d != nil {
		t.Errorf("no followed branch: drift %+v", d)
	}
}
