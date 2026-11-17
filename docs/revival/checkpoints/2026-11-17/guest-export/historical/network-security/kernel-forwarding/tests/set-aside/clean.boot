interfaces {
	dataplane dp0s3 {
		address 10.0.0.1/24
		description "braces { } in a value"
		speed auto
	}
}
/* nothing here is DPDK-only */
system {
	host-name r1
}
/* === vyatta-config-version: "system@10" === */
