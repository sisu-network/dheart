package core

import (
	chaineth "github.com/sisu-network/tuktuk/chain/eth"
	common "github.com/sisu-network/tuktuk/common"
)

const (
	CHAIN_ETH = "eth"
)

type TutTuk struct {
}

func NewTutTuk() *TutTuk {
	return &TutTuk{}
}

func (t *TutTuk) Setup(configs []common.ChainConfig) {
	for _, config := range configs {
		switch config.ChainSymbol {
		case CHAIN_ETH:
			chaineth.NewEthBridge(config.ChainId, config.ChainSymbol, config.Url)
		}
	}
}
