// Package engine runs containers through the docker or podman CLI.
package engine

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
)

// Mount is a bind mount.
type Mount struct {
	Host, Container string
	ReadOnly        bool
}

// RunSpec describes one container run.
type RunSpec struct {
	Image   string
	Mounts  []Mount
	Env     map[string]string
	Workdir string
	Network string // empty = engine default
	Cmd     []string
}

// Engine is a docker-compatible CLI ("docker" or "podman").
type Engine struct {
	Bin string
}

// OwnerIDs returns the uid:gid that container scripts should chown bind-mounted
// output to so the host user owns it. Rootless podman maps container uid 0 to
// the invoking user, so there the answer is 0:0; docker needs the host ids.
func (e Engine) OwnerIDs(hostUID, hostGID int) (int, int) {
	if e.Bin == "podman" {
		return 0, 0
	}
	return hostUID, hostGID
}

// RunArgs returns the CLI arguments for s, with env vars in sorted order.
func (e Engine) RunArgs(s RunSpec) []string {
	args := []string{"run", "--rm"}
	if s.Network != "" {
		args = append(args, "--network", s.Network)
	}
	for _, m := range s.Mounts {
		v := m.Host + ":" + m.Container
		if m.ReadOnly {
			v += ":ro"
		}
		args = append(args, "-v", v)
	}
	keys := make([]string, 0, len(s.Env))
	for k := range s.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", k+"="+s.Env[k])
	}
	if s.Workdir != "" {
		args = append(args, "-w", s.Workdir)
	}
	args = append(args, s.Image)
	return append(args, s.Cmd...)
}

// Run executes s and waits for it.
func (e Engine) Run(ctx context.Context, s RunSpec, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, e.Bin, e.RunArgs(s)...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s run %s: %w", e.Bin, s.Image, err)
	}
	return nil
}

// BuildImage builds the Dockerfile in dir and tags it.
func (e Engine) BuildImage(ctx context.Context, dir, tag string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, e.Bin, "build", "--pull", "-t", tag, dir)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s build %s: %w", e.Bin, dir, err)
	}
	return nil
}
