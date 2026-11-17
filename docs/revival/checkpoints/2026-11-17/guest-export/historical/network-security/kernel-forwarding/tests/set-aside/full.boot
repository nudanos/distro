interfaces {
	dataplane dp0s3 {
		address 10.0.0.1/24
		cpu-affinity 1
		description "uplink { not a block }"
		speed 100m
	}
	dataplane dp0s4 {
		address 10.0.1.1/24
		speed auto
	}
	loopback lo {
	}
}
/* a comment the operator wrote */
policy {
	qos {
		name LIMIT
	}
}
service {
	ssh {
	}
}
system {
	host-name r1
}
/* Warning: Do not remove the following line. */
/* === vyatta-config-version: "system@10" === */
/* Release: 2105 */
