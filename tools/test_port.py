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

    def test_lazy_fixture_package_renamed(self):
        # pytest-lazy-fixture broke with pytest 8; Debian 13 ships the maintained pytest-lazy-fixtures.
        text, _ = port.port_control("Source: v\nBuild-Depends: debhelper (>= 9), python3-pytest-lazy-fixture\n\nPackage: v\n", "v")
        self.assertIn("python3-pytest-lazy-fixtures", text)
        self.assertNotIn("python3-pytest-lazy-fixture,", text + ",")

    def test_idempotent(self):
        once, _ = port.port_control(OLD_CONTROL, "vyatta-cfg-system")
        twice, notes = port.port_control(once, "vyatta-cfg-system")
        self.assertEqual(once, twice)
        self.assertEqual(notes, [])


class FieldCaseTest(unittest.TestCase):
    def test_lowercase_build_depends_is_recognised(self):
        # libvci writes "Build-depends:"; a second Build-Depends field breaks dpkg.
        text, _ = port.port_control("Source: libvci\nBuild-depends: debhelper (>= 9), dh-golang\n\nPackage: libvci1\n", "libvci")
        src = text.split("\n\n")[0]
        self.assertEqual(src.lower().count("build-depends:"), 1)
        self.assertIn("Build-Depends: debhelper-compat (= 13),\n dh-golang", src)


class RulesTest(unittest.TestCase):
    def test_golang_rules_get_gopath_mode(self):
        rules = "#!/usr/bin/make -f\nexport DH_GOPKG := github.com/danos/utils\n\n%:\n\tdh $@ --buildsystem=golang --with golang\n"
        out = port.port_rules(rules)
        self.assertIn("export GO111MODULE := off\n", out)
        self.assertEqual(port.port_rules(out), out)
        self.assertNotIn("GO111MODULE", port.port_rules("%:\n\tdh $@\n"))

    def test_gopath_export_gets_gopath_mode(self):
        # libvci builds Go from a plain Makefile ("go: cannot find main module") and exports GOPATH.
        rules = "#!/usr/bin/make -f\nexport GOPATH=/usr/share/gocode\n\n%:\n\tdh $@ --with python3\n"
        out = port.port_rules(rules)
        self.assertIn("export GO111MODULE := off\n", out)
        self.assertEqual(port.port_rules(out), out)

    def test_drops_obsolete_addons(self):
        rules = "%:\n\tdh $@ --with systemd,python3,yang --parallel\n\noverride_x:\n\tdh $@ --with=autotools_dev\n"
        out = port.port_rules(rules)
        self.assertIn("\tdh $@ --with python3,yang\n", out)
        self.assertIn("\tdh $@\n", out)
        self.assertEqual(port.port_rules(out), out)


class SystemdOverrideTest(unittest.TestCase):
    """compat 11 merged dh_systemd_enable/start into dh_installsystemd; dh aborts on the old overrides."""

    def test_enable_lines_get_start_flags(self):
        rules = ("%:\n\tdh $@\n\noverride_dh_systemd_enable:\n"
                 "\tdh_systemd_enable -p a --name=x x.service\n\tdh_systemd_enable --name=snmptrapd --no-enable\n"
                 "override_dh_systemd_start:\n\tdh_systemd_start --no-start\n")
        out = port.port_rules(rules)
        self.assertNotIn("dh_systemd_", out)
        self.assertIn("override_dh_installsystemd:\n"
                      "\tdh_installsystemd -p a --name=x --no-start x.service\n"
                      "\tdh_installsystemd --name=snmptrapd --no-enable --no-start\n", out)
        self.assertEqual(port.port_rules(out), out)

    def test_start_only_becomes_global_no_start(self):
        rules = ("override_dh_systemd_start:\n\tdh_systemd_start --no-start \\\n"
                 "\t\tvyatta-autoinstall.service \\\n\t\tother.service\n\noverride_dh_auto_test:\n\ttrue\n")
        out = port.port_rules(rules)
        self.assertIn("override_dh_installsystemd:\n\tdh_installsystemd --no-start\n", out)
        self.assertIn("override_dh_auto_test:\n\ttrue\n", out)
        self.assertNotIn("autoinstall", out)

    def test_dangling_continuation_keeps_flags(self):
        # vyatta-image-tools' override ends with "\\" before a blank line.
        out = port.port_rules("override_dh_systemd_start:\n\tdh_systemd_start --no-start \\\n\t\tvyatta-autoinstall.service \\\n\n")
        self.assertIn("\tdh_installsystemd --no-start\n", out)

    def test_enable_only_keeps_starting(self):
        out = port.port_rules("override_dh_systemd_enable:\n\tdh_systemd_enable --package p --name=m\n")
        self.assertEqual(out, "override_dh_installsystemd:\n\tdh_installsystemd --package p --name=m\n")


