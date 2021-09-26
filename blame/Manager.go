package blame

import (
	"sync"

	mapset "github.com/deckarep/golang-set"
	"github.com/sisu-network/tss-lib/tss"
)

type Manager struct {
	// Message cache from round -> sender
	sentMsg map[string]map[string]struct{}

	roundCulprits        map[string][]*tss.PartyID
	preExecutionCulprits []*tss.PartyID

	mgrLock *sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		sentMsg:       make(map[string]map[string]struct{}),
		roundCulprits: make(map[string][]*tss.PartyID),
		mgrLock:       &sync.RWMutex{},
	}
}

func (m *Manager) AddSender(round, sender string) {
	m.mgrLock.Lock()
	defer m.mgrLock.Unlock()

	if _, ok := m.sentMsg[round]; !ok {
		m.sentMsg[round] = make(map[string]struct{})
	}

	m.sentMsg[round][sender] = struct{}{}
}

func (m *Manager) AddPreExecutionCulprit(culprits []*tss.PartyID) {
	m.mgrLock.Lock()
	defer m.mgrLock.Unlock()

	m.preExecutionCulprits = append(m.preExecutionCulprits, culprits...)
}

func (m *Manager) AddCulpritByRound(round string, culprits []*tss.PartyID) {
	m.mgrLock.Lock()
	defer m.mgrLock.Unlock()

	m.roundCulprits[round] = append(m.roundCulprits[round], culprits...)
}

// Return the different between node that sends messages and requests nodes
func (m *Manager) GetRoundCulprits(round string, peers map[string]*tss.PartyID) []*tss.PartyID {
	m.mgrLock.RLock()
	defer m.mgrLock.RUnlock()

	sentNodes := mapset.NewSet()
	for node := range m.sentMsg[round] {
		sentNodes.Add(node)
	}

	allPeers := mapset.NewSet()
	for _, p := range peers {
		allPeers.Add(p)
	}

	diff := allPeers.Difference(sentNodes).ToSlice()
	res := make([]*tss.PartyID, len(diff))
	for i, d := range diff {
		res[i] = peers[d.(string)]
	}

	// Peers not sent messages and peers sent error messages
	// Dedup
	for _, pid := range m.roundCulprits[round] {
		found := false
		for _, r := range res {
			if r.Id == pid.Id {
				found = true
				break
			}
		}

		if !found {
			res = append(res, pid)
		}
	}

	return res
}

func (m *Manager) GetPreExecutionCulprits() []*tss.PartyID {
	m.mgrLock.RLock()
	defer m.mgrLock.RUnlock()

	return m.preExecutionCulprits
}
