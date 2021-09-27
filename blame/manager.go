package blame

import (
	"sync"

	mapset "github.com/deckarep/golang-set"
	"github.com/sisu-network/tss-lib/tss"
)

type Manager struct {
	// For each round, check which nodes sent message
	sentNodes map[string]map[string]struct{}

	roundCulprits        map[string][]*tss.PartyID
	preExecutionCulprits []*tss.PartyID

	mgrLock *sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		sentNodes:     make(map[string]map[string]struct{}),
		roundCulprits: make(map[string][]*tss.PartyID),
		mgrLock:       &sync.RWMutex{},
	}
}

func (m *Manager) AddSender(round, sender string) {
	m.mgrLock.Lock()
	defer m.mgrLock.Unlock()

	if _, ok := m.sentNodes[round]; !ok {
		m.sentNodes[round] = make(map[string]struct{})
	}

	m.sentNodes[round][sender] = struct{}{}
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

func (m *Manager) GetRoundCulprits(round string, peers map[string]*tss.PartyID) []*tss.PartyID {
	m.mgrLock.RLock()
	defer m.mgrLock.RUnlock()

	sentNodes := mapset.NewSet()
	for node := range m.sentNodes[round] {
		sentNodes.Add(node)
	}

	allPeers := mapset.NewSet()
	for _, p := range peers {
		allPeers.Add(p.Id)
	}

	// Nodes that have not sent messages.
	diff := allPeers.Difference(sentNodes)

	// Merge with nodes that have sent error messages, and deduplicate.
	for _, pid := range m.roundCulprits[round] {
		diff.Add(pid.Id)
	}

	culprits := make([]*tss.PartyID, 0, diff.Cardinality())
	for _, p := range diff.ToSlice() {
		if v, ok := peers[p.(string)]; ok {
			culprits = append(culprits, v)
		}
	}

	return culprits
}

func (m *Manager) GetPreExecutionCulprits() []*tss.PartyID {
	m.mgrLock.RLock()
	defer m.mgrLock.RUnlock()

	return m.preExecutionCulprits
}
