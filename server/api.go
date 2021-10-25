package server

import (
	"encoding/hex"
	"os"

	"github.com/sisu-network/dheart/client"
	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/store"
	"github.com/sisu-network/dheart/types"
)

// Api is a common interface for both single and TSS API.
type Api interface {
	Init()
	SetPrivKey(encodedKey string, keyType string) error
	KeyGen(keygenId string, chain string, tPubKeys []types.PubKeyWrapper) error
	KeySign(req *types.KeysignRequest) error
}

func getHeart(cfg config.HeartConfig, client client.Client) *core.Heart {
	// DB Config
	dbConfig := config.DbConfig{
		Port:          cfg.Db.Port,
		Host:          cfg.Db.Host,
		Username:      cfg.Db.Username,
		Password:      cfg.Db.Password,
		Schema:        cfg.Db.Schema,
		MigrationPath: cfg.Db.MigrationPath,
	}

	aesKey, err := hex.DecodeString(os.Getenv("AES_KEY_HEX"))
	if err != nil {
		panic(err)
	}

	// Heart Config
	heartConfig := config.HeartConfig{
		Db:         dbConfig,
		AesKey:     aesKey,
		Connection: cfg.Connection,
	}

	return core.NewHeart(heartConfig, client)
}

func GetApi(cfg config.HeartConfig, store store.Store, client client.Client) Api {
	if cfg.UseOnMemory {
		api := NewSingleNodeApi(client, store)
		api.Init()
		return api
	} else {
		heart := getHeart(cfg, client)
		return NewTssApi(heart)
	}
}
