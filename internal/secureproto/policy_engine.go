package secureproto

func SelectObfsMode(seq uint32, payloadLen int) string {
	if payloadLen > 900 {
		return "burst"
	}
	switch seq % 3 {
	case 0:
		return "drip"
	case 1:
		return "cover"
	default:
		return "adaptive"
	}
}

func SelectObfsModeForProfile(profile string, seq uint32, payloadLen int) string {
	switch profile {
	case "aggressive":
		if payloadLen > 1100 {
			return "burst"
		}
		if seq%2 == 0 {
			return "cover"
		}
		return "adaptive"
	case "recovery":
		if seq%2 == 0 {
			return "drip"
		}
		return "cover"
	default:
		return SelectObfsMode(seq, payloadLen)
	}
}
