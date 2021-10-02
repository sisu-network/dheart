package blame

import (
	"fmt"
	"sync"

	mapset "github.com/deckarep/golang-set"
	"github.com/sisu-network/tss-lib/tss"
)

// Manager keeps track of the nodes that have sent messages in each round to find the culprits.
// It also keeps track of pre_execution culprits in keysign and presign round.
type Manager struct {
	// For each round, check which nodes sent message

	// Key: workID + round, value: set of sent nodes
	sentNodes map[string]map[string]struct{}

	// Key: workID + round, value: set of sent nodes
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

func (m *Manager) AddSender(key, sender string) {
	m.mgrLock.Lock()
	defer m.mgrLock.Unlock()

	if _, ok := m.sentNodes[key]; !ok {
		m.sentNodes[key] = make(map[string]struct{})
	}

	m.sentNodes[key][sender] = struct{}{}
}

func (m *Manager) AddPreExecutionCulprit(culprits []*tss.PartyID) {
	m.mgrLock.Lock()
	defer m.mgrLock.Unlock()

	m.preExecutionCulprits = append(m.preExecutionCulprits, culprits...)
}

func (m *Manager) AddCulpritByRound(key string, culprits []*tss.PartyID) {
	m.mgrLock.Lock()
	defer m.mgrLock.Unlock()

	m.roundCulprits[key] = append(m.roundCulprits[key], culprits...)
}

func (m *Manager) GetRoundCulprits(key string, peers map[string]*tss.PartyID) []*tss.PartyID {
	m.mgrLock.RLock()
	defer m.mgrLock.RUnlock()

	sentNodes := mapset.NewSet()
	for node := range m.sentNodes[key] {
		sentNodes.Add(node)
	}

	allPeers := mapset.NewSet()
	for _, p := range peers {
		allPeers.Add(p.Id)
	}

	// Nodes that have not sent messages.
	diff := allPeers.Difference(sentNodes)

	// Merge with nodes that have sent error messages, and deduplicate.
	for _, pid := range m.roundCulprits[key] {
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

func CreateRoundKey(workID string, round uint32) string {
	return fmt.Sprintf("%s:%d", workID, round)
}
