package run

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/multiformats/go-multiaddr"

	"github.com/joho/godotenv"
	"github.com/sisu-network/dheart/client"
	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/server"
	"github.com/sisu-network/dheart/store"
	"github.com/sisu-network/dheart/utils"
)

func LoadConfigEnv(filenames ...string) {
	err := godotenv.Load(filenames...)
	if err != nil {
		panic(err)
	}
}

func GetSisuClient() client.Client {
	url := os.Getenv("SISU_SERVER_URL")
	c := client.NewClient(url)
	return c
}

func GetHeart(conConfig p2p.ConnectionsConfig, client client.Client) *core.Heart {
	// DB Config
	port, err := strconv.Atoi(os.Getenv("DB_PORT"))
	if err != nil {
		panic(err)
	}

	dbConfig := config.DbConfig{
		Port:          port,
		Host:          os.Getenv("DB_HOST"),
		Username:      os.Getenv("DB_USERNAME"),
		Password:      os.Getenv("DB_PASSWORD"),
		Schema:        os.Getenv("DB_SCHEMA"),
		MigrationPath: os.Getenv("DB_MIGRATION_PATH"),
	}

	aesKey, err := hex.DecodeString(os.Getenv("AES_KEY_HEX"))
	if err != nil {
		panic(err)
	}

	// Heart Config
	heartConfig := config.HeartConfig{
		Db:         dbConfig,
		AesKey:     aesKey,
		Connection: conConfig,
	}

	return core.NewHeart(heartConfig, client)
}

func getConnectionConfig() p2p.ConnectionsConfig {
	var connectionConfig p2p.ConnectionsConfig
	connectionConfig.HostId = "localhost"
	port, err := strconv.Atoi(os.Getenv("DHEART_PORT"))
	if err != nil {
		panic(err)
	}
	connectionConfig.Port = port

	// Bootstrapped peers.
	connectionConfig.BootstrapPeers = make([]multiaddr.Multiaddr, 0)
	peerString := os.Getenv("BOOTSTRAP_PEERS")
	peers := strings.Split(peerString, ",")
	for _, peerInfo := range peers {
		arr := strings.Split(peerInfo, "@")
		if len(arr) != 2 {
			panic(fmt.Errorf("invalid peer info: %s", peerInfo))
		}
		peerId := arr[0]
		ip := arr[1]

		mulAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", ip, port, peerId))
		if err != nil {
			panic(err)
		}
		connectionConfig.BootstrapPeers = append(connectionConfig.BootstrapPeers, mulAddr)
	}

	return connectionConfig
}

func SetupApiServer() {
	c := GetSisuClient()

	path := os.Getenv("HOME_DIR")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}

	aesKey, err := hex.DecodeString(os.Getenv("AES_KEY_HEX"))
	if err != nil {
		panic(err)
	}

	store, err := store.NewStore(path+"/apidb", aesKey)
	if err != nil {
		panic(err)
	}

	handler := rpc.NewServer()
	if os.Getenv("USE_ON_MEMORY") == "true" {
		utils.LogInfo("Running single node mode...")
		api := server.NewSingleNodeApi(c, store)
		api.Init()

		handler.RegisterName("tss", api)
	} else {
		// Use Heart
		utils.LogInfo("Running multiple nodes mode...")
		connectionConfig := getConnectionConfig()
		heart := GetHeart(connectionConfig, client.NewClient(os.Getenv("SISU_SERVER_URL")))
		handler.RegisterName("tss", server.NewTssApi(heart))
	}

	s := server.NewServer(handler, "0.0.0.0", 5678)

	go c.TryDial()
	go s.Run()
}

func Run() {
	LoadConfigEnv()
	SetupApiServer()
}
