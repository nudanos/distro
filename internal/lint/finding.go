// Package lint collects analyzer findings for the NuDanOS ports
// (docs/superpowers/specs/2026-10-14-code-quality-design.md).
package lint

import "fmt"

// Finding is one analyzer complaint about one place in a port.
type Finding struct {
	Port    string `json:"port"`
	Tool    string `json:"tool"`
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

func (f Finding) String() string {
	return fmt.Sprintf("%s: %s:%d: %s %s: %s", f.Port, f.File, f.Line, f.Tool, f.Rule, f.Message)
}
