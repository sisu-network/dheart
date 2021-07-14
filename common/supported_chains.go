package common

import "math/big"

const (
	CHAIN_ETH         = "eth"
	CHAIN_ETH_MAINNET = "eth-mainnet"
)

var (
	SUPPORTED_CHAINS = []string{
		CHAIN_ETH, // generic ETH chain
		CHAIN_ETH_MAINNET,
	}

	SUPPORTED_CHAINS_ID_MAP = map[string]*big.Int{}
)

func init() {
	SUPPORTED_CHAINS_ID_MAP["eth"] = big.NewInt(1)
}
