package lint

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
)

// BaselineFile is the ratchet's file, at the distro root.
const BaselineFile = "lint-baseline.json"

// Baseline maps a port to the findings it may still have. A port that is
// not in it may have none.
type Baseline map[string]int

// LoadBaseline reads path. ok is false when the file does not exist: then
// the returned (empty) Baseline allows no finding anywhere.
func LoadBaseline(path string) (b Baseline, ok bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Baseline{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, false, fmt.Errorf("%s: %w", path, err)
	}
	return b, true, nil
}

// Check returns one message per port whose count exceeds its allowance.
func (b Baseline) Check(counts map[string]int) []string {
	var out []string
	for _, port := range sortedKeys(counts) {
		if n := counts[port]; n > b[port] {
			out = append(out, fmt.Sprintf("%s: %d findings, baseline allows %d", port, n, b[port]))
		}
	}
	return out
}

// Update returns a copy with each of ports lowered to its count in counts
// (removed at zero). It refuses to raise any port.
func (b Baseline) Update(counts map[string]int, ports []string) (Baseline, error) {
	out := Baseline{}
	for k, v := range b {
		out[k] = v
	}
	for _, port := range ports {
		n := counts[port]
		if n > b[port] {
			return nil, fmt.Errorf("%s: %d findings, more than its baseline %d; the baseline only goes down", port, n, b[port])
		}
		if n == 0 {
			delete(out, port)
		} else {
			out[port] = n
		}
	}
	return out, nil
}

// NewBaseline is the first baseline: today's counts, ports at zero left out.
func NewBaseline(counts map[string]int) Baseline {
	out := Baseline{}
	for k, n := range counts {
		if n > 0 {
			out[k] = n
		}
	}
	return out
}

// Save writes b as JSON, one port per line, sorted.
func (b Baseline) Save(path string) error {
	data, err := json.MarshalIndent(b, "", "  ") // map keys are sorted
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
