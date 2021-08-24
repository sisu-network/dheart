package core

import (
	"strconv"

	"github.com/libp2p/go-libp2p-core/peer"
	protocol "github.com/libp2p/go-libp2p-protocol"
	"github.com/sisu-network/dheart/p2p"
	"github.com/sisu-network/tss-lib/tss"
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

func GetTestNodes(partyIds tss.SortedPartyIDs) []*Node {
	nodes := make([]*Node, len(partyIds))

	for i, partyId := range partyIds {
		node := &Node{
			PeerId:  peer.ID("peer" + strconv.Itoa(i)),
			PartyId: partyId,
		}
		nodes[i] = node
	}

	return nodes
}
