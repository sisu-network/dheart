package ecdsa

import (
	"sort"
	"sync"

	"github.com/sisu-network/tss-lib/tss"
)

type partyInfo struct {
	partyId *tss.PartyID
	maxJob  int
}

type AvailableParties struct {
	parties map[string]*partyInfo
	lock    *sync.RWMutex
}

func NewAvailableParties() *AvailableParties {
	return &AvailableParties{
		parties: make(map[string]*partyInfo),
		lock:    &sync.RWMutex{},
	}
}

func (ap *AvailableParties) add(p *tss.PartyID, computingPower int) {
	ap.lock.Lock()
	defer ap.lock.Unlock()

	ap.parties[p.Id] = &partyInfo{
		partyId: p,
		maxJob:  computingPower,
	}
}

func (ap *AvailableParties) getLength() int {
	ap.lock.RLock()
	defer ap.lock.RUnlock()

	return len(ap.parties)
}

func (ap *AvailableParties) getParty(pid string) *tss.PartyID {
	ap.lock.RLock()
	defer ap.lock.RUnlock()

	partyInfo := ap.parties[pid]
	if partyInfo == nil {
		return nil
	}

	return partyInfo.partyId
}

func (ap *AvailableParties) hasPartyId(pid string) bool {
	ap.lock.RLock()
	defer ap.lock.RUnlock()

	_, ok := ap.parties[pid]
	return ok
}

func (ap *AvailableParties) getPartyList(n int) []*tss.PartyID {
	ap.lock.RLock()
	defer ap.lock.RUnlock()

	arr := make([]*tss.PartyID, 0, n)
	for _, partyInfo := range ap.parties {
		arr = append(arr, partyInfo.partyId)
		if len(arr) == n {
			break
		}
	}

	return arr
}

func (ap *AvailableParties) getAllPartiesMap() map[string]*tss.PartyID {
	ap.lock.RLock()
	defer ap.lock.RUnlock()

	ret := make(map[string]*tss.PartyID)
	for pid, info := range ap.parties {
		ret[pid] = info.partyId
	}

	return ret
}

// getTopParties returns a list of parties with highest computing power.
func (ap *AvailableParties) getTopParties(count int) ([]*tss.PartyID, int) {
	ap.lock.RLock()
	defer ap.lock.RUnlock()

	arr := make([]*partyInfo, 0, len(ap.parties))
	for _, partyInfo := range ap.parties {
		arr = append(arr, partyInfo)
	}

	// Sort all parties descendingly by their max job.
	sort.SliceStable(arr, func(i, j int) bool {
		return arr[i].maxJob > arr[j].maxJob
	})

	parties := make([]*tss.PartyID, 0)
	for i := 0; i < count && i < len(arr); i++ {
		parties = append(parties, arr[i].partyId)
	}

	return parties, arr[len(parties)-1].maxJob
}
