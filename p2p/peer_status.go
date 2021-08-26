package p2p

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
)

type Status int

const (
	STATUS_UNKNOWN Status = iota
	STATUS_CONNECTED
	STATUS_DISCONNECTED
)

type PeerData struct {
	peerId       peer.ID
	status       Status
	lastPingTime time.Time
	statusLock   *sync.RWMutex
}

func NewPeerData(peerId peer.ID) *PeerData {
	return &PeerData{
		status:       STATUS_UNKNOWN,
		statusLock:   &sync.RWMutex{},
		lastPingTime: time.Unix(0, 0),
	}
}

func (p *PeerData) SetStatus(status Status) {
	p.statusLock.Lock()
	defer p.statusLock.Unlock()

	p.status = status
	if status == STATUS_CONNECTED || status == STATUS_DISCONNECTED {
		p.lastPingTime = time.Now()
	}
}

func (p *PeerData) GetStatus() Status {
	p.statusLock.RLock()
	defer p.statusLock.RUnlock()

	return p.status
}

func (p *PeerData) NeedPing() bool {
	p.statusLock.Lock()
	defer p.statusLock.Unlock()

	tmp := p.lastPingTime.Add(PING_FREQUENCY)
	if tmp.Before(time.Now()) {
		return true
	}

	return false
}
