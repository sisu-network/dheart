module github.com/sisu-network/dheart

go 1.16

require (
	github.com/BurntSushi/toml v0.4.1
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/deckarep/golang-set v1.7.1
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/ethereum/go-ethereum v1.10.12
	github.com/go-sql-driver/mysql v1.6.0
	github.com/golang-migrate/migrate v3.5.4+incompatible
	github.com/golang/protobuf v1.5.2
	github.com/joho/godotenv v1.3.0
	github.com/libp2p/go-libp2p v0.13.0
	github.com/libp2p/go-libp2p-core v0.8.0
	github.com/libp2p/go-libp2p-discovery v0.5.0
	github.com/libp2p/go-libp2p-kad-dht v0.11.1
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/sisu-network/cosmos-sdk v0.42.1-fork004
	github.com/sisu-network/lib v0.0.1-alpha10
	github.com/sisu-network/tss-lib v0.1.1
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7
	github.com/tendermint/tendermint v0.34.13 // indirect
	google.golang.org/protobuf v1.27.1
)

replace google.golang.org/grpc => google.golang.org/grpc v1.33.2

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
