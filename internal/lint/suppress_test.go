package lint

import "testing"

// A suppression must say why on its own line (Review Focus 3).
func TestUnexplainedSuppressions(t *testing.T) {
	cases := []struct {
		name, lang, src string
		want            int
	}{
		{"go without reason", "go", "x := 1 //lint:ignore SA4006\n", 1},
		{"go file-ignore without reason", "go", "//lint:file-ignore SA1019\n", 1},
		{"go nolint without reason", "go", "f() //nolint\n", 1},
		{"go explained", "go", "//lint:ignore SA4006 reason: assigned for the debugger\n", 0},
		{"perl no critic without reason", "perl", "## no critic (ProhibitStringyEval)\n", 1},
		{"perl tidy block without reason", "perl", "#<<<\nmy @t = (1,2);\n#>>>\n", 1},
		{"perl explained", "perl", "## no critic (ProhibitStringyEval) reason: loads a generated module\n", 0},
		{"shell without reason", "shell", "# shellcheck disable=SC2086\necho $x\n", 1},
		{"shell empty reason", "shell", "# shellcheck disable=SC2086 # reason:   \n", 1},
		{"shell reason on the next line", "shell", "# shellcheck disable=SC2086\n# reason: word splitting is intended\n", 1},
		{"shell explained", "shell", "# shellcheck disable=SC2086 # reason: word splitting is intended\n", 0},
		{"python noqa without reason", "python", "import os  # noqa: F401\n", 1},
		{"python pylint without reason", "python", "x = 1  # pylint: disable=invalid-name\n", 1},
		{"python explained", "python", "import os  # noqa: F401  # reason: re-exported\n", 0},
		{"python marker inside a string", "python", "doc = \"# noqa: F401\"\n", 0},
		{"c cppcheck without reason", "c", "// cppcheck-suppress uninitvar\n", 1},
		{"c block cppcheck without reason", "c", "/* cppcheck-suppress uninitvar */\n", 1},
		{"c pragma without reason", "c", "#pragma GCC diagnostic ignored \"-Wunused\"\n", 1},
		{"c clang pragma without reason", "c", "#pragma clang diagnostic ignored \"-Wunused\"\n", 1},
		{"c explained", "c", "#pragma GCC diagnostic ignored \"-Wunused\" // reason: generated table\n", 0},
		{"c marker inside a string", "c", "puts(\"// cppcheck-suppress x\");\n", 0},
	}
	for _, c := range cases {
		got := Unexplained(c.lang, "f", []byte(c.src))
		if len(got) != c.want {
			t.Errorf("%s: %d findings, want %d: %v", c.name, len(got), c.want, got)
			continue
		}
		for _, f := range got {
			if f.Tool != "lint" || f.Rule != "suppression-without-reason" || f.File != "f" || f.Line != 1 {
				t.Errorf("%s: finding %+v", c.name, f)
			}
		}
	}
}
