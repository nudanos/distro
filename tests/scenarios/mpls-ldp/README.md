# mpls-ldp

New for NuDanOS (no DANOS suite). Reported, not gating: MPLS forwarding
depends on kernel modules, `net.mpls.*` sysctls and FRR's ldpd, and a
failure here is recorded rather than blocking a release.

R1 – R2 – R3 with loopbacks 10.255.0.N/32, OSPF area 0 for reachability,
LDP on both links with the loopbacks as transport addresses and labels for
host routes. Checks: R2 has two operational sessions, R1 holds R2's label
for R3's loopback, and a loopback-to-loopback ping crosses the LSP.

Known NuDanOS 1.0 limitation (reported, not gating): on NuDanOS LDP
discovery works — each router sees its neighbours' hellos and creates the
neighbour — but the TCP session to the transport address fails
("nbr_establish_connection: error while connecting to 10.255.0.1") and no
session reaches OPERATIONAL, so no labels are exchanged. The kernel MPLS
modules are loaded and OSPF reachability between the loopbacks works (the
ping passes). DANOS 2105 establishes both sessions with the same
configuration; its output is the reference.
