package run

import (
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/rpc"

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
	connectionConfig.Port = cfg.Port

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
