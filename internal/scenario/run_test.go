package scenario

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nudanos/distro/internal/boottest"
	"github.com/nudanos/distro/internal/topology"
)

// fakeRouter answers a console like a NuDanOS router: login, configuration
// mode, commit, show commands (from shows) and poweroff.
func fakeRouter(conn net.Conn, name string, shows map[string]string) {
	host := strings.ToLower(name)
	op, cfg := "\r\nvyatta@"+host+":~$ ", "\r\n[edit]\r\nvyatta@"+host+"# "
	io.WriteString(conn, "\r\n"+host+" login: ")
	state := "user"
	buf := make([]byte, 0, 256)
	b := make([]byte, 256)
	for {
		n, err := conn.Read(b)
		if err != nil {
			return
		}
		buf = append(buf, b[:n]...)
		for {
			i := bytes.IndexByte(buf, '\r')
			if i < 0 {
				break
			}
			line := string(buf[:i])
			buf = buf[i+1:]
			reply := ""
			switch {
			case state == "user":
				reply, state = line+"\r\nPassword: ", "pass"
			case state == "pass":
				reply, state = op, "op"
			case line == "configure":
				reply, state = line+cfg, "cfg"
			case state == "cfg" && (line == "exit" || line == "exit discard"):
				reply, state = line+op, "op"
			case state == "cfg":
				reply = line + cfg
			case line == "poweroff":
				reply = line + "\r\nProceed with poweroff? (Yes/No) [No] "
			case line == "y":
				conn.Close()
				return
			default:
				// output ends with its own newline; the prompt follows directly
				reply = line + "\r\n" + strings.ReplaceAll(shows[line], "\n", "\r\n") + strings.TrimPrefix(op, "\r\n")
			}
			io.WriteString(conn, reply)
		}
	}
}

// fakeStart starts a fakeRouter per VM; a router named in fail does not start.
func fakeStart(shows map[string]string, fail map[string]bool, stopped *sync.Map) func(topology.VMSpec, io.Writer) (*topology.VM, error) {
	return func(s topology.VMSpec, log io.Writer) (*topology.VM, error) {
		if fail[s.Name] {
			return nil, fmt.Errorf("%s: qemu exited before its console answered: exit status 1", s.Name)
		}
		a, b := net.Pipe()
		if _, ok := shows["show interfaces"]; !ok {
			if shows == nil {
				shows = map[string]string{}
			}
			shows["show interfaces"] = "dp0s3  -\ndp0s4  -\ndp0s10  10.0.2.15/24\n"
		}
		go fakeRouter(b, s.Name, shows)
		stopped.Store(s.Name, false)
		return topology.NewVM(s, boottest.NewConsole(a, log), func(time.Duration) { stopped.Store(s.Name, true); a.Close() }), nil
	}
}

func scenarioTree(t *testing.T, name, body string) string {
	t.Helper()
	tests := t.TempDir()
	os.MkdirAll(filepath.Join(tests, "scenarios", name), 0o755)
	os.WriteFile(filepath.Join(tests, "scenarios", name, "scenario.yaml"), []byte(body), 0o644)
	return tests
}

func fakeRun(t *testing.T, shows map[string]string, fail map[string]bool) *sync.Map {
	t.Helper()
	var stopped sync.Map
	oldStart, oldBase, oldOverlay := startVM, ensureBase, overlay
	startVM = fakeStart(shows, fail, &stopped)
	ensureBase = func(Image, string, bool, io.Writer, func(time.Duration) time.Duration) (string, error) {
		return "/work/base.qcow2", nil
	}
	overlay = func(base, path string) error { return nil }
	opInterval = 10 * time.Millisecond
	t.Cleanup(func() { startVM, ensureBase, overlay = oldStart, oldBase, oldOverlay })
	return &stopped
}

const pair = `
name: pair
gating: true
routers:
  R1: {config: ["set interfaces dataplane ${R1:R2} address 10.0.12.1/24"]}
  R2: {config: ["set interfaces dataplane ${R2:R1} address 10.0.12.2/24"]}
links: [[R1, R2]]
show: ["show ip route"]
checks:
  - name: R1 route
    router: R1
    timeout: 1s
    op: {command: "show ip route", want: "10.0.12.0/24"}
`

