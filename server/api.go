package server

import (
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
	KeySign(req *types.KeysignRequest, tPubKeys []types.PubKeyWrapper) error
	BlockEnd(blockHeight int64) error
	SetSisuReady(isReady bool)
}

func GetApi(cfg config.HeartConfig, store store.Store, client client.Client) Api {
	if cfg.UseOnMemory {
		api := NewSingleNodeApi(client, store)
		api.Init()
		return api
	} else {
		heart := core.NewHeart(cfg, client)
		return NewTssApi(heart)
	}
}
