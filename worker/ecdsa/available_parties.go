package ecdsa

import (
	"sync"

	"github.com/sisu-network/tss-lib/tss"
)

type AvailableParties struct {
	parties map[string]*tss.PartyID
	lock    *sync.RWMutex
}

func NewAvailableParties() *AvailableParties {
	return &AvailableParties{
		parties: make(map[string]*tss.PartyID),
		lock:    &sync.RWMutex{},
	}
}

func (ap *AvailableParties) add(p *tss.PartyID) {
	ap.lock.Lock()
	defer ap.lock.Unlock()

	ap.parties[p.Id] = p
}

func (ap *AvailableParties) getLength() int {
	ap.lock.RLock()
	defer ap.lock.RUnlock()

	return len(ap.parties)
}

func (ap *AvailableParties) getParty(pid string) *tss.PartyID {
	ap.lock.RLock()
	defer ap.lock.RUnlock()

	return ap.parties[pid]
}

func (ap *AvailableParties) getPartyList(n int) []*tss.PartyID {
	ap.lock.RLock()
	defer ap.lock.RUnlock()

	arr := make([]*tss.PartyID, 0, n)
	for _, party := range ap.parties {
		arr = append(arr, party)
		if len(arr) == n {
			break
		}
	}

	return arr
}
