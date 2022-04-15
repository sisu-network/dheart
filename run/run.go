package run

import (
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/joho/godotenv"
	"github.com/logdna/logdna-go/logger"

	"github.com/sisu-network/dheart/client"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/server"
	"github.com/sisu-network/dheart/store"
	"github.com/sisu-network/lib/log"
)

func LoadConfigEnv(filenames ...string) {
	err := godotenv.Load(filenames...)
	if err != nil {
		panic(err)
	}
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

	if len(cfg.LogDNA.Secret) > 0 {
		opts := logger.Options{
			App:           cfg.LogDNA.AppName,
			FlushInterval: cfg.LogDNA.FlushInterval.Duration,
			Hostname:      cfg.LogDNA.HostName,
			MaxBufferLen:  cfg.LogDNA.MaxBufferLen,
		}
		setupLogger(cfg.LogDNA.Secret, opts)
	}

	log.Info("homeDir = ", homeDir)
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
	go s.Run()

	go c.TryDial()
}

func setupLogger(key string, options logger.Options) {
	logDNA := log.NewDNALogger(key, options, false)
	log.SetLogger(logDNA)
}

func Run() {
	LoadConfigEnv()
	SetupApiServer()
}
