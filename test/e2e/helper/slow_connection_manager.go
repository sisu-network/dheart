package helper

import (
	"crypto/rand"
	"encoding/json"
	"math/big"

	p2ptypes "github.com/sisu-network/dheart/p2p/types"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sisu-network/dheart/p2p"
	p2pTypes "github.com/sisu-network/dheart/p2p/types"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/lib/log"
)

type SlowConnectionManager struct {
	cm p2p.ConnectionManager
}

func (scm *SlowConnectionManager) Start(privKeyBytes []byte, keyType string) error {
	return scm.cm.Start(privKeyBytes, keyType)
}

func (scm *SlowConnectionManager) WriteToStream(pID peer.ID, protocolId protocol.ID, msg []byte) error {
	signedMsg := &common.SignedMessage{}
	if err := json.Unmarshal(msg, signedMsg); err != nil {
		panic(err)
	}

	if signedMsg.TssMessage.Type != common.TssMessage_UPDATE_MESSAGES {
		return scm.cm.WriteToStream(pID, protocolId, msg)
	}

	rd, _ := rand.Int(rand.Reader, big.NewInt(100))
	if rd.Int64()%2 == 0 {
		log.Verbose("Drop broadcast message with type ", signedMsg.TssMessage.Type)
		return nil
	}

	return scm.cm.WriteToStream(pID, protocolId, msg)
}

func (scm *SlowConnectionManager) AddListener(protocol protocol.ID, listener p2p.P2PDataListener) {
	scm.cm.AddListener(protocol, listener)
}

func (scm *SlowConnectionManager) IsReady() bool {
	return scm.cm.IsReady()
}

func (scm *SlowConnectionManager) AddPeers([]p2ptypes.Peer) {}

func NewSlowConnectionManager(config p2pTypes.ConnectionsConfig, savedPeers []p2ptypes.Peer) p2p.ConnectionManager {
	return &SlowConnectionManager{
		cm: p2p.NewConnectionManager(config, savedPeers),
	}
}
