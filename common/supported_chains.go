package common

import "math/big"

const (
	CHAIN_ETH      = "eth"
	CHAIN_SISU_ETH = "sisu-eth"
)

var (
	SUPPORTED_CHAINS = []string{
		CHAIN_ETH, // generic ETH chain
		CHAIN_SISU_ETH,
	}

	SUPPORTED_CHAINS_ID_MAP = make(map[string]*big.Int)
)

func init() {
	// TODO: Add more chain ids here.
	SUPPORTED_CHAINS_ID_MAP[CHAIN_ETH] = big.NewInt(1)
	SUPPORTED_CHAINS_ID_MAP[CHAIN_SISU_ETH] = big.NewInt(36767)
}

func IsEthBasedChain(chain string) bool {
	switch chain {
	case CHAIN_ETH:
		return true
	case CHAIN_SISU_ETH:
		return true
	}

	return false
}
