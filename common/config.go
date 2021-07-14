package common

type Config struct {
	RpcHost string
	RpcPort int
}

type ChainConfig struct {
	ChainSymbol string
	Url         string
	ChainId     int
}
