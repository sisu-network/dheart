package config

import (
	"github.com/BurntSushi/toml"
	p2ptypes "github.com/sisu-network/dheart/p2p/types"
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

	Db         DbConfig                   `toml:"db"`
	Connection p2ptypes.ConnectionsConfig `toml:"connection"`

	LogDNA LogDNA `toml:"log_dna"`

	// Key to decrypt data sent over network.
	AesKey []byte
}

type LogDNA struct {
	Secret        string   `toml:"secret"`
	AppName       string   `toml:"app_name"`
	HostName      string   `toml:"host_name"`
	FlushInterval duration `toml:"flush_interval"`
	MaxBufferLen  int      `toml:"max_buffer_len"`
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
