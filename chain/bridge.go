package chain

type Bridge interface {
	GetChainId() int
	GetChainSymbol() string

	DeliverTx(tx interface{}) error
	GetBlock(height int) interface{}
}
