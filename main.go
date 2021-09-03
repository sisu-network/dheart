package main

import (
	"math/big"
	"os/signal"
	"strconv"
	"syscall"

	"os"

	"github.com/sisu-network/tss-lib/tss"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/joho/godotenv"
	"github.com/sisu-network/dheart/client"
	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/server"
)

func initialize() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
}

func getSisuClient() *client.DefaultClient {
	url := os.Getenv("SISU_SERVER_URL")
	c := client.NewClient(url)
	return c
}

func getHeart() *core.Heart {
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

	// Heart Config
	heartConfig := config.HeartConfig{
		Db:     dbConfig,
		AesKey: os.Getenv("AES_KEY"),
	}

	return core.NewHeart(heartConfig)
}

func setupApiServer() {
	c := getSisuClient()

	dheart := getHeart()

	handler := rpc.NewServer()
	if os.Getenv("USE_ON_MEMORY") == "" {
		// handler.RegisterName("tss", getHeart())
		handler.RegisterName("tss", server.NewTssApi(dheart))
	} else {
		api := server.NewSingleNodeApi(dheart, c)
		api.Init()

		handler.RegisterName("tss", api)
	}

	s := server.NewServer(handler, "localhost", 5678)

	go c.TryDial()
	go s.Run()
}

func getSortedPartyIds(n int) tss.SortedPartyIDs {
	keys := p2p.GetAllPrivateKeys(n)
	partyIds := make([]*tss.PartyID, n)

	// Creates list of party ids
	for i := 0; i < n; i++ {
		bz := keys[i].PubKey().Bytes()
		peerId := p2p.P2PIDFromKey(keys[i])
		party := tss.NewPartyID(peerId.String(), "", new(big.Int).SetBytes(bz))
		partyIds[i] = party
	}

	return tss.SortPartyIDs(partyIds, 0)
}

func main() {
	initialize()
	setupApiServer()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

}
