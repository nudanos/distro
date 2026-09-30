// Package fetch checks out package sources.
package fetch

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func run(ctx context.Context, log io.Writer, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %v: %w", args, err)
	}
	return nil
}

// Git clones repo into dir (or fetches if dir already holds a clone), checks out
// ref detached (a remote branch, else a tag), and removes untracked files.
func Git(ctx context.Context, repo, ref, dir string, log io.Writer) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		if err := run(ctx, log, "", "clone", "--no-checkout", repo, dir); err != nil {
			return err
		}
	} else {
		// The manifest may have moved the entry to another repository.
		if err := run(ctx, log, dir, "remote", "set-url", "origin", repo); err != nil {
			return err
		}
		if err := run(ctx, log, dir, "fetch", "--tags", "--force", "--prune", "origin"); err != nil {
			return err
		}
	}
	target := "origin/" + ref
	if run(ctx, io.Discard, dir, "rev-parse", "--verify", "--quiet", target+"^{commit}") != nil {
		target = "refs/tags/" + ref
		if run(ctx, io.Discard, dir, "rev-parse", "--verify", "--quiet", target+"^{commit}") != nil {
			return fmt.Errorf("%s: no branch or tag %q", repo, ref)
		}
	}
	if err := run(ctx, log, dir, "checkout", "--quiet", "--force", "--detach", target); err != nil {
		return err
	}
	return run(ctx, log, dir, "clean", "-ffdxq")
}
