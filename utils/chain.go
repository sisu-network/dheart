package utils

func IsETHBasedChain(chain string) bool {
	switch chain {
	case "sisu-eth":
		return true
	case "eth":
		return true
	}

	return false
}
