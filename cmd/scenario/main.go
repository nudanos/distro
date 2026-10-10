// Command scenario runs layer 4 scenarios (plan 4) inside
// nudanos/tester:trixie: QEMU routers from one ISO, configured and checked
// as each tests/scenarios/<name>/scenario.yaml says.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/nudanos/distro/internal/scenario"
)

type names []string

func (n *names) String() string     { return fmt.Sprint(*n) }
func (n *names) Set(v string) error { *n = append(*n, v); return nil }

func main() {
	var only names
	flag.Var(&only, "scenario", "scenario to run (repeatable)")
	all := flag.Bool("all", false, "run every scenario in -tests/scenarios")
	o := scenario.Options{}
	flag.StringVar(&o.ISO, "iso", "", "ISO to boot")
	flag.BoolVar(&o.Reference, "reference", false, "the ISO is DANOS 2105")
	flag.BoolVar(&o.Capture, "capture", false, "write the 2105 references")
	flag.BoolVar(&o.KVM, "kvm", false, "use KVM (otherwise emulation)")
	flag.StringVar(&o.Work, "work", "/work", "scratch directory")
	flag.StringVar(&o.Tests, "tests", "/tests", "distro's tests/ directory")
	fix := flag.Bool("fixtures", false, "load every captured 2105 configuration on one router, then the reboot test")
	flag.Parse()
	if *fix {
		if !scenario.RunFixtures(context.Background(), o, os.Stdout) {
			os.Exit(1)
		}
		return
	}
	list := []string(only)
	if *all {
		dirs, _ := filepath.Glob(filepath.Join(o.Tests, "scenarios", "*", "scenario.yaml"))
		for _, d := range dirs {
			list = append(list, filepath.Base(filepath.Dir(d)))
		}
		sort.Strings(list)
	}
	if len(list) == 0 {
		fmt.Fprintln(os.Stderr, "scenario: nothing to run (-scenario name or -all)")
		os.Exit(2)
	}
	if !scenario.RunAll(context.Background(), o, list, os.Stdout) {
		os.Exit(1)
	}
}
