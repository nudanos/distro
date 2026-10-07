package scenario

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nudanos/distro/internal/boottest"
	"github.com/nudanos/distro/internal/tacacs"
	"github.com/nudanos/distro/internal/topology"
)

// Replaced in tests.
var (
	startVM    = topology.Start
	ensureBase = EnsureBase
	overlay    = topology.Overlay
	runOne     = Run
	tacacsAddr = "127.0.0.1:49" // the routers reach it as 10.0.2.2:49
)

// Options says which scenario to run, on which image, and where.
type Options struct {
	Scenario  string
	ISO       string
	Reference bool // run on the DANOS 2105 ISO
	Capture   bool // write the 2105 references instead of comparing with them
	KVM       bool
	Work      string // scratch: overlays, transcripts, diffs
	Tests     string // the distro tests/ directory
	refImage  *Image // tests: a 2105 profile without the real ISO
}

// Result is one scenario's outcome.
type Result struct {
	Name        string
	Gating      bool
	Passed      bool
	Failed      []string
	Transcripts string
	Duration    time.Duration
}

// Run boots the scenario's routers, configures them, runs its checks in
// order (a failed check is recorded and the next still runs), captures or
// compares its show output, and always stops every router.
func Run(ctx context.Context, o Options) (res Result, err error) {
	begin := time.Now()
	defer func() { res.Duration = time.Since(begin) }()
	f, err := Load(filepath.Join(o.Tests, "scenarios", o.Scenario, "scenario.yaml"))
	if err != nil {
		return Result{Name: o.Scenario, Gating: true}, err
	}
	res = Result{Name: f.Name, Gating: f.Gating}
	scale := 1.0
	if !o.KVM {
		scale = 6 // emulation is 5-20x slower; timeouts scale with it
	}
	t := func(d time.Duration) time.Duration { return time.Duration(float64(d) * scale) }
	img := NuDanOS(o.ISO)
	if o.Reference {
		if o.refImage != nil {
			img = *o.refImage
		} else if img, err = Reference2105(o.ISO); err != nil {
			return res, err
		}
	}
	outDir := filepath.Join(o.Work, img.Name, o.Scenario) // per image: runs may overlap
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return res, err
	}
	res.Transcripts = outDir
	runLog, err := os.Create(filepath.Join(outDir, "run.log"))
	if err != nil {
		return res, err
	}
	defer runLog.Close()
	base, err := ensureBase(img, o.Work, o.KVM, runLog, t)
	if err != nil {
		return res, err
	}
	specs, err := topology.Plan(f.RouterOrder, f.Links, img.MemMB, o.KVM, topology.AllocatePorts)
	if err != nil {
		return res, err
	}
	if err := f.Expand(specs); err != nil {
		return res, err
	}
	vms := map[string]*topology.VM{}
	ports := map[string]topology.Ports{}
	defer func() {
		for _, vm := range vms {
			vm.Stop(0)
		}
	}()
	for _, s := range specs {
		if img.Live {
			s.ISO = img.ISO
		} else {
			s.Disk = filepath.Join(outDir, s.Name+".qcow2")
			os.Remove(s.Disk)
			if err := overlay(base, s.Disk); err != nil {
				return res, err
			}
		}
		lf, err := os.Create(filepath.Join(outDir, s.Name+".log"))
		if err != nil {
			return res, err
		}
		defer lf.Close()
		vm, err := startVM(s, lf)
		if err != nil {
			return res, fmt.Errorf("starting %s: %w", s.Name, err)
		}
		vms[s.Name], ports[s.Name] = vm, s.Mgmt
	}
	for _, name := range f.RouterOrder {
		c := vms[name].Console
		if err := boottest.Login(c, img.User, img.Password, boottest.LoginPrompt, t(30*time.Minute)); err != nil {
			return res, fmt.Errorf("%s: %w", name, err)
		}
		// no pager, no line wrapping: show output is read whole
		if _, err := boottest.OpOutput(c, "export VYATTA_PAGER=cat; stty cols 250", t(time.Minute)); err != nil {
			return res, fmt.Errorf("%s: %w", name, err)
		}
		if err := waitInterfaces(c, interfaces(specs, name), t(10*time.Minute)); err != nil {
			return res, fmt.Errorf("%s: %w", name, err)
		}
		if err := ConfigureSession(c, append(BaseConfig(name), f.Routers[name].Config...), true, t); err != nil {
			return res, fmt.Errorf("%s: configuring: %w", name, err)
		}
	}
	if f.TACACS != nil {
		l, err := net.Listen("tcp", tacacsAddr)
		if err != nil {
			return res, fmt.Errorf("TACACS+ server: %w", err)
		}
		defer l.Close()
		go tacacs.ServeLog(l, f.TACACS.Secret, f.TACACS.Users, func(format string, a ...any) { fmt.Fprintf(runLog, format+"\n", a...) })
	}
	routers := &Routers{VMs: vms, Ports: ports, Admin: img.User, Password: img.Password, T: t, Log: runLog}
	for _, c := range f.Checks {
		fmt.Fprintf(runLog, "check %q\n", c.Name)
		if err := RunCheck(ctx, c, routers); err != nil {
			res.Failed = append(res.Failed, c.Name+": "+err.Error())
		}
	}
	if err := showStep(f, o, vms, outDir, t, &res); err != nil {
		return res, err
	}
	for _, name := range f.RouterOrder {
		if boottest.Halt(vms[name].Console, t) == nil {
			vms[name].Stop(t(2 * time.Minute))
		}
	}
	res.Passed = len(res.Failed) == 0
	return res, nil
}

