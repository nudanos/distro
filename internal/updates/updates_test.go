package updates

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
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
