package p2p

import (
	"encoding/hex"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	ctypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	maddr "github.com/multiformats/go-multiaddr"
	p2ptypes "github.com/sisu-network/dheart/p2p/types"
)

const (
	TEST_PORT_BASE = 1000
)

var (
	KEYS_Secp256k1 = []string{
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

	KEYS_Ed25519 = []string{
		"a0727212aa363641c7845fe6544ca61b7e03e85f76d960f4cc624563a1ae170830a84ac6ed8306d5d5160c763cd90a0450eff4f77e3bc1f0fd2cff9abdca0d5f",
		"e9564c22deca51783d72ca31bd6908488d16b34853a6db76ebf6771e3b016d7e4326a4ff4af5775eca238eb249d098672d3edd36b77481bf103d04173091870c",
		"67fafd62981c81d396c150fabeaed9df574194b6be860dad769d4ccd7f5f18a2fccc753c73f98768d5040d62b5e382e4b65ab26744790a2a903036e72cb1304e",
		"625d6104547a4542efcb57aa87086efa84b758cd831ad7af5c8a0ff9cb0df3e4448b2f7df6eb679ecb7dc3f3f78dae2acb3899fcd49e23d71f2575f1f6c53b32",
		"4f2e176a7132c9aec530b896ec1caa31ada8b49fa05a612e42a8fa34dad6d72fa58d5b5dfc5121913e63f7cd234677c4946b5ac9042373e59a43102b598c7805",
		"a42617ef216ebbd0b0c28039d7018d35473e1e8816bef0e2bbb195e8f41a646a96474a58e40b648e839814642592efa8695742b6f207af4299c876cc38502ea3",
		"ff4a2bbca55318db992ec7a970691801ebcb1d75ca37eeb9447436c54118521cc13a450021914aa9ead6f9c7212e599339262bb08427b68067191209f6d45eb5",
		"6021366850949b59961055913030a978c564cfd0e44fe8b3116f9e6a20cd388febed2b0c5b94c0d3435948a7b90866500f7fad90ea8f7835ff61b36f35064d8b",
		"d68a3870d57a62b3bbafa571c8ab7173abfb9923fcb8edc504025609ee55567bac2bac2f95b8090620d421e100ef0424954951bbef61e128a40e141530812c8e",
		"b918e9bd13722471e9fec2b089a6829ff1a3e8b4e06ef5bd1e310f73ad7d06e1b346375e85fb9211640c7ac499a3c403443d9a624b2fb8d69abbb8de9f885e14",
		"0470ef9b389537900412ac99fdaef5b6d243556526be23766d2684370d37292421e2f06635b014a0a5f776b3c058fd935d3eb3f19733f6e0a3f6a8ca2f123d7d",
		"9537a42242132b3564db697fff9b465e557959db54c6419fdf896298b1f690f56ff9fcfd3df27f103c58ea2e8465ba653984f760384afed29806a861b305c2fd",
		"45733afd96c48d2b1fd1e3ab2d7d0299ce3f7baba878cb14add60ad8e5d82ded64f95d6935840fee86b5a694faf3ebb56178b7c189fdb67bf2ad771b16eb9791",
		"84520916fdceca61ac250db4a47da27f7e73e63640254cfe3f811a6e6840ffd965334a983824cb30dca069f37b31d9480e588add2f3d436838c3bbda80ae1e58",
		"5139b75a486be6c5778977772b6424464fb568880e1664d63b5a1000e0be8ac3af66b599ce2d7cfd7f2261c9de93f7665fb4721b932c9c1546ee15e636400fa7",
	}
)

func GeneratePrivateKey(keyType string) ctypes.PrivKey {
	switch keyType {
	case "secp256k1":
		return secp256k1.GenPrivKey()
	case "ed25519":
		return ed25519.GenPrivKey()
	}

	return nil
}

func GetAllSecp256k1PrivateKeys(n int) []ctypes.PrivKey {
	keys := make([]ctypes.PrivKey, 0)
	for i := 0; i < n; i++ {
		bz := GetPrivateKeyBytes(i, "secp256k1")
		keys = append(keys, &secp256k1.PrivKey{Key: bz})
	}

	return keys
}

func GetPrivateKeyBytes(index int, keyType string) []byte {
	var key []byte
	var err error

	switch keyType {
	case "secp256k1":
		key, err = hex.DecodeString(KEYS_Secp256k1[index])
	case "ed25519":
		key, err = hex.DecodeString(KEYS_Ed25519[index])
	}

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

func GetMockPeers(n int, keyType string) []peer.ID {
	peerIds := make([]peer.ID, 0)
	for i := 0; i < n; i++ {
		var keyString string
		switch keyType {
		case "secp256k1":
			keyString = KEYS_Secp256k1[i]
		case "ed25519":
			keyString = KEYS_Ed25519[i]
		}

		bz, err := hex.DecodeString(keyString)
		if err != nil {
			panic(err)
		}

		var prvKey ctypes.PrivKey
		switch keyType {
		case "secp256k1":
			prvKey = &secp256k1.PrivKey{Key: bz}
		case "ed25519":
			prvKey = &ed25519.PrivKey{Key: bz}
		}

		peerId := P2PIDFromKey(prvKey)
		peerIds = append(peerIds, peerId)
	}

	return peerIds
}

func GetMockEd25519Peers(n int) []peer.ID {
	peerIds := make([]peer.ID, 0)
	for i := 0; i < n; i++ {

		keyString := KEYS_Secp256k1[i]
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

func GetMockSecp256k1Config(n, index int) (p2ptypes.ConnectionsConfig, []byte) {
	return GetMockConnectionConfig(n, index, "secp256k1")
}

func GetMockConnectionConfig(n, index int, keyType string) (p2ptypes.ConnectionsConfig, []byte) {
	peerIds := GetMockPeers(n, keyType)

	privateKey := GetPrivateKeyBytes(index, keyType)
	peers := make([]*p2ptypes.Peer, 0)

	// create peers
	for i := 0; i < n; i++ {
		if i == index {
			continue
		}

		port := TEST_PORT_BASE * (i + 1)
		peerId := peerIds[i]

		peer := &p2ptypes.Peer{
			Address: fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, peerId),
		}

		peers = append(peers, peer)
	}

	return p2ptypes.ConnectionsConfig{
		Host:           "0.0.0.0",
		Port:           TEST_PORT_BASE * (index + 1),
		Rendezvous:     "rendezvous",
		Protocol:       TSSProtocolID,
		BootstrapPeers: peers,
	}, privateKey
}
