package p2p

import (
	"encoding/hex"
	"fmt"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	maddr "github.com/multiformats/go-multiaddr"
	"github.com/sisu-network/cosmos-sdk/crypto/keys/secp256k1"
	ctypes "github.com/sisu-network/cosmos-sdk/crypto/types"
)

const (
	TEST_PORT_BASE = 1000
)

var (
	KEYS = []string{
		"72d46bf8140c95301a476b4c99f443c183b20442eaee0c76c8f6432dcdb78dcd",
		"b73455295e07b38072ac64a85c6057550f09f5e1f2e707feaa5790f719ad5084",
		"49b7f860c4c036ccb504b799ae4665aa91fdbe7348dfe7952e8d58329662213e",
		"b716f8e94585d38e9b0d5f5da47c5ebaa93b9869e14169844668b48d89079a15",
		"80edb8c16026ab94dfe847931f8e72d38eed925cafba27a94021b825f5ee7f5a",
		"f93e1217a26f35ff8eb0cbab2dd42a5db60f4014717c291b961bad37a15357de",
		"1dcc68d2b30e0f56fad4cdb510c63d72811f1d86e58cd4186be3954e6dc24d04",
		"9cdb97a75d437729811ffad23c563a4040f75073f25a4feb9251d25379ca93ae",
		"e26f881c0bf24b8bf882eb44a89fd3658871a96cd53a7f5c8e73e8773be6db5e",
		"f83a08c326ac2fcda36e4ec6f2de3a4c3a3a5155207aea1d1154d3c0c84bf851",
		"901d9358cfe84d7583f5599899e3ca8e31ccfa638a8037fa4fc7e1dfce25e451",
		"e461cf23ea4bbbdb08b2d403581a7fcd0160f87589d49a2fffe59b201c5988ea",
		"887b5ec477a9b884331feca27ec4f1d747193e9de66e5eabcf622bebb2784a41",
		"0a8788750c9b629a36c4f5c407f8155fcc4830c13052cb09c951c771cc252072",
		"610283d69092296926deef185d7a0d8866c1399b247883d68c54a26b4c15b53b",
	}
)

func GeneratePrivateKey() ctypes.PrivKey {
	// secret := make([]byte, 32)
	// rand.Read(secret)

	// var priKey secp256k1.PrivKey
	// priKey = secret[:32]
	return secp256k1.GenPrivKey()
}

func GetAllPrivateKeys(n int) []ctypes.PrivKey {
	keys := make([]ctypes.PrivKey, 0)
	for i := 0; i < n; i++ {
		bz := GetPrivateKeyBytes(i)
		keys = append(keys, &secp256k1.PrivKey{Key: bz})
	}

	return keys
}

func GetPrivateKeyBytes(index int) []byte {
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

func P2PIDFromKey(prvKey ctypes.PrivKey) peer.ID {
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

func GetMockPeers(n int) []peer.ID {
	peerIds := make([]peer.ID, 0)
	for i := 0; i < n; i++ {
		keyString := KEYS[i]
		bz, err := hex.DecodeString(keyString)
		if err != nil {
			panic(err)
		}

		prvKey := &secp256k1.PrivKey{Key: bz}
		peerId := P2PIDFromKey(prvKey)

		peerIds = append(peerIds, peerId)
	}

	return peerIds
}

func GetMockConnectionConfig(n, index int) (ConnectionsConfig, []byte) {
	peerIds := GetMockPeers(n)

	privateKey := GetPrivateKeyBytes(index)
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

	return ConnectionsConfig{
		Port:           TEST_PORT_BASE * (index + 1),
		Rendezvous:     "rendezvous",
		Protocol:       TSSProtocolID,
		BootstrapPeers: peers,
		HostId:         peerIds[index].String(),
	}, privateKey
}
