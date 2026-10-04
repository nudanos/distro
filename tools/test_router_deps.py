from __future__ import annotations

import importlib.util
import os
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("router_deps", os.path.join(HERE, "router_deps.py"))
rd = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rd)

PACKAGES = """Package: vyatta-system
Source: vyatta-cfg-system
Architecture: amd64

Package: vplane-config
Source: vyatta-cfg-dataplane
Architecture: all

Package: libfoo-dev
Source: foo
Architecture: amd64

Package: vyatta-kernel-forwarding
Architecture: all

Package: vyatta-dataplane
Architecture: amd64
"""

DOC = """## DPDK-only binaries (left out of nudanos-router)

| Package | Why |
|---|---|
| vplane-config | the DPDK dataplane's config backend |

## Deferred to a later milestone

| Package | Milestone | Why |
|---|---|---|
"""


class RouterDepsTest(unittest.TestCase):
    def test_excludes_dpdk_only_dev_and_dataplane(self):
        self.assertEqual(rd.router_deps(PACKAGES, DOC), ["vyatta-kernel-forwarding", "vyatta-system"])

    def test_check_reports_drift(self):
        control = "Package: nudanos-router\nDepends: vyatta-system, ${misc:Depends}\n"
        missing, extra = rd.drift(control, PACKAGES, DOC)
        self.assertEqual((missing, extra), (["vyatta-kernel-forwarding"], []))

    def test_optional_section_is_left_out(self):
        doc = DOC + "\n## Optional (installable, left out of nudanos-router)\n\n| Package | Why |\n|---|---|\n| vyatta-system | needs the network at install time |\n"
        self.assertEqual(rd.router_deps(PACKAGES, doc), ["vyatta-kernel-forwarding"])

    def test_platform_section_is_left_out(self):
        pkgs = PACKAGES + "\nPackage: vyatta-interfaces-switch-deviations-siad-v1-yang\n"
        doc = DOC + "\n## Platform-specific (hardware deviations, left out of nudanos-router)\n\n| Package | Platform |\n|---|---|\n| vyatta-interfaces-switch-deviations-siad-v1-yang | SIAD |\n"
        self.assertEqual(rd.router_deps(pkgs, doc), ["vyatta-kernel-forwarding", "vyatta-system"])

    def test_undecided_platform_deviation_fails(self):
        # A new hardware deviation module must not reach every box by default.
        pkgs = PACKAGES + "\nPackage: vyatta-interfaces-tunnel-deviations-broadcom-dpp-v1-yang\n"
        with self.assertRaises(ValueError):
            rd.router_deps(pkgs, DOC)

    def test_danos_and_kernel_forwarding_deviations_stay(self):
        pkgs = PACKAGES + "\nPackage: vyatta-policy-route-deviation-danos-v1-yang\n\nPackage: vyatta-kernel-forwarding-deviations-v1-yang\n"
        self.assertIn("vyatta-policy-route-deviation-danos-v1-yang", rd.router_deps(pkgs, DOC))
        self.assertIn("vyatta-kernel-forwarding-deviations-v1-yang", rd.router_deps(pkgs, DOC))


if __name__ == "__main__":
    unittest.main()
