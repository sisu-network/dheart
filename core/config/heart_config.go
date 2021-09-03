package config

import "github.com/sisu-network/dheart/p2p"

type DbConfig struct {
	Port          int
	Host          string
	Username      string
	Password      string
	Schema        string
	MigrationPath string
}

type HeartConfig struct {
	Db         DbConfig
	Connection p2p.ConnectionsConfig
	// Key to decrypt data sent over network.
	AesKey string
}
