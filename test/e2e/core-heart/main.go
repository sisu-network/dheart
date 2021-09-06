package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	tcrypto "github.com/tendermint/tendermint/crypto"

	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/run"
	"github.com/sisu-network/dheart/test/mock"
	"github.com/sisu-network/dheart/utils"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

func getPublicKeys(n int) []tcrypto.PubKey {
	pubKeys := make([]tcrypto.PubKey, n)

	for i := 0; i < n; i++ {
		pubKeys[i] = secp256k1.PrivKey(p2p.GetPrivateKeyBytes(i)).PubKey()
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
	flag.IntVar(&index, "index", 0, "listening port")
	flag.IntVar(&n, "n", 0, "listening port")
	flag.Parse()

	if n == 0 {
		n = 2
	}

	run.LoadConfigEnv("../../../.env")

	// Overwrite some env variables (like schema names) so that it fits into testing multiple local
	// nodes.
	os.Setenv("DB_MIGRATION_PATH", "file://../../../db/migrations/")
	if index != 0 {
		os.Setenv("DB_SCHEMA", fmt.Sprintf("dheart%d", index))
	}
	os.Setenv("USE_ON_MEMORY", "")

	done := make(chan bool)
	mockClient := mock.NewClient(nil, func(workId string) {
		done <- true
	})

	conConfig, privKey := p2p.GetMockConnectionConfig(n, index)
	encryptedKey := getEncrypted(privKey)
	heart := run.GetHeart(conConfig, mockClient)

	err := heart.SisuHandshake(hex.EncodeToString(encryptedKey), "secp256k1")
	if err != nil {
		panic(err)
	}

	heart.Keygen("keygenId", "eth", getPublicKeys(n))

	select {
	case <-time.After(time.Second * 30):
		panic(fmt.Errorf("Time out"))
	case <-done:
		utils.LogVerbose("core-heart Test passed")
	}
}
