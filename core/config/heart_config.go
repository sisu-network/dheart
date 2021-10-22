package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/sisu-network/dheart/p2p"
)

type DbConfig struct {
	Port          int
	Host          string
	Username      string
	Password      string
	Schema        string
	MigrationPath string
}

type HeartConfig struct {
	HomeDir       string `toml:"home-dir"`
	UseOnMemory   bool   `toml:"use-on-memory"`
	SisuServerUrl string `toml:"sisu-server-url"`
	Port          int    `toml:"port"`

	Db         DbConfig              `toml:"db"`
	Connection p2p.ConnectionsConfig `toml:"connection"`

	// Key to decrypt data sent over network.
	AesKey []byte
}

func ReadConfig(path string) (HeartConfig, error) {
	cfg := HeartConfig{}

	fmt.Println("path = ", path)

	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}
