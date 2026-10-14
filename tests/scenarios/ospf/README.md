# ospf

New for NuDanOS (no DANOS suite). R1 (area 0) – R2 (area border) – R3 (area 1),
loopbacks 10.255.0.N/32: adjacencies Full, an inter-area route through R2, a
loopback-to-loopback ping, and a cost change reflected in the route's metric.

R1 and R3 have OSPF priority 0 on their links to R2, so R2 is the designated
router on both links whichever router boots first (with equal priorities the
first router up on a link keeps the DR role, and the show comparison with
2105 changed from run to run).
