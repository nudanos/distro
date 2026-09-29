// Package aptmirror copies pinned packages from third-party apt repositories
// (such as FRR's) into the build output, so releases are reproducible.
package aptmirror

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/nudanos/distro/internal/build"
	"github.com/nudanos/distro/internal/engine"
	"github.com/nudanos/distro/internal/manifest"
)

func pins(e manifest.Entry) []string {
	var p []string
	for pkg, ver := range e.Packages {
		p = append(p, pkg+"="+ver)
	}
	sort.Strings(p)
	return p
}

// Key identifies an entry's content; it changes when source, key or pins change.
func Key(e manifest.Entry) string {
	h := sha256.Sum256([]byte(e.Source + "\x00" + e.Key + "\x00" + strings.Join(pins(e), " ")))
	return hex.EncodeToString(h[:])
}

// Spec is the container run that downloads e's pinned binaries and sources into outDir.
func Spec(image string, e manifest.Entry, outDir string, uid, gid int) engine.RunSpec {
	return engine.RunSpec{Image: image,
		Mounts: []engine.Mount{{Host: outDir, Container: "/out"}},
		Env: map[string]string{"SOURCE": e.Source, "KEY_URL": e.Key, "PINS": strings.Join(pins(e), " "),
			"HOST_UID": strconv.Itoa(uid), "HOST_GID": strconv.Itoa(gid)},
		Cmd: []string{"/usr/local/bin/mirror-apt"}}
}

func readState(path string) (map[string]string, error) {
	st := map[string]string{}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	return st, json.Unmarshal(b, &st)
}

// Mirror downloads e into outRoot/<name> unless the stored key matches and the
// .debs are present. It verifies every pinned package arrived.
func Mirror(ctx context.Context, eng engine.Engine, image string, e manifest.Entry,
	outRoot, stateFile string, log io.Writer) (bool, error) {
	st, err := readState(stateFile)
	if err != nil {
		return false, err
	}
	out := filepath.Join(outRoot, e.Name)
	key := Key(e)
	if st[e.Name] == key && build.HasArtifacts(out) {
		return true, nil
	}
	if err := build.EmptyDir(out); err != nil {
		return false, err
	}
	uid, gid := eng.OwnerIDs(os.Getuid(), os.Getgid())
	if err := eng.Run(ctx, Spec(image, e, out, uid, gid), log, log); err != nil {
		return false, fmt.Errorf("mirror %s: %w", e.Name, err)
	}
	for pkg := range e.Packages {
		if m, _ := filepath.Glob(filepath.Join(out, pkg+"_*.deb")); len(m) == 0 {
			return false, fmt.Errorf("mirror %s: %s was not downloaded", e.Name, pkg)
		}
	}
	st[e.Name] = key
	b, _ := json.MarshalIndent(st, "", " ")
	return false, os.WriteFile(stateFile, b, 0o644)
}
