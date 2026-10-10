package scenario

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nudanos/distro/internal/boottest"
	"github.com/nudanos/distro/internal/fixtures"
	"github.com/nudanos/distro/internal/topology"
)

// single boots one router, R1, with data NICs dp0s3-dp0s5: links to three
// peers that never start (R1 listens on each socket, so its NICs exist with
// no carrier). The sampler and the fixtures need interfaces to configure,
// not neighbours.
type single struct {
	vm   *topology.VM
	log  io.Writer
	t    func(time.Duration) time.Duration
	stop func()
}

func bootSingle(o Options, img Image, outDir string) (*single, error) {
	scale := 1.0
	if !o.KVM {
		scale = 6
	}
	t := func(d time.Duration) time.Duration { return time.Duration(float64(d) * scale) }
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	runLog, err := os.Create(filepath.Join(outDir, "run.log"))
	if err != nil {
		return nil, err
	}
	base, err := ensureBase(img, o.Work, o.KVM, runLog, t)
	if err != nil {
		runLog.Close()
		return nil, err
	}
	routers := []string{"R1", "P1", "P2", "P3"}
	links := []topology.Link{{A: "R1", B: "P1"}, {A: "R1", B: "P2"}, {A: "R1", B: "P3"}}
	specs, err := topology.Plan(routers, links, img.MemMB, o.KVM, topology.AllocatePorts)
	if err != nil {
		runLog.Close()
		return nil, err
	}
	s := specs[0]
	if img.Live {
		s.ISO = img.ISO
	} else {
		s.Disk = filepath.Join(outDir, "R1.qcow2")
		os.Remove(s.Disk)
		if err := overlay(base, s.Disk); err != nil {
			runLog.Close()
			return nil, err
		}
	}
	lf, err := os.Create(filepath.Join(outDir, "R1.log"))
	if err != nil {
		runLog.Close()
		return nil, err
	}
	vm, err := startVM(s, lf)
	if err != nil {
		lf.Close()
		runLog.Close()
		return nil, fmt.Errorf("starting R1: %w", err)
	}
	r := &single{vm: vm, log: runLog, t: t, stop: func() {
		if boottest.Halt(vm.Console, t) == nil {
			vm.Stop(t(2 * time.Minute))
		} else {
			vm.Stop(0)
		}
		lf.Close()
		runLog.Close()
	}}
	c := vm.Console
	steps := []func() error{
		func() error {
			return boottest.Login(c, img.User, img.Password, boottest.LoginPrompt, t(30*time.Minute))
		},
		func() error { return boottest.WaitBooted(c, img.User, img.Password, t(20*time.Minute)) },
		func() error {
			_, err := boottest.OpOutput(c, "export VYATTA_PAGER=cat; stty cols 250", t(time.Minute))
			return err
		},
		func() error { return waitInterfaces(c, interfaces(specs, "R1"), t(10*time.Minute)) },
		func() error { return ConfigureSession(c, BaseConfig("R1"), true, t) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			vm.Stop(0)
			lf.Close()
			runLog.Close()
			return nil, fmt.Errorf("R1: %w", err)
		}
	}
	return r, nil
}

