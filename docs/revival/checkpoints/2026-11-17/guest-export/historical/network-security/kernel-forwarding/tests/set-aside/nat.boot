service {
	nat {
		source {
			rule 10 {
				translation {
					address masquerade
				}
			}
		}
	}
}
