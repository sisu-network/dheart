package eth

type EthBridge struct {
	chainId     int
	chainSymbol string
}

func NewEthBridge(chainId int, chainSymbol string, url string) *EthBridge {
	return &EthBridge{
		chainId:     chainId,
		chainSymbol: chainSymbol,
	}
}

func (b *EthBridge) GetChainId() int {
	return b.chainId
}

func (b *EthBridge) GetChainSymbol() string {
	return b.chainSymbol
}

func (b *EthBridge) DeliverTx(tx interface{}) error {
	return nil
}

func (b *EthBridge) GetBlock(height int) interface{} {
	return nil
}
