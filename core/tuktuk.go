package core

import (
	chaineth "github.com/sisu-network/dheart/chain/eth"
	common "github.com/sisu-network/dheart/common"
)

const (
	CHAIN_ETH = "eth"
)

type Dheart struct {
}

func NewTutTuk() *Dheart {
	return &Dheart{}
}

func (t *Dheart) Setup(configs []common.ChainConfig) {
	for _, config := range configs {
		switch config.ChainSymbol {
		case CHAIN_ETH:
			chaineth.NewEthBridge(config.ChainId, config.ChainSymbol, config.Url)
		}
	}
}
