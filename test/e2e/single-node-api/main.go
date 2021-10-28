package main

import (
	"context"
	"crypto/elliptic"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"github.com/sisu-network/dheart/server"
	"github.com/sisu-network/dheart/test/mock"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"

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
		log.Fatal(err)
	}

	toAddress := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
	value := big.NewInt(100000000000000000) // in wei (0.1eth)
	gasLimit := uint64(21000)
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	var data []byte

	tx := etypes.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)
	bz, err := tx.MarshalBinary()
	if err != nil {
		panic(err)
	}

	request := &types.KeysignRequest{
		OutChain: "eth",
		OutHash:  "",
		OutBytes: bz,
	}

	done := make(chan []byte)

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
	signer := etypes.NewEIP2930Signer(big.NewInt(1))
	hash := signer.Hash(tx)

	mockSisuClient := &mock.MockClient{
		PostKeysignResultFunc: func(result *types.KeysignResult) error {
			ok := crypto.VerifySignature(pubKeyBytes, hash[:], result.Signature[:len(result.Signature)-1])
			if !ok {
				panic("signature verification failed")
			}

			utils.LogInfo("Signature is correct")
			done <- result.Signature

			return nil
		},
	}

	mockStore := &mock.MockStore{
		GetEncryptedFunc: func(key []byte) (value []byte, err error) {
			// private key of mnemonic "draft attract behave allow rib raise puzzle frost neck curtain gentle bless letter parrot hold century diet budget paper fetch hat vanish wonder maximum"
			encodedPrivateKey := "9f575b88940d452da46a6ceec06a108fcd5863885524aec7fb0bc4906eb63ab1"
			return hex.DecodeString(encodedPrivateKey)
		},
	}

	api := server.NewSingleNodeApi(mockSisuClient, mockStore)
	api.Init()
	api.KeySign(request, nil)

	signature := <-done
	if signature == nil {
		panic(fmt.Errorf("invalid signature"))
	}

	signedTx, err := tx.WithSignature(signer, signature)
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

	utils.LogInfo("balance = ", balance)

	utils.LogInfo("Test passed")
}
