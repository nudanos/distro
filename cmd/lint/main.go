// Command lint runs the analyzers over one port inside the nudanos/lint
// image and prints its findings as JSON. distro-build lint runs it; see
// docs/superpowers/specs/2026-10-14-code-quality-design.md.
//
//	lint -port <name> <dir>
//
// Exit status 2 means a tool could not check a file: the port is not clean,
// it is unchecked.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/nudanos/distro/internal/lint"
)

// The analyzers for each language, in the order they run.
var toolsFor = map[string][]string{
	"go":     {"gofmt"},
	"perl":   {"perltidy", "perlcritic"},
	"shell":  {"shellcheck"},
	"python": {"flake8", "pylint"},
	"c":      {"cppcheck"},
}

// toolArgs is the command line for tool over files, in the output format
// lint.Parse reads. Profiles live in /lint in the image.
func toolArgs(tool string, files []string) []string {
	var a []string
	switch tool {
	case "gofmt":
		a = []string{"gofmt", "-l"}
	case "perltidy":
		a = []string{"perltidy", "--profile=/lint/perltidyrc", "-st", "-se"}
	case "perlcritic":
		a = []string{"perlcritic", "--profile", "/lint/perlcriticrc", "--verbose", `%f:%l:%c:%p:%m\n`}
	case "shellcheck":
		a = []string{"shellcheck", "-f", "gcc", "-S", "style"}
	case "flake8":
		a = []string{"flake8"}
	case "pylint":
		a = []string{"pylint", "--rcfile", "/lint/pylintrc", "--output-format=parseable"}
	case "cppcheck":
		a = []string{"cppcheck", "-q", "--enable=warning,style,performance,portability",
			"--suppress=missingIncludeSystem", "--inline-suppr", "--template={file}:{line}:{id}:{message}"}
	}
	return append(a, files...)
}

// exitOK reports whether code is tool's status for "ran, maybe with
// findings", as opposed to "could not run".
func exitOK(tool string, code int) bool {
	switch tool {
	case "perlcritic":
		return code == 0 || code == 2
	case "shellcheck", "flake8":
		return code == 0 || code == 1
	case "pylint": // bit 1 fatal, bit 32 usage error; the rest are message classes
		return code&(1|32) == 0
	}
	return code == 0 // gofmt -l, perltidy, cppcheck (no --error-exitcode)
}

func chunks(files []string, n int) [][]string {
	var out [][]string
	for len(files) > n {
		out = append(out, files[:n])
		files = files[n:]
	}
	if len(files) > 0 {
		out = append(out, files)
	}
	return out
}

// run executes argv in dir and returns its stdout and stderr; an exit status
// tool does not use for findings is an error.
func run(dir, tool string, argv []string) (stdout, stderr []byte, err error) {
	var o, e bytes.Buffer
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, &o, &e
	err = cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) && exitOK(tool, ee.ExitCode()) {
		err = nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %v: %s", tool, err, bytes.TrimSpace(e.Bytes()))
	}
	return o.Bytes(), e.Bytes(), nil
}

// untidy lists the Perl files perltidy would change, one per line.
func untidy(dir string, files []string) ([]byte, error) {
	var list bytes.Buffer
	for _, f := range files {
		tidied, _, err := run(dir, "perltidy", toolArgs("perltidy", []string{f}))
		if err != nil {
			return nil, err
		}
		orig, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(orig, tidied) {
			fmt.Fprintln(&list, f)
		}
	}
	return list.Bytes(), nil
}

func check(port, dir string) ([]lint.Finding, error) {
	cfg, err := lint.LoadConfig(dir)
	if err != nil {
		return nil, err
	}
	byLang, err := lint.Classify(dir, cfg)
	if err != nil {
		return nil, err
	}
	out := []lint.Finding{}
	langs := make([]string, 0, len(byLang))
	for l := range byLang {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	for _, lang := range langs {
		files := byLang[lang]
		for _, f := range files {
			src, err := os.ReadFile(filepath.Join(dir, f))
			if err != nil {
				return nil, err
			}
			out = append(out, lint.Unexplained(lang, f, src)...)
		}
		for _, tool := range toolsFor[lang] {
			for _, part := range chunks(files, 200) {
				var raw []byte
				if tool == "perltidy" {
					raw, err = untidy(dir, part)
				} else {
					var stderr []byte
					raw, stderr, err = run(dir, tool, toolArgs(tool, part))
					if tool == "cppcheck" { // cppcheck reports on stderr
						raw = stderr
					}
				}
				if err != nil {
					return nil, err
				}
				fs, err := lint.Parse(tool, raw)
				if err != nil {
					return nil, err
				}
				out = append(out, fs...)
			}
		}
	}
	for i := range out {
		out[i].Port = port
	}
	return out, nil
}

func main() {
	port := flag.String("port", "", "port name recorded in each finding")
	flag.Parse()
	if *port == "" || flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: lint -port <name> <dir>")
		os.Exit(2)
	}
	fs, err := check(*port, flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "lint %s: %v\n", *port, err)
		os.Exit(2)
	}
	if err := json.NewEncoder(os.Stdout).Encode(fs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
