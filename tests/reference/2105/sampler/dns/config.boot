interfaces {
	dataplane dp0s10 {
		address 10.0.2.15/24
	}
	loopback lo
}
service {
	ssh
}
system {
	acm {
		enable
		operational-ruleset {
			rule 9969 {
				action deny
				command /show/log/audit
				group vyattaop
			}
			rule 9970 {
				action deny
				command "/add/system/image/*/packages/*"
				group vyattaop
				group vyattaadm
			}
			rule 9971 {
				action deny
				command "/spawn/*"
				group vyattaop
				group vyattaadm
			}
			rule 9972 {
				action deny
				command "/set/console/*"
				group vyattaop
			}
			rule 9973 {
				action deny
				command "/set/date/*"
				group vyattaop
			}
			rule 9974 {
				action deny
				command "/set/system/*"
				group vyattaop
			}
			rule 9975 {
				action allow
				command "/set/*"
				group vyattaop
			}
			rule 9976 {
				action allow
				command "/twping/*"
				group vyattaop
			}
			rule 9977 {
				action allow
				command /show/tech-support/save
				group vyattaop
			}
			rule 9978 {
				action deny
				command "/show/tech-support/save/*"
				group vyattaop
			}
			rule 9979 {
				action allow
				command /show/tech-support/save-uncompressed
				group vyattaop
			}
			rule 9980 {
				action deny
				command "/show/tech-support/save-uncompressed/*"
				group vyattaop
			}
			rule 9981 {
				action allow
				command /show/tech-support/brief/save
				group vyattaop
			}
			rule 9982 {
				action deny
				command "/show/tech-support/brief/save/*"
				group vyattaop
			}
			rule 9983 {
				action allow
				command /show/tech-support/brief/save-uncompressed
				group vyattaop
			}
			rule 9984 {
				action deny
				command "/show/tech-support/brief/save-uncompressed/*"
				group vyattaop
			}
			rule 9985 {
				action allow
				command /show/tech-support/brief/
				group vyattaop
			}
			rule 9986 {
				action deny
				command /show/tech-support/brief
				group vyattaop
			}
			rule 9987 {
				action deny
				command /show/tech-support
				group vyattaop
			}
			rule 9988 {
				action deny
				command /show/configuration
				group vyattaop
			}
			rule 9989 {
				action allow
				command "/clear/*"
				group vyattaop
			}
			rule 9990 {
				action allow
				command "/show/*"
				group vyattaop
			}
			rule 9991 {
				action allow
				command "/monitor/*"
				group vyattaop
			}
			rule 9992 {
				action allow
				command "/ping/*"
				group vyattaop
			}
			rule 9993 {
				action allow
				command "/reset/*"
				group vyattaop
			}
			rule 9994 {
				action allow
				command "/release/*"
				group vyattaop
			}
			rule 9995 {
				action allow
				command "/renew/*"
				group vyattaop
			}
			rule 9996 {
				action allow
				command "/telnet/*"
				group vyattaop
			}
			rule 9997 {
				action allow
				command "/traceroute/*"
				group vyattaop
			}
			rule 9998 {
				action allow
				command "/update/*"
				group vyattaop
			}
			rule 9999 {
				action deny
				command "*"
				group vyattaop
			}
		}
		rpc-ruleset {
			rule 9981 {
				action allow
				group vyattaop
				rpc-name vyatta-system-tacplus-v1:reset-suppression-timers
			}
			rule 9982 {
				action allow
				group vyattaop
				rpc-name vyatta-system-bmc-v1:clear-bmc-sel
			}
			rule 9983 {
				action allow
				group vyattaop
				rpc-name vyatta-interfaces-dataplane-transceiver-v1:xcvr-info
			}
			rule 9984 {
				action allow
				group vyattaop
				rpc-name vyatta-interfaces-dataplane-ethernet-info-v1:eth-info
			}
			rule 9985 {
				action allow
				group vyattaop
				rpc-name vyatta-interfaces-dataplane-rpc-v1:slowpath-info
			}
			rule 9986 {
				action allow
				group vyattaop
				rpc-name vyatta-interfaces-dataplane-rpc-v1:identify-info
			}
			rule 9987 {
				action allow
				group vyattaop
				rpc-name vyatta-interfaces-dataplane-rpc-v1:affinity-info
			}
			rule 9988 {
				action allow
				group vyattaop
				rpc-name vyatta-system-storage-v1:clear-block-device-unused-blocks
			}
			rule 9989 {
				action allow
				group vyattaop
				rpc-name vyatta-system-storage-v1:get-block-device
			}
			rule 9990 {
				action allow
				group vyattaop
				rpc-name vyatta-cpp-rate-limiter-v1:clear-statistics
			}
			rule 9991 {
				action allow
				group vyattaop
				rpc-name vyatta-ippf-v1:clear-statistics
			}
			rule 9992 {
				action allow
				group vyattaop
				rpc-name vyatta-ippf-v1:get-statistics
			}
			rule 9993 {
				action allow
				group vyattaop
				rpc-name vyatta-virtualization-v1:list-images
			}
			rule 9994 {
				action allow
				group vyattaop
				rpc-name vyatta-system-sensor-v1:system-event-logs
			}
			rule 9995 {
				action allow
				group vyattaop
				rpc-name vyatta-service-twamp-v1:twping
			}
			rule 9996 {
				action allow
				group vyattaop
				rpc-name vyatta-op-v1:route
			}
			rule 9997 {
				action allow
				group vyattaop
				rpc-name vyatta-op-v1:interface
			}
			rule 9998 {
				action allow
				group vyattaop
				rpc-name vyatta-op-v1:ping
			}
			rule 9999 {
				action deny
				group vyattaop
				module-name "*"
				rpc-name "*"
			}
		}
		ruleset {
			rule 9998 {
				action allow
				group vyattapwdcfg
				operation "*"
				path "/system/login/user/*/authentication/*"
			}
			rule 9999 {
				action allow
				group vyattacfg
				operation "*"
				path "*"
			}
		}
	}
	config-management {
		commit-revisions 20
	}
	console {
		device ttyS0 {
			speed 115200
		}
	}
	domain-name example.net
	host-name r1
	login {
		user tmpuser {
			level admin
		}
	}
	name-server 192.0.2.53
}
