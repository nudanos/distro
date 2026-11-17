interfaces {
	dataplane dp0s3 {
		firewall {
			in SAMPLER
		}
	}
	dataplane dp0s10 {
		address 10.0.2.15/24
	}
	loopback lo
}
security {
	firewall {
		name SAMPLER {
			default-action drop
		}
	}
}
