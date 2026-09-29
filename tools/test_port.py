from __future__ import annotations

import importlib.util
import os
import shutil
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("port", os.path.join(HERE, "port.py"))
port = importlib.util.module_from_spec(spec)
spec.loader.exec_module(port)

OLD_CONTROL = """Source: vyatta-cfg-system
Section: contrib/net
Priority: extra
Maintainer: Vyatta Package Maintainers <DL-vyatta-help@att.com>
Uploaders: Someone <x@brocade.com>
Build-Depends: debhelper (>= 9.20160709),
 dh-systemd,
 dh-yang,
 pylint3,
 python3-pytest-pep8,
 bvnos-linux-libc-dev (>> 4.19),
 python-setuptools,
 autotools-dev
Standards-Version: 3.9.6
Vcs-Git: git://git.vyatta.com/vyatta-cfg-system

Package: vyatta-cfg-system
Architecture: all
Priority: extra
Section: contrib/admin
Depends: ${misc:Depends}
Description: system config
"""


class ControlTest(unittest.TestCase):
    def test_source_paragraph(self):
        text, notes = port.port_control(OLD_CONTROL, "vyatta-cfg-system")
        src = text.split("\n\n")[0]
        self.assertIn("Maintainer: NuDanOS Maintainers <jon@fernandez.tech>", src)
        self.assertNotIn("Uploaders", src)
        self.assertIn("Standards-Version: 4.7.2", src)
        self.assertIn("Rules-Requires-Root: no", src)
        self.assertIn("Vcs-Git: https://github.com/nudanos/vyatta-cfg-system.git", src)
        self.assertIn("Vcs-Browser: https://github.com/nudanos/vyatta-cfg-system", src)
        self.assertIn("Section: net", src)
        self.assertIn("Priority: optional", src)
        bd = src.split("Build-Depends:")[1].split("\nStandards")[0]
        deps = [d.strip() for d in bd.replace("\n", " ").split(",")]
        self.assertEqual(deps, ["debhelper-compat (= 13)", "dh-yang", "pylint", "linux-libc-dev",
                                "python3-setuptools"])
        self.assertTrue(any("bvnos" in n for n in notes))

    def test_binary_paragraphs(self):
        text, _ = port.port_control(OLD_CONTROL, "vyatta-cfg-system")
        binp = text.split("\n\n")[1]
        self.assertIn("Section: admin", binp)
        self.assertNotIn("Priority", binp)
        self.assertIn("Description: system config", binp)

    def test_idempotent(self):
        once, _ = port.port_control(OLD_CONTROL, "vyatta-cfg-system")
        twice, notes = port.port_control(once, "vyatta-cfg-system")
        self.assertEqual(once, twice)
        self.assertEqual(notes, [])


class RulesTest(unittest.TestCase):
    def test_drops_obsolete_addons(self):
        rules = "%:\n\tdh $@ --with systemd,python3,yang --parallel\n\noverride_x:\n\tdh $@ --with=autotools_dev\n"
        out = port.port_rules(rules)
        self.assertIn("\tdh $@ --with python3,yang\n", out)
        self.assertIn("\tdh $@\n", out)
        self.assertEqual(port.port_rules(out), out)


class VersionTest(unittest.TestCase):
    def test_bump(self):
        self.assertEqual(port.bump_version("1.29"), "1.30")
        self.assertEqual(port.bump_version("2.12"), "2.13")
        self.assertEqual(port.bump_version("1:6"), "1:7")
        self.assertEqual(port.bump_version("0.1.27"), "0.1.28")
        self.assertEqual(port.bump_version("1.0.1-1"), "1.0.1-1+nudanos1")
        self.assertEqual(port.bump_version("4.0.0~git20170308-0vyatta3"), "4.0.0~git20170308-0vyatta3+nudanos1")


class TreeTest(unittest.TestCase):
    def setUp(self):
        self.d = tempfile.mkdtemp()
        os.makedirs(os.path.join(self.d, "debian", "source"))
        with open(os.path.join(self.d, "debian", "control"), "w") as f:
            f.write(OLD_CONTROL)
        with open(os.path.join(self.d, "debian", "compat"), "w") as f:
            f.write("9\n")
        with open(os.path.join(self.d, "debian", "rules"), "w") as f:
            f.write("%:\n\tdh $@ --with systemd\n")
        with open(os.path.join(self.d, "debian", "changelog"), "w") as f:
            f.write("vyatta-cfg-system (2.35) unstable; urgency=medium\n\n  * Old.\n\n -- V <v@v>  Mon, 01 Jan 2021 00:00:00 +0000\n")

    def tearDown(self):
        shutil.rmtree(self.d)

    def test_port_tree_then_rerun_changes_nothing(self):
        port.port_tree(self.d, "vyatta-cfg-system", date="Tue, 29 Sep 2026 12:00:00 +0000")
        self.assertFalse(os.path.exists(os.path.join(self.d, "debian", "compat")))
        self.assertEqual(open(os.path.join(self.d, "debian", "rules")).read(), "%:\n\tdh $@\n")
        cl = open(os.path.join(self.d, "debian", "changelog")).read()
        self.assertTrue(cl.startswith("vyatta-cfg-system (2.36) trixie; urgency=medium\n"))
        self.assertIn(" -- NuDanOS Maintainers <jon@fernandez.tech>  Tue, 29 Sep 2026 12:00:00 +0000", cl)
        snapshot = {p: open(os.path.join(self.d, "debian", p)).read() for p in ("control", "rules", "changelog")}
        port.port_tree(self.d, "vyatta-cfg-system", date="Wed, 30 Sep 2026 12:00:00 +0000")
        again = {p: open(os.path.join(self.d, "debian", p)).read() for p in ("control", "rules", "changelog")}
        self.assertEqual(snapshot, again)


if __name__ == "__main__":
    unittest.main()