func showStep(f *File, o Options, vms map[string]*topology.VM, outDir string, t func(time.Duration) time.Duration, res *Result) error {
	refDir := filepath.Join(o.Tests, "reference", "2105", o.Scenario)
	var diffs strings.Builder
	for _, name := range f.RouterOrder {
		c := vms[name].Console
		dir := filepath.Join(refDir, name)
		if o.Capture {
			if err := os.MkdirAll(filepath.Join(dir, "show"), 0o755); err != nil {
				return err
			}
			// The live 2105 ISO saves its configuration outside the login
			// shell's view (/config does not exist there), so config.boot is
			// the tree show configuration prints, without the version footer.
			for file, cmd := range map[string]string{"config.boot": "show configuration", "commands.txt": "show configuration commands"} {
				out, err := boottest.OpOutput(c, cmd, t(time.Minute))
				if err != nil {
					return fmt.Errorf("%s: %s: %w", name, cmd, err)
				}
				if err := os.WriteFile(filepath.Join(dir, file), []byte(out), 0o644); err != nil {
					return err
				}
			}
		}
		for _, cmd := range f.Show {
			out, err := boottest.OpOutput(c, cmd, t(2*time.Minute))
			if err != nil {
				return fmt.Errorf("%s: %s: %w", name, cmd, err)
			}
			ref := filepath.Join(dir, "show", slug(cmd)+".txt")
			if o.Capture {
				if err := os.WriteFile(ref, []byte(out), 0o644); err != nil {
					return err
				}
				continue
			}
			want, err := os.ReadFile(ref)
			if err != nil {
				res.Failed = append(res.Failed, fmt.Sprintf("%s %q: no 2105 reference (capture with -reference-iso -capture)", name, cmd))
				continue
			}
			if d := Diff(Normalize(cmd, string(want)), Normalize(cmd, out)); d != "" {
				fmt.Fprintf(&diffs, "### %s %s\n%s", name, cmd, d)
			}
		}
	}
	if o.Capture {
		return nil
	}
	os.WriteFile(filepath.Join(outDir, "show.diff"), []byte(diffs.String()), 0o644)
	accepted, _ := os.ReadFile(filepath.Join(refDir, "accepted.diff"))
	if err := CheckAccepted(diffs.String(), string(accepted)); err != nil {
		res.Failed = append(res.Failed, err.Error())
	}
	return nil
}

// interfaces lists a router's NICs as the router names them.
func interfaces(specs []topology.VMSpec, router string) []string {
	names := []string{fmt.Sprintf("dp0s%d", topology.MgmtPCI)}
	for _, s := range specs {
		if s.Name == router {
			for _, d := range s.Data {
				names = append(names, fmt.Sprintf("dp0s%d", d.PCI))
			}
		}
	}
	return names
}

// waitInterfaces waits until show interfaces lists every name: a router
// accepts logins before its NICs are all registered (2105's dataplane, the
// udev renames on NuDanOS), and a commit before then fails on them.
func waitInterfaces(c *boottest.Console, names []string, timeout time.Duration) error {
	return retry(context.Background(), timeout, func() error {
		out, err := boottest.OpOutput(c, "show interfaces", time.Minute)
		if err != nil {
			return err
		}
		for _, n := range names {
			if !regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(n) + `\b`).MatchString(out) {
				return fmt.Errorf("interface %s not listed yet:\n%s", n, out)
			}
		}
		return nil
	})
}

// slug turns a command into a file name: "show ip route" -> "show-ip-route".
func slug(cmd string) string {
	var b strings.Builder
	dash := false
	for _, r := range cmd {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// RunAll runs every named scenario, one after another (a failure never stops
// the next), prints one line per scenario and returns whether every gating
// scenario passed.
func RunAll(ctx context.Context, o Options, names []string, out io.Writer) bool {
	ok := true
	for _, name := range names {
		run := o
		run.Scenario = name
		res, err := runOne(ctx, run)
		if res.Name == "" {
			res.Name, res.Gating = name, true
		}
		passed := err == nil && res.Passed
		kind := "reported"
		if res.Gating {
			kind = "gating"
		}
		verdict := "PASS"
		if !passed {
			verdict = "FAIL"
			if res.Gating {
				ok = false
			}
		}
		fmt.Fprintf(out, "%s %s (%s) %.1f min\n", verdict, res.Name, kind, res.Duration.Minutes())
		if err != nil {
			fmt.Fprintf(out, "    error: %v\n", err)
		}
		for _, f := range res.Failed {
			fmt.Fprintf(out, "    failed: %s\n", strings.ReplaceAll(f, "\n", "\n        "))
		}
	}
	return ok
}
