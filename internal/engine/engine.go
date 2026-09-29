// Package engine runs containers through the docker or podman CLI.
package engine

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
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

// BuildArgs returns the CLI arguments to build dir as tag with the given labels
// (in sorted order).
func (e Engine) BuildArgs(dir, tag string, labels map[string]string) []string {
	args := []string{"build", "--pull"}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--label", k+"="+labels[k])
	}
	return append(args, "-t", tag, dir)
}

// LabelArgs returns the CLI arguments that print one label of an image.
func (e Engine) LabelArgs(image, label string) []string {
	return []string{"image", "inspect", "--format", `{{ index .Config.Labels "` + label + `" }}`, image}
}

// ImageLabel returns the value of label on image ("" if the label is unset).
func (e Engine) ImageLabel(ctx context.Context, image, label string) (string, error) {
	out, err := exec.CommandContext(ctx, e.Bin, e.LabelArgs(image, label)...).Output()
	if err != nil {
		return "", fmt.Errorf("%s image inspect %s: %w (run distro-build builder first?)", e.Bin, image, err)
	}
	v := strings.TrimSpace(string(out))
	if v == "<no value>" {
		v = ""
	}
	return v, nil
}

// BuildImage builds the Dockerfile in dir, tags it and applies labels.
func (e Engine) BuildImage(ctx context.Context, dir, tag string, labels map[string]string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, e.Bin, e.BuildArgs(dir, tag, labels)...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s build %s: %w", e.Bin, dir, err)
	}
	return nil
}
