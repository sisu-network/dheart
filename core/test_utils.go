package core

import (
	"crypto/rand"

	tcrypto "github.com/tendermint/tendermint/crypto"

	"github.com/libp2p/go-libp2p-core/peer"
	protocol "github.com/libp2p/go-libp2p-protocol"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

// MockConnectionManager implements p2p.ConnectionManager for testing purposes.
type MockConnectionManager struct {
	msgCh chan *p2p.P2PMessage
}

func NewMockConnectionManager(msgCh chan *p2p.P2PMessage) p2p.ConnectionManager {
	return &MockConnectionManager{
		msgCh: msgCh,
	}
}

func (mock *MockConnectionManager) Start(privKeyBytes []byte) error {
	return nil
}

// Sends an array of byte to a particular peer.
func (mock *MockConnectionManager) WriteToStream(pID peer.ID, protocolId protocol.ID, msg []byte) error {
	mock.msgCh <- &p2p.P2PMessage{
		FromPeerId: string(pID),
		Data:       msg,
	}
	return nil
}

func (mock *MockConnectionManager) AddListener(protocol protocol.ID, listener p2p.P2PDataListener) {
	// Do nothing.
}

// ---- /
func generatePartyTestData(n int) ([]tcrypto.PrivKey, []*Node, tss.SortedPartyIDs) {
	nodes := make([]*Node, n)
	keys := make([]tcrypto.PrivKey, n)
	partyIds := make([]*tss.PartyID, n)

	// Generate private key.
	for i := 0; i < n; i++ {
		secret := make([]byte, 32)
		rand.Read(secret)

		var priKey secp256k1.PrivKey
		priKey = secret[:32]
		pubKey := priKey.PubKey()
		keys[i] = priKey

		node := NewNode(pubKey)
		nodes[i] = node
		partyIds[i] = node.PartyId
	}

	return keys, nodes, tss.SortPartyIDs(partyIds)
}
