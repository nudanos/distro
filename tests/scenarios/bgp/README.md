# bgp

Translated from the DANOS BGP suite (`nudanos/tests`
`Test_Automation/script/BGP_DANOS.robot`, data in
`testdata/BGP_DANOS_testdata.robot`). Four routers; addressing follows the
suite without its LAN hosts. The suite's test cases become ten phases, each
an action on the routers it reconfigures followed by its checks. Command
forms are DANOS's own (`show protocols bgp ...`), as the suite used them.

| Phase | Checks | Suite test case |
|---|---|---|
| 1 eBGP direct | R2 learns 1.1.1.1/32 from 201.1.1.3; ping | Verify EBGP on directly connected neighbors |
| 2 eBGP multihop | R2 learns 1.1.1.1/32 over the loopback session (`ebgp-multihop 3`, `update-source`, static routes) | Verify EBGP Neighbor with Multihop option |
| 3 iBGP direct | R3 learns 1.1.1.2/32 from 202.1.1.3 | Verify iBGP on directly connected neighbors |
| 4 iBGP multihop | R3 learns 1.1.1.2/32 over the loopback session (static routes stand in for the suite's OSPF) | Verify iBGP Neighbor with Multihop option |
| 5 route reflection | client to client, client to non-client, non-client to client, each with an `Originator:` | Route Reflector Rule-1, -2, -3 |
| 6 next hop | R3 sees the eBGP next hop 201.1.1.3, then 202.1.1.3 with `nexthop-self` on R2 | Validate Next-hop attribute |
| 7 iBGP vs eBGP | R4 holds both paths to 1.1.1.1/32; the eBGP one is best | Validate BGP route selection between iBGP v/s eBGP |
| 8 local preference | R3's path from R2 carries `localpref 300` | Verify local preference |
| 9 confederation | R3 sees `(65002) 100`; R1 sees AS 200 and no sub-AS | Validate BGP confederation |
| 10 dampening | R2 counts two flaps of 1.1.1.1/32 | Verify Route Flaping/dampening Feature |

Show output (`show protocols bgp all summary`, `show protocols bgp ipv4
unicast`) is captured after phase 8 (`capture_show`), before the
confederation reshapes the AS layout.

Differences from the suite, all deliberate: router ids are set explicitly
(1.1.1.N) so originator and cluster ids are stable; route reflection runs
with R4 as the reflector (R2 and R3 clients, R1 a non-client in AS 200), as
the plan's brief places it; dampening runs on R2 and counts two flaps
rather than waiting for the dampened state, which takes a third flap and
minutes of penalty decay.
