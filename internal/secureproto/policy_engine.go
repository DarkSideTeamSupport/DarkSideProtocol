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
