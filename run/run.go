package run

import (
	"encoding/hex"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/joho/godotenv"
	"github.com/sisu-network/dheart/client"
	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/server"
)

func LoadConfigEnv(filenames ...string) {
	err := godotenv.Load(filenames...)
	if err != nil {
		panic(err)
	}
}

func GetSisuClient() *client.DefaultClient {
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

	// Heart Config
	heartConfig := config.HeartConfig{
		Db:         dbConfig,
		AesKey:     aesKey,
		Connection: conConfig,
	}

	return core.NewHeart(heartConfig, client)
}

func SetupApiServer() {
	c := GetSisuClient()

	// TODO: Setup connection config
	heart := GetHeart(p2p.ConnectionsConfig{Port: 1000},
		client.NewClient(os.Getenv("SISU_SERVER_URL")),
	)

	handler := rpc.NewServer()
	if os.Getenv("USE_ON_MEMORY") == "true" {
		api := server.NewSingleNodeApi(heart, c)
		api.Init()

		handler.RegisterName("tss", api)
	} else {
		handler.RegisterName("tss", server.NewTssApi(heart))
	}

	s := server.NewServer(handler, "localhost", 5678)

	go c.TryDial()
	go s.Run()
}

func Run() {
	LoadConfigEnv()
	SetupApiServer()
}