// sampler is the capture-only pseudo-scenario: each
// tests/reference/2105/sampler/<feature>.set is committed on one 2105
// router, captured as sampler/<feature>/{config.boot,commands.txt}, and the
// router returns to its base configuration before the next.
func runSampler(ctx context.Context, o Options) (res Result, err error) {
	begin := time.Now()
	res = Result{Name: "sampler", Gating: false}
	defer func() { res.Duration = time.Since(begin) }()
	if !o.Capture || !o.Reference {
		return res, fmt.Errorf("sampler only captures 2105 references (-reference-iso -capture); 'test fixtures' loads them on NuDanOS")
	}
	img, err := Reference2105(o.ISO)
	if o.refImage != nil {
		img, err = *o.refImage, nil
	}
	if err != nil {
		return res, err
	}
	dir := filepath.Join(o.Tests, "reference", "2105", "sampler")
	inputs, _ := filepath.Glob(filepath.Join(dir, "*.set"))
	sort.Strings(inputs)
	if len(inputs) == 0 {
		return res, fmt.Errorf("no sampler inputs in %s", dir)
	}
	outDir := filepath.Join(o.Work, img.Name, "sampler")
	res.Transcripts = outDir
	r, err := bootSingle(o, img, outDir)
	if err != nil {
		return res, err
	}
	defer r.stop()
	c, t := r.vm.Console, r.t
	// "save"/"load" run in configd, which sees /config even where the
	// admin's sandboxed shell does not
	if err := ConfigureSession(c, []string{"save sampler-base.boot"}, false, t); err != nil {
		return res, fmt.Errorf("saving the base configuration: %w", err)
	}
	for _, in := range inputs {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		feature := strings.TrimSuffix(filepath.Base(in), ".set")
		if _, err := os.Stat(filepath.Join(dir, feature, "config.boot")); err == nil {
			fmt.Fprintf(r.log, "feature %s: already captured (delete its directory to recapture)\n", feature)
			continue
		}
		fmt.Fprintf(r.log, "feature %s\n", feature)
		lines, err := setLines(in)
		if err != nil {
			return res, err
		}
		if err := ConfigureSession(c, lines, true, t); err != nil {
			res.Failed = append(res.Failed, feature+": "+err.Error())
		} else if err := captureFeature(c, filepath.Join(dir, feature), t); err != nil {
			res.Failed = append(res.Failed, feature+": capture: "+err.Error())
		}
		if err := ConfigureSession(c, []string{"load sampler-base.boot"}, true, t); err != nil {
			return res, fmt.Errorf("%s: back to the base configuration: %w", feature, err)
		}
	}
	res.Passed = len(res.Failed) == 0
	return res, nil
}

func setLines(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

func captureFeature(c *boottest.Console, dir string, t func(time.Duration) time.Duration) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for file, cmd := range map[string]string{"config.boot": "show configuration", "commands.txt": "show configuration commands"} {
		out, err := boottest.OpOutput(c, cmd, t(time.Minute))
		if err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte(out), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Refs lists every captured 2105 configuration under tests/reference/2105:
// each scenario router's and each sampler feature's.
func Refs(tests string) ([]fixtures.Ref, error) {
	boots, _ := filepath.Glob(filepath.Join(tests, "reference", "2105", "*", "*", "config.boot"))
	sort.Strings(boots)
	var refs []fixtures.Ref
	for _, b := range boots {
		dir := filepath.Dir(b)
		cb, err := os.ReadFile(b)
		if err != nil {
			return nil, err
		}
		cmds, err := os.ReadFile(filepath.Join(dir, "commands.txt"))
		if err != nil {
			return nil, err
		}
		name := filepath.Base(filepath.Dir(dir)) + "/" + filepath.Base(dir)
		refs = append(refs, fixtures.Ref{Name: name, ConfigBoot: string(cb), Commands: string(cmds)})
	}
	return refs, nil
}

// RunFixtures loads every captured 2105 configuration on one NuDanOS router
// and then boots the cpu-affinity sampler's (layer 4, plan 4). It prints
// "fixtures: OK (<n> configs)" or the failures, and returns whether all
// passed.
func RunFixtures(ctx context.Context, o Options, out io.Writer) bool {
	refs, err := Refs(o.Tests)
	if err != nil || len(refs) == 0 {
		fmt.Fprintf(out, "fixtures: no references (%v)\n", err)
		return false
	}
	var reboot *fixtures.Ref
	for i := range refs {
		if refs[i].Name == "sampler/cpu-affinity" {
			reboot = &refs[i]
		}
	}
	if reboot == nil {
		fmt.Fprintln(out, "fixtures: no sampler/cpu-affinity capture for the reboot test")
		return false
	}
	img := NuDanOS(o.ISO)
	r, err := bootSingle(o, img, filepath.Join(o.Work, img.Name, "fixtures"))
	if err != nil {
		fmt.Fprintf(out, "fixtures: %v\n", err)
		return false
	}
	defer r.stop()
	failures := fixtures.Check(ctx, r.vm, refs, r.t)
	if err := fixtures.RebootTest(ctx, r.vm, *reboot, img.User, img.Password, r.t); err != nil {
		failures = append(failures, fixtures.Failure{Name: "reboot with sampler/cpu-affinity", Detail: err.Error()})
	}
	if len(failures) == 0 {
		fmt.Fprintf(out, "fixtures: OK (%d configs)\n", len(refs))
		return true
	}
	for _, f := range failures {
		fmt.Fprintf(out, "FAIL %s\n    %s\n", f.Name, strings.ReplaceAll(strings.TrimSpace(f.Detail), "\n", "\n    "))
	}
	fmt.Fprintf(out, "fixtures: %d of %d failed\n", len(failures), len(refs)+1)
	return false
}
