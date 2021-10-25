package run

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/multiformats/go-multiaddr"

	"github.com/joho/godotenv"
	"github.com/sisu-network/dheart/client"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/server"
	"github.com/sisu-network/dheart/store"
)

func LoadConfigEnv(filenames ...string) {
	err := godotenv.Load(filenames...)
	if err != nil {
		panic(err)
	}
}

func getConnectionConfig(cfg config.HeartConfig) p2p.ConnectionsConfig {
	var connectionConfig p2p.ConnectionsConfig
	connectionConfig.HostId = "0.0.0.0"
	connectionConfig.Port = cfg.Port

	// Bootstrapped peers.
	connectionConfig.BootstrapPeerAddrs = make([]multiaddr.Multiaddr, 0)
	peerString := cfg.Connection.BootstrapPeers
	peers := strings.Split(peerString, ",")
	for _, peerInfo := range peers {
		arr := strings.Split(peerInfo, "@")
		if len(arr) != 2 {
			panic(fmt.Errorf("invalid peer info: %s", peerInfo))
		}
		peerId := arr[0]
		ip := arr[1]

		mulAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", ip, cfg.Port, peerId))
		if err != nil {
			panic(err)
		}
		connectionConfig.BootstrapPeerAddrs = append(connectionConfig.BootstrapPeerAddrs, mulAddr)
	}

	return connectionConfig
}

func SetupApiServer() {
	homeDir := os.Getenv("HOME_DIR")
	if _, err := os.Stat(homeDir); os.IsNotExist(err) {
		err := os.MkdirAll(homeDir, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	cfg, err := config.ReadConfig(filepath.Join(homeDir, "dheart.toml"))
	if err != nil {
		panic(err)
	}

	aesKey, err := hex.DecodeString(os.Getenv("AES_KEY_HEX"))
	if err != nil {
		panic(err)
	}

	store, err := store.NewStore(filepath.Join(homeDir, "/apidb"), aesKey)
	if err != nil {
		panic(err)
	}

	c := client.NewClient(cfg.SisuServerUrl)

	handler := rpc.NewServer()
	serverApi := server.GetApi(cfg, store, c)
	serverApi.Init()
	handler.RegisterName("tss", serverApi)

	s := server.NewServer(handler, "0.0.0.0", 5678)

	go c.TryDial()
	go s.Run()
}

func Run() {
	LoadConfigEnv()
	SetupApiServer()
}
