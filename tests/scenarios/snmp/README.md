# snmp

New for NuDanOS (no DANOS suite). One router with an SNMPv2c community and an
SNMPv3 authPriv user: sysName, IF-MIB ifName (2105 puts the hardware description in ifDescr), and dp0s10's ifInOctets growing
after traffic (`snmp-counter-moves` in `internal/scenario/gochecks.go`).
