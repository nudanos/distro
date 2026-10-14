package lint

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCheckFailsAboveAllowance(t *testing.T) {
	b := Baseline{"aaa": 3, "config": 1}
	got := b.Check(map[string]int{"aaa": 4, "config": 1})
	if len(got) != 1 || !strings.Contains(got[0], "aaa") || !strings.Contains(got[0], "4") {
		t.Errorf("got %q, want one failure naming aaa and its count", got)
	}
	if got := b.Check(map[string]int{"aaa": 2, "config": 0}); len(got) != 0 {
		t.Errorf("fewer findings than allowed failed: %q", got)
	}
}

// A port added to scope, or never baselined, starts at zero (Review Focus 4).
func TestCheckPortMissingFromBaselineMustBeZero(t *testing.T) {
	got := Baseline{"aaa": 3}.Check(map[string]int{"newport": 1})
	if len(got) != 1 || !strings.Contains(got[0], "newport") {
		t.Errorf("got %q", got)
	}
}

func TestNoBaselineFileMeansZeroEverywhere(t *testing.T) {
	b, ok, err := LoadBaseline(filepath.Join(t.TempDir(), "lint-baseline.json"))
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v, want a missing file reported as not ok", ok, err)
	}
	if got := b.Check(map[string]int{"aaa": 1, "config": 0}); len(got) != 1 {
		t.Errorf("got %q, want aaa to fail", got)
	}
}

func TestUpdateOnlyLowers(t *testing.T) {
	b := Baseline{"aaa": 3, "config": 2, "opd": 5}
	got, err := b.Update(map[string]int{"aaa": 1, "config": 0, "opd": 9}, []string{"aaa", "config"})
	if err != nil {
		t.Fatal(err)
	}
	if want := (Baseline{"aaa": 1, "opd": 5}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if _, err := b.Update(map[string]int{"opd": 9}, []string{"opd"}); err == nil || !strings.Contains(err.Error(), "opd") {
		t.Errorf("an increase was accepted: %v", err)
	}
	if b["aaa"] != 3 {
		t.Error("Update changed the receiver")
	}
}

func TestSaveIsStable(t *testing.T) {
	dir := t.TempDir()
	p1, p2 := filepath.Join(dir, "a.json"), filepath.Join(dir, "b.json")
	if err := (Baseline{"opd": 5, "aaa": 1, "config": 2}).Save(p1); err != nil {
		t.Fatal(err)
	}
	if err := (Baseline{"config": 2, "aaa": 1, "opd": 5}).Save(p2); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(p1)
	b, _ := os.ReadFile(p2)
	if !bytes.Equal(a, b) || strings.Count(string(a), "\n") < 3 {
		t.Errorf("not stable or not one port per line:\n%s\n%s", a, b)
	}
	got, ok, err := LoadBaseline(p1)
	if err != nil || !ok || got["opd"] != 5 {
		t.Errorf("round trip: %v %v %v", got, ok, err)
	}
}

func TestNewBaselineOmitsZero(t *testing.T) {
	if got := NewBaseline(map[string]int{"aaa": 2, "config": 0}); !reflect.DeepEqual(got, Baseline{"aaa": 2}) {
		t.Errorf("got %v", got)
	}
}
