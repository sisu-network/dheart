package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/sisu-network/lib/log"

	"github.com/sisu-network/dheart/core"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/run"
	"github.com/sisu-network/dheart/test/e2e/helper"
	"github.com/sisu-network/dheart/test/mock"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
)

func getPublicKeys(n int) []ctypes.PubKey {
	pubKeys := make([]ctypes.PubKey, n)

	for i := 0; i < n; i++ {
		privKey := &secp256k1.PrivKey{Key: p2p.GetPrivateKeyBytes(i, "secp256k1")}
		pubKeys[i] = privKey.PubKey()
	}

	return pubKeys
}

func getEncrypted(privKey []byte) []byte {
	aesKey, err := hex.DecodeString(os.Getenv("AES_KEY_HEX"))
	if err != nil {
		panic(err)
	}

	// Encrypt with AES key
	encrypted, err := utils.AESDEncrypt(privKey, aesKey)
	if err != nil {
		panic(err)
	}

	return encrypted
}

func main() {
	var index, n int
	flag.IntVar(&index, "index", 0, "node index")
	flag.IntVar(&n, "n", 0, "Total nodes")
	flag.Parse()

	if n == 0 {
		n = 2
	}

	helper.ResetDb(index)

	run.LoadConfigEnv("/Users/sonltcoder/go/src/github.com/sisu-network/dheart/.env")

	done := make(chan bool)
	mockClient := &mock.MockClient{
		PostKeygenResultFunc: func(result *types.KeygenResult) error {
			done <- true
			return nil
		},
	}

	dbConfig := config.GetLocalhostDbConfig()
	dbConfig.Schema = fmt.Sprintf("dheart%d", index)

	cfg := config.HeartConfig{
		UseOnMemory: false,
		Db:          dbConfig,
	}

	conConfig, privKey := p2p.GetMockSecp256k1Config(n, index)
	cfg.Connection = conConfig

	encryptedKey := getEncrypted(privKey)

	aesKey, err := hex.DecodeString(os.Getenv("AES_KEY_HEX"))
	if err != nil {
		panic(err)
	}

	heartConfig := config.HeartConfig{
		Db:         dbConfig,
		AesKey:     aesKey,
		Connection: cfg.Connection,
	}
	pubkeys := getPublicKeys(n)

	heart := core.NewHeart(heartConfig, mockClient)
	heart.SetBootstrappedKeys(pubkeys)

	err = heart.SetPrivKey(hex.EncodeToString(encryptedKey), "secp256k1")
	if err != nil {
		panic(err)
	}

	heart.Keygen("keygenId", "ecdsa", pubkeys)

	select {
	case <-time.After(time.Second * 30):
		panic("Time out")
	case <-done:
		log.Verbose("core-heart Test passed")
	}

	// os.Exit(123)
}