class SystemdStartSelectionTest(unittest.TestCase):
    """A start override that named units started only those; the rest were enabled, not started."""

    def test_enable_lines_not_in_start_override_get_no_start(self):
        # ifmgrd: both units enabled, only ifmgrd started.
        rules = ("override_dh_systemd_enable:\n\tdh_systemd_enable --name=ifmgrd\n"
                 "\tdh_systemd_enable --name=ifmgrctl-hook\n\n"
                 "override_dh_systemd_start:\n\tdh_systemd_start --name=ifmgrd\n")
        out = port.port_rules(rules)
        self.assertIn("override_dh_installsystemd:\n\tdh_installsystemd --name=ifmgrd\n"
                      "\tdh_installsystemd --name=ifmgrctl-hook --no-start\n", out)
        self.assertEqual(port.port_rules(out), out)

    def test_selective_start_without_enable_override_is_left_for_a_human(self):
        # vyatta-platform: only vyatta-sfpd.service started; its socket and
        # vyatta-platform-util's unit must stay stopped, which needs the unit list.
        rules = ("override_dh_systemd_start:\n\tdh_systemd_start -p vyatta-sfpd vyatta-sfpd.service\n")
        notes = []
        out = port.port_rules(rules, notes)
        self.assertEqual(out, rules)  # dh refuses the old target, so the build fails loudly
        self.assertTrue(any("vyatta-sfpd.service" in n and "--no-start" in n for n in notes), notes)

    def test_empty_overrides_mean_none(self):
        out = port.port_rules("override_dh_systemd_start:\n\noverride_dh_auto_test:\n\ttrue\n")
        self.assertIn("override_dh_installsystemd:\n\tdh_installsystemd --no-start\n", out)
        out = port.port_rules("override_dh_systemd_enable:\n\noverride_dh_systemd_start:\n\n")
        self.assertIn("\tdh_installsystemd --no-enable --no-start\n", out)

    def test_foreign_commands_are_kept_with_a_note(self):
        notes = []
        out = port.port_rules("override_dh_systemd_start:\n\tdh_systemd_start --no-start\n\ttouch stamp\n", notes)
        self.assertIn("\tdh_installsystemd --no-start\n\ttouch stamp\n", out)
        self.assertTrue(any("touch stamp" in n for n in notes), notes)


class MergedUsrTest(unittest.TestCase):
    """Debian 13 is merged-usr: lintian rejects files shipped under /lib, /bin, /sbin (aliased-location)."""

    def test_install_destinations(self):
        text = ("debian/x.service /lib/systemd/system\n"
                "platform/a.platform lib/vyatta-platform/platforms\n"
                "lib/vci-rollback-ephemeral/vci-rollback lib/vci-rollback-ephemeral/\n"
                "scripts/* opt/vyatta/sbin/\n"
                "usr/lib/foo\n")
        out, notes = port.port_install(text)
        self.assertEqual(out, ("debian/x.service /usr/lib/systemd/system\n"
                               "platform/a.platform usr/lib/vyatta-platform/platforms\n"
                               "lib/vci-rollback-ephemeral/vci-rollback usr/lib/vci-rollback-ephemeral/\n"
                               "scripts/* opt/vyatta/sbin/\n"
                               "usr/lib/foo\n"))
        self.assertEqual(port.port_install(out)[0], out)

    def test_single_column_under_lib_is_noted(self):
        out, notes = port.port_install("lib/systemd\n")
        self.assertEqual(out, "lib/systemd\n")
        self.assertTrue(notes)

    def test_links(self):
        self.assertEqual(port.port_links("bin/bash bin/vbash\nlib/x usr/share/x\n"),
                         "usr/bin/bash usr/bin/vbash\nusr/lib/x usr/share/x\n")


