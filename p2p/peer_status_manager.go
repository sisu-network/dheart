package p2p

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	types "github.com/sisu-network/dheart/p2p/types"
	"github.com/sisu-network/lib/log"
)

var (
	PingFrequency = time.Second * 5
	PingMessage   = "ping"
)

type StatusManager interface {
	Start()
	Stop()
	GetStatus(peerId peer.ID) Status
	UpdatePeerStatus(peerId peer.ID, status Status)
}

type DefaultStatusManager struct {
	peerIds   *sync.Map
	cm        ConnectionManager
	lock      *sync.Mutex
	isStopped bool
}

func NewStatusManager(peerIds []peer.ID, cm ConnectionManager) StatusManager {
	peerIdMap := &sync.Map{}

	for _, peerId := range peerIds {
		peerIdMap.Store(peerId, NewPeerData(peerId))
	}

	return &DefaultStatusManager{
		peerIds: peerIdMap,
		cm:      cm,
		lock:    &sync.Mutex{},
	}
}

func (m *DefaultStatusManager) Start() {
	go func() {
		// Periodically ping other nodes.
		for !m.isStopped {
			m.peerIds.Range(func(key, value interface{}) bool {
				peerId := key.(peer.ID)
				peer := value.(*PeerData)

				if !peer.NeedPing() {
					return true
				}

				err := m.cm.WriteToStream(peerId, PingProtocolID, []byte(PingMessage))
				if err != nil {
					peer.SetStatus(STATUS_DISCONNECTED)
				} else {
					peer.SetStatus(STATUS_CONNECTED)
				}

				return true
			})

			time.Sleep(PingFrequency)
		}
	}()
}

func (m *DefaultStatusManager) GetStatus(peerId peer.ID) Status {
	value, ok := m.peerIds.Load(peerId)
	if !ok {
		return STATUS_UNKNOWN
	}

	peer := value.(*PeerData)
	return peer.GetStatus()
}

func (m *DefaultStatusManager) UpdatePeerStatus(peerId peer.ID, status Status) {
	value, ok := m.peerIds.Load(peerId)
	if !ok {
		return
	}

	peer := value.(*PeerData)
	peer.SetStatus(status)
}

func (m *DefaultStatusManager) Stop() {
	m.lock.Lock()
	m.isStopped = true
	m.lock.Unlock()
}

func (m *DefaultStatusManager) OnNetworkMessage(message *types.P2PMessage) {
	peerIdString := message.FromPeerId
	peerId, err := peer.IDFromString(peerIdString)
	if err != nil {
		log.Error("Cannot parse peer id string", peerIdString, "err =", err)
		return
	}

	// Update peer status
	value, ok := m.peerIds.Load(peerId)
	var peer *PeerData
	if !ok {
		peer = NewPeerData(peerId)
	} else {
		peer = value.(*PeerData)
	}
	peer.SetStatus(STATUS_CONNECTED)
}
