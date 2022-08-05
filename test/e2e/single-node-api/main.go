package main

import (
	"context"
	"crypto/elliptic"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/sisu-network/lib/log"

	libchain "github.com/sisu-network/lib/chain"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"github.com/sisu-network/dheart/server"
	"github.com/sisu-network/dheart/test/mock"
	"github.com/sisu-network/dheart/types"

	"github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
)

func main() {
	err := godotenv.Load("../../../.env")
	if err != nil {
		panic(err)
	}

	client, err := ethclient.Dial("http://localhost:7545")
	if err != nil {
		panic(err)
	}

	fromAddress := common.HexToAddress("0xbeF23B2AC7857748fEA1f499BE8227c5fD07E70c")
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		panic(err)
	}

	toAddress := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
	value := big.NewInt(100000000000000000) // in wei (0.1eth)
	gasLimit := uint64(21000)
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		panic(err)
	}
	var data []byte

	tx := etypes.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)
	bz, err := tx.MarshalBinary()
	if err != nil {
		panic(err)
	}

	request := &types.KeysignRequest{
		KeyType: libchain.KEY_TYPE_ECDSA,
		KeysignMessages: []*types.KeysignMessage{
			{
				OutChain:    "eth",
				OutHash:     "",
				BytesToSign: bz,
			},
		},
	}

	done := make(chan [][]byte)

	privKeyBytes, err := hex.DecodeString("9f575b88940d452da46a6ceec06a108fcd5863885524aec7fb0bc4906eb63ab1")
	if err != nil {
		panic(err)
	}
	privKey, err := crypto.ToECDSA(privKeyBytes)
	if err != nil {
		panic(err)
	}

	pubKey := privKey.PublicKey
	pubKeyBytes := elliptic.Marshal(pubKey, pubKey.X, pubKey.Y)
	signer := etypes.NewLondonSigner(big.NewInt(1))
	hash := signer.Hash(tx)

	mockSisuClient := &mock.MockClient{
		PostKeysignResultFunc: func(result *types.KeysignResult) error {
			for _, sig := range result.Signatures {
				ok := crypto.VerifySignature(pubKeyBytes, hash[:], sig[:len(sig)-1])
				if !ok {
					panic("signature verification failed")
				}

				log.Info("Signature is correct")
			}

			done <- result.Signatures

			return nil
		},
	}

	api := server.NewSingleNodeApi(mockSisuClient)
	api.Init()
	api.KeySign(request, nil)

	signatures := <-done
	if signatures == nil {
		panic(fmt.Errorf("invalid signature"))
	}

	signedTx, err := tx.WithSignature(signer, signatures[0])
	if err != nil {
		panic(err)
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		panic(err)
	}

	balance, err := client.BalanceAt(context.Background(), toAddress, nil)
	if err != nil {
		panic(err)
	}

	log.Info("balance = ", balance)

	log.Info("Test passed")
}
