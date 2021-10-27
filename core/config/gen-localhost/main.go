package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/sisu-network/dheart/core/config"
)

func genLocalhostConfig() {
	cfg := config.HeartConfig{}

	cfg.HomeDir = filepath.Join(os.Getenv("HOME"), ".sisu/dheart")
	cfg.UseOnMemory = true
	cfg.Port = 28300
	cfg.SisuServerUrl = "http://0.0.0.0:25456"

	cfg.Db = config.DbConfig{
		Host:          "0.0.0.0",
		Port:          3306,
		Username:      "root",
		Password:      "password",
		Schema:        "dheart",
		MigrationPath: "file://db/migrations/",
	}

	configFilePath := filepath.Join(cfg.HomeDir, "./dheart.toml")
	config.WriteConfigFile(configFilePath, cfg)
}

func genEnv() {
	homeDir := filepath.Join(os.Getenv("HOME"), ".sisu/dheart")
	content := fmt.Sprintf(`HOME_DIR=%s
AES_KEY_HEX=c787ef22ade5afc8a5e22041c17869e7e4714190d88ecec0a84e241c9431add0
`, homeDir)

	ioutil.WriteFile(".env", []byte(content), 0600)
}

func main() {
	genLocalhostConfig()
	genEnv()
}
