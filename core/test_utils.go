package core

import (
	"encoding/hex"
	"math/big"
	"sort"

	ctypes "github.com/sisu-network/cosmos-sdk/crypto/types"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/sisu-network/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/tss"
)

type p2pDataWrapper struct {
	msg *p2p.P2PMessage
	To  string
}

// MockConnectionManager implements p2p.ConnectionManager for testing purposes.
type MockConnectionManager struct {
	msgCh          chan *p2pDataWrapper
	fromPeerString string
}

func NewMockConnectionManager(fromPeerString string, msgCh chan *p2pDataWrapper) p2p.ConnectionManager {
	return &MockConnectionManager{
		msgCh:          msgCh,
		fromPeerString: fromPeerString,
	}
}

func (mock *MockConnectionManager) Start(privKeyBytes []byte, keyType string) error {
	return nil
}

// Sends an array of byte to a particular peer.
func (mock *MockConnectionManager) WriteToStream(toPeerId peer.ID, protocolId protocol.ID, msg []byte) error {
	p2pMsg := &p2p.P2PMessage{
		FromPeerId: mock.fromPeerString,
		Data:       msg,
	}

	mock.msgCh <- &p2pDataWrapper{
		msg: p2pMsg,
		To:  toPeerId.String(),
	}
	return nil
}

func (mock *MockConnectionManager) AddListener(protocol protocol.ID, listener p2p.P2PDataListener) {
	// Do nothing.
}

// ---- /
func getEngineTestData(n int) ([]ctypes.PrivKey, []*Node, tss.SortedPartyIDs, []*keygen.LocalPartySaveData) {
	type dataWrapper struct {
		key    *secp256k1.PrivKey
		pubKey ctypes.PubKey
		node   *Node
	}

	data := make([]*dataWrapper, n)

	// Generate private key.
	for i := 0; i < n; i++ {
		secret, err := hex.DecodeString(helper.PRIVATE_KEY_HEX[i])
		if err != nil {
			panic(err)
		}

		var key *secp256k1.PrivKey
		key = &secp256k1.PrivKey{Key: secret}
		pubKey := key.PubKey()

		node := NewNode(pubKey)

		data[i] = &dataWrapper{
			key, pubKey, node,
		}
	}

	sort.SliceStable(data, func(i, j int) bool {
		key1 := data[i].node.PartyId.KeyInt()
		key2 := data[j].node.PartyId.KeyInt()
		return key1.Cmp(key2) <= 0
	})

	nodes := make([]*Node, n)
	keys := make([]ctypes.PrivKey, n)
	partyIds := make([]*tss.PartyID, n)

	for i := range data {
		keys[i] = data[i].key
		nodes[i] = data[i].node
		partyIds[i] = data[i].node.PartyId
	}

	sorted := tss.SortPartyIDs(partyIds)

	// Verify that sorted id are the same with the one in data array
	for i := range data {
		if data[i].node.PartyId.Id != sorted[i].Id {
			panic("ID does not match")
		}
	}

	return keys, nodes, sorted, helper.LoadKeygenSavedData(sorted)
}

func getPartyIdsFromStrings(pids []string) []*tss.PartyID {
	partyIds := make([]*tss.PartyID, len(pids))
	for i := 0; i < len(pids); i++ {
		partyIds[i] = tss.NewPartyID(pids[i], "", big.NewInt(int64(i+1)))
	}

	return partyIds
}
