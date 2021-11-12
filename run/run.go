package run

import (
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sisu-network/lib/log"

	"github.com/joho/godotenv"
	"github.com/sisu-network/dheart/client"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/server"
	"github.com/sisu-network/dheart/store"
)

func LoadConfigEnv(filenames ...string) {
	err := godotenv.Load(filenames...)
	if err != nil {
		panic(err)
	}
}

func SetupApiServer() {
	homeDir := os.Getenv("HOME_DIR")
	log.Info("homeDir = ", homeDir)

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

	cfg.AesKey = aesKey

	store, err := store.NewStore(filepath.Join(homeDir, "/apidb"), aesKey)
	if err != nil {
		panic(err)
	}

	c := client.NewClient(cfg.SisuServerUrl)

	handler := rpc.NewServer()
	serverApi := server.GetApi(cfg, store, c)
	serverApi.Init()
	handler.RegisterName("tss", serverApi)

	s := server.NewServer(handler, "0.0.0.0", uint16(cfg.Port))

	go c.TryDial()
	go s.Run()
}

func Run() {
	LoadConfigEnv()
	SetupApiServer()
}
