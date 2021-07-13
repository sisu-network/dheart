package server

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sisu-network/tuktuk/core"
	"github.com/sisu-network/tuktuk/store"
	"github.com/sisu-network/tuktuk/utils"
)

const (
	encodedAESKey = "Jxq9PhUzvP4RZFBQivXGfA"
)

// This is a mock API to use for single localhost node. It does not have TSS signing round and
// generates a private key instead.
type SingleNodeApi struct {
	tutuk *core.TutTuk

	keyMap map[string]interface{}
	store  *store.Store

	ethKeys map[string]*ecdsa.PrivateKey
}

func NewSingleNodeApi(tuktuk *core.TutTuk) *SingleNodeApi {
	aesKey, err := base64.RawStdEncoding.DecodeString(encodedAESKey)
	if err != nil {
		panic(err)
	}

	path := os.Getenv("HOME_DIR")

	store, err := store.NewStore(path+"/apidb", aesKey)
	if err != nil {
		panic(err)
	}

	return &SingleNodeApi{
		tutuk:   tuktuk,
		keyMap:  make(map[string]interface{}),
		store:   store,
		ethKeys: make(map[string]*ecdsa.PrivateKey),
	}
}

func (api *SingleNodeApi) Version() string {
	return "1"
}

func (api *SingleNodeApi) KeyGen(chain string) error {
	utils.LogInfo("chain = ", chain)
	var err error
	switch chain {
	case "eth":
		err = api.keyGenEth(chain)
	case "eth1":
		err = api.keyGenEth(chain)
	}

	return err
}

func (api *SingleNodeApi) keyGenEth(chain string) error {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return err
	}

	api.ethKeys[chain] = privateKey

	x509Encoded, _ := x509.MarshalECPrivateKey(privateKey)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	return api.store.PutEncrypted([]byte(fmt.Sprintf("key-%s", chain)), []byte(pemEncoded))
}
