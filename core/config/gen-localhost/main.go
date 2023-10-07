package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sisu-network/dheart/core/config"
	p2ptypes "github.com/sisu-network/dheart/p2p/types"
)

func genLocalhostConfig() {
	cfg := config.HeartConfig{}

	cfg.HomeDir = filepath.Join(os.Getenv("HOME"), ".sisu/dheart")
	cfg.UseOnMemory = true
	cfg.Port = 28300
	cfg.SisuServerUrl = "http://0.0.0.0:25456"

	path, _ := os.Getwd()

	cfg.Db = config.DbConfig{
		Host:          "0.0.0.0",
		Port:          3306,
		Username:      "root",
		Password:      "password",
		Schema:        "dheart",
		MigrationPath: path + "/db/migrations/",
	}

	peer := &p2ptypes.Peer{
		Address:    "/dns/dheart0/tcp/28300/p2p/12D3KooWD6JaQEHnpeCKHZ1bYA9axESG1MyqTRWRLhkf7btYYpRk",
		PubKey:     "30a84ac6ed8306d5d5160c763cd90a0450eff4f77e3bc1f0fd2cff9abdca0d5f",
		PubKeyType: "ed25519",
	}
	peers := make([]*p2ptypes.Peer, 0)
	peers = append(peers, peer)

	cfg.Connection = p2ptypes.ConnectionsConfig{
		Host:       "127.0.0.1",
		Port:       28300,
		Rendezvous: "rendezvous",
		Peers:      peers,
	}

	configFilePath := filepath.Join(cfg.HomeDir, "./dheart.toml")

	os.MkdirAll(cfg.HomeDir, os.ModePerm)
	config.WriteConfigFile(configFilePath, cfg)
	fmt.Printf("Successfully write config file %s\n", configFilePath)
}

func genEnv() {
	homeDir := filepath.Join(os.Getenv("HOME"), ".sisu/dheart")
	content := fmt.Sprintf(`HOME_DIR=%s
AES_KEY_HEX=c787ef22ade5afc8a5e22041c17869e7e4714190d88ecec0a84e241c9431add0
`, homeDir)

	os.WriteFile(".env", []byte(content), 0600)
	fmt.Printf("Successfully write config file .env\n")
}

func main() {
	genLocalhostConfig()
	genEnv()
}
