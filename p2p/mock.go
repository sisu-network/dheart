package p2p

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	maddr "github.com/multiformats/go-multiaddr"
	tcrypto "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

const (
	TEST_PORT_BASE = 1000
)

var (
	KEYS = []string{
		"88fb0ad19ccb62405fdfb52f595fcf631c10dfb51c3b07596301ee522997a4b1",
		"a14d35bbc1592f1452f0bcca948c6cdb1b8abe2b02e5c10ea3408a1306e4abf6",
		"f087caf632d722b0beaa2471a802ebc808a65416d441149b122188857f855d52",
		"a38883c9c13cf91001e6e8b1fb10031d3c52e2ff7a2d717dd977102773e534ff",
	}
)

func GetPriKey(priKeyString string) (tcrypto.PrivKey, error) {
	priHexBytes, err := base64.StdEncoding.DecodeString(priKeyString)
	if err != nil {
		return nil, fmt.Errorf("fail to decode private key: %w", err)
	}
	rawBytes, err := hex.DecodeString(string(priHexBytes))
	if err != nil {
		return nil, fmt.Errorf("fail to hex decode private key: %w", err)
	}
	var priKey secp256k1.PrivKey
	priKey = rawBytes[:32]
	return priKey, nil
}

func GeneratePrivateKeys() tcrypto.PrivKey {
	secret := make([]byte, 32)
	rand.Read(secret)
	hexString := hex.EncodeToString(secret)
	priKeyBytes := base64.StdEncoding.EncodeToString([]byte(hexString))

	priKey, err := GetPriKey(priKeyBytes)
	if err != nil {
		return nil
	}

	return priKey
}

func GetPrivateKey(index int) []byte {
	key, err := hex.DecodeString(KEYS[index])
	if err != nil {
		panic(err)
	}

	return key
}

func GetBootstrapPeers(nodeSize int, myIndex int, peerIds []string) []maddr.Multiaddr {
	peers := []maddr.Multiaddr{}
	for i := 0; i < nodeSize; i++ {
		if i == myIndex {
			continue
		}

		peerPort := 1000 * (i + 1)
		peerId := peerIds[i]

		peer, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", peerPort, peerId))
		if err != nil {
			panic(err)
		}
		peers = append(peers, peer)
	}

	return peers
}

func P2PIDFromKey(prvKey tcrypto.PrivKey) peer.ID {
	p2pPriKey, err := crypto.UnmarshalSecp256k1PrivateKey(prvKey.Bytes())
	if err != nil {
		return ""
	}

	id, err := peer.IDFromPrivateKey(p2pPriKey)
	if err != nil {
		return ""
	}

	return id
}

func GetMockPeers(n int) []string {
	peerIds := make([]string, 0)
	// for i, keyString := range KEYS {
	for i := 0; i < n; i++ {
		keyString := KEYS[i]
		key, err := hex.DecodeString(keyString)
		if err != nil {
			panic(err)
		}

		prvKey := secp256k1.PrivKey(key)
		peerId := P2PIDFromKey(prvKey).String()

		peerIds = append(peerIds, peerId)
	}

	return peerIds
}

func GetMockConnectionConfig(n, index int) (*ConnectionsConfig, []byte) {
	peerIds := GetMockPeers(n)

	privateKey := GetPrivateKey(index)
	peers := make([]maddr.Multiaddr, 0)

	// create peers
	for i := 0; i < n; i++ {
		if i == index {
			continue
		}

		port := TEST_PORT_BASE * (i + 1)
		peerId := peerIds[i]

		peer, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, peerId))
		if err != nil {
			panic(err)
		}
		peers = append(peers, peer)
	}

	return &ConnectionsConfig{
		Port:           TEST_PORT_BASE * (index + 1),
		Rendezvous:     "rendezvous",
		Protocol:       TSSProtocolID,
		BootstrapPeers: peers,
		HostId:         peerIds[index],
	}, privateKey
}