class TransformTest(unittest.TestCase):
    def test_backreferences_escaped_for_compat13(self):
        # debhelper compat 13 expands ${...} in config files; perl backreferences must be ${Dollar}{N}.
        t = "/etc/x.vyatta perl -pe 's|(a)b(c)|${1}d${2}|'\n"
        out = port.port_transform(t)
        self.assertEqual(out, "/etc/x.vyatta perl -pe 's|(a)b(c)|${Dollar}{1}d${Dollar}{2}|'\n")
        self.assertEqual(port.port_transform(out), out)


class VersionTest(unittest.TestCase):
    def test_bump(self):
        self.assertEqual(port.bump_version("1.29"), "1.30")
        self.assertEqual(port.bump_version("2.12"), "2.13")
        self.assertEqual(port.bump_version("1:6"), "1:7")
        self.assertEqual(port.bump_version("0.1.27"), "0.1.28")
        self.assertEqual(port.bump_version("1.0.1-1"), "1.0.1-1+nudanos1")
        self.assertEqual(port.bump_version("4.0.0~git20170308-0vyatta3"), "4.0.0~git20170308-0vyatta3+nudanos1")

    def test_bump_native_drops_the_revision(self):
        # A native package's version may not contain '-' (lintian malformed-debian-changelog-version).
        self.assertEqual(port.bump_version("1.0.1-1", native=True), "1.0.1+nudanos1")
        self.assertEqual(port.bump_version("4.2.0-1", native=True), "4.2.0+nudanos1")
        self.assertEqual(port.bump_version("2:1.3-0vyatta2", native=True), "2:1.3+nudanos1")
        self.assertEqual(port.bump_version("1.29", native=True), "1.30")


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
        with open(os.path.join(self.d, "debian", "x.transform"), "w") as f:
            f.write("/etc/a.vyatta perl -pe 's|(a)|${1}|'\n")
        with open(os.path.join(self.d, "debian", "changelog"), "w") as f:
            f.write("vyatta-cfg-system (2.35) unstable; urgency=medium\n\n  * Old.\n\n -- V <v@v>  Mon, 01 Jan 2021 00:00:00 +0000\n")

    def tearDown(self):
        shutil.rmtree(self.d)

    def test_port_tree_then_rerun_changes_nothing(self):
        port.port_tree(self.d, "vyatta-cfg-system", date="Tue, 29 Sep 2026 12:00:00 +0000")
        self.assertFalse(os.path.exists(os.path.join(self.d, "debian", "compat")))
        self.assertEqual(open(os.path.join(self.d, "debian", "rules")).read(), "%:\n\tdh $@\n")
        self.assertEqual(open(os.path.join(self.d, "debian", "x.transform")).read(),
                         "/etc/a.vyatta perl -pe 's|(a)|${Dollar}{1}|'\n")
        cl = open(os.path.join(self.d, "debian", "changelog")).read()
        self.assertTrue(cl.startswith("vyatta-cfg-system (2.36) trixie; urgency=medium\n"))
        self.assertIn(" -- NuDanOS Maintainers <jon@fernandez.tech>  Tue, 29 Sep 2026 12:00:00 +0000", cl)
        snapshot = {p: open(os.path.join(self.d, "debian", p)).read() for p in ("control", "rules", "changelog")}
        port.port_tree(self.d, "vyatta-cfg-system", date="Wed, 30 Sep 2026 12:00:00 +0000")
        again = {p: open(os.path.join(self.d, "debian", p)).read() for p in ("control", "rules", "changelog")}
        self.assertEqual(snapshot, again)

    def test_dirs_move_out_of_aliased_locations(self):
        # vci's deb-vci-helper.dirs created lib/vci/components (lintian aliased-location).
        with open(os.path.join(self.d, "debian", "deb-vci-helper.dirs"), "w") as f:
            f.write("lib/vci/components\n/sbin\nopt/vyatta/etc\n")
        port.port_tree(self.d, "vyatta-cfg-system", date="Tue, 29 Sep 2026 12:00:00 +0000")
        self.assertEqual(open(os.path.join(self.d, "debian", "deb-vci-helper.dirs")).read(),
                         "usr/lib/vci/components\n/usr/sbin\nopt/vyatta/etc\n")


if __name__ == "__main__":
    unittest.main()
