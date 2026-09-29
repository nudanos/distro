from __future__ import annotations

import importlib.util
import os
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("set_ready", os.path.join(HERE, "set_ready.py"))
sr = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sr)

TEXT = """# header
packages:
  - name: "a"
    kind: "danos"
    milestone: "1.0"
    repo: "https://github.com/nudanos/a"
    ref: "trixie"
  - name: "b"
    kind: "danos"
    milestone: "1.0"
    ready: true
    repo: "https://github.com/nudanos/b"
    ref: "trixie"
"""


class SetReadyTest(unittest.TestCase):
    def test_sets_after_milestone_and_is_idempotent(self):
        out = sr.set_ready(TEXT, ["a", "b"])
        self.assertIn('  - name: "a"\n    kind: "danos"\n    milestone: "1.0"\n    ready: true\n    repo:', out)
        self.assertEqual(out.count("ready: true"), 2)
        self.assertEqual(sr.set_ready(out, ["a", "b"]), out)

    def test_unknown_name(self):
        with self.assertRaises(SystemExit):
            sr.set_ready(TEXT, ["zzz"])


if __name__ == "__main__":
    unittest.main()
