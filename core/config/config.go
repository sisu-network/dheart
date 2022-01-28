package config

import (
	"github.com/BurntSushi/toml"
	"github.com/sisu-network/dheart/p2p"
)

type DbConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Username string `toml:"username"`
	Password string `toml:"password"`
	Schema   string `toml:"schema"`
}

type HeartConfig struct {
	HomeDir           string `toml:"home-dir"`
	UseOnMemory       bool   `toml:"use-on-memory"`
	ShortcutPreparams bool   `toml:"shortcut-preparams"`
	SisuServerUrl     string `toml:"sisu-server-url"`
	Port              int    `toml:"port"`

	Db         DbConfig              `toml:"db"`
	Connection p2p.ConnectionsConfig `toml:"connection"`

	// Key to decrypt data sent over network.
	AesKey []byte
}

func ReadConfig(path string) (HeartConfig, error) {
	cfg := HeartConfig{}

	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

func GetLocalhostDbConfig() DbConfig {
	return DbConfig{
		Host:     "0.0.0.0",
		Port:     3306,
		Username: "root",
		Password: "password",
		Schema:   "dheart",
	}
}