func TestRunPassesAndCapturesShow(t *testing.T) {
	// The live 2105 ISO saves outside the login shell's view (/config does
	// not exist there), so config.boot is captured from show configuration.
	stopped := fakeRun(t, map[string]string{
		"show ip route":      "C>* 10.0.12.0/24 is directly connected, dp0s3, 00:01:02\n",
		"show configuration": "interfaces {\n    dataplane dp0s3 {\n    }\n}\n",
	}, nil)
	tests := scenarioTree(t, "pair", pair)
	work := t.TempDir()
	res, err := Run(context.Background(), Options{Scenario: "pair", ISO: "/iso/n.iso", Capture: true, Reference: true, refImage: &Image{Name: "2105", MemMB: 1536, User: "tmpuser", Password: "tmppwd", Live: true}, Work: work, Tests: tests})
	if err != nil || !res.Passed {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	boot, err := os.ReadFile(filepath.Join(tests, "reference", "2105", "pair", "R1", "config.boot"))
	if err != nil || !strings.Contains(string(boot), "dataplane dp0s3") {
		t.Errorf("captured config.boot = %q, %v", boot, err)
	}
	got, err := os.ReadFile(filepath.Join(tests, "reference", "2105", "pair", "R1", "show", "show-ip-route.txt"))
	if err != nil || !strings.Contains(string(got), "10.0.12.0/24") {
		t.Errorf("captured show = %q, %v", got, err)
	}
	// 2105 and NuDanOS runs of one scenario keep separate transcripts
	if want := filepath.Join(work, "2105", "pair"); res.Transcripts != want {
		t.Errorf("Transcripts = %q, want %q", res.Transcripts, want)
	}
	stopped.Range(func(k, v any) bool {
		if v != true {
			t.Errorf("%v was not stopped", k)
		}
		return true
	})
}

func TestRunRecordsFailedCheckAndContinues(t *testing.T) {
	fakeRun(t, map[string]string{"show ip route": "nothing\n", "show version": "Version: 1.0\n"}, nil)
	body := pair + `  - name: R2 version
    router: R2
    timeout: 1s
    op: {command: "show version", want: "Version"}
`
	tests := scenarioTree(t, "pair", body)
	os.MkdirAll(filepath.Join(tests, "reference", "2105", "pair", "R1", "show"), 0o755)
	os.MkdirAll(filepath.Join(tests, "reference", "2105", "pair", "R2", "show"), 0o755)
	os.WriteFile(filepath.Join(tests, "reference", "2105", "pair", "R1", "show", "show-ip-route.txt"), []byte("nothing\n"), 0o644)
	os.WriteFile(filepath.Join(tests, "reference", "2105", "pair", "R2", "show", "show-ip-route.txt"), []byte("nothing\n"), 0o644)
	res, err := Run(context.Background(), Options{Scenario: "pair", ISO: "/iso/n.iso", Work: t.TempDir(), Tests: tests})
	if err != nil {
		t.Fatal(err)
	}
	if res.Duration <= 0 {
		t.Error("Result.Duration not set")
	}
	if res.Passed || len(res.Failed) != 1 || !strings.Contains(res.Failed[0], "R1 route") {
		t.Errorf("Result = %+v; want exactly 'R1 route' failed and R2's check attempted", res)
	}
}

// Review Focus 1: a router that never starts fails the scenario by name and
// leaves nothing running.
func TestRunStopsAllVMsOnError(t *testing.T) {
	stopped := fakeRun(t, nil, map[string]bool{"R2": true})
	tests := scenarioTree(t, "pair", pair)
	_, err := Run(context.Background(), Options{Scenario: "pair", ISO: "/iso/n.iso", Work: t.TempDir(), Tests: tests})
	if err == nil || !strings.Contains(err.Error(), "R2") {
		t.Fatalf("err = %v, want it to name R2", err)
	}
	if v, ok := stopped.Load("R1"); !ok || v != true {
		t.Error("R1 was left running")
	}
}

func TestRunnerContinuesAfterFailedScenario(t *testing.T) {
	var ran []string
	old := runOne
	runOne = func(ctx context.Context, o Options) (Result, error) {
		ran = append(ran, o.Scenario)
		if o.Scenario == "a" {
			return Result{Name: "a", Gating: true}, fmt.Errorf("boom")
		}
		return Result{Name: o.Scenario, Gating: true, Passed: true}, nil
	}
	defer func() { runOne = old }()
	var out bytes.Buffer
	ok := RunAll(context.Background(), Options{}, []string{"a", "b"}, &out)
	if ok || strings.Join(ran, ",") != "a,b" {
		t.Errorf("RunAll = %v, ran %v; want false and both run", ok, ran)
	}
	for _, want := range []string{"FAIL a (gating)", "PASS b (gating)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary lacks %q:\n%s", want, out.String())
		}
	}
}

// 2105 accepts logins before its dataplane has registered every port, and a
// commit then warns "device dp0s10 does not exist" (rest run 1). Run waits
// until show interfaces lists the router's interfaces.
func TestWaitInterfacesUntilListed(t *testing.T) {
	opInterval = 10 * time.Millisecond
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	go func() {
		b := make([]byte, 128)
		for _, answer := range []string{"Interface  IP Address\nlo  127.0.0.1/8\n", "Interface  IP Address\ndp0s3  -\ndp0s10  -\n"} {
			n, err := vm.Read(b)
			if err != nil {
				return
			}
			io.WriteString(vm, string(b[:n])+"\n"+strings.ReplaceAll(answer, "\n", "\r\n")+"vyatta@r1:~$ ")
		}
	}()
	c := boottest.NewConsole(a, &bytes.Buffer{})
	if err := waitInterfaces(c, []string{"dp0s3", "dp0s10"}, 5*time.Second); err != nil {
		t.Fatal(err)
	}
}
