package core

import (
	"strings"
	"sync"

	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

type AvailPresignManager struct {
	db db.Database
	// Group all available presign by its list of pids.
	// map between: list of pids (string) -> array of available presigns.
	available map[string][]*common.AvailablePresign

	// Set of presign data that being used by a worker. In case the worker fails, this list of presigns
	// are added back to the available pool.
	// map between: list of pids (string) -> array of available presigns.
	inUse map[string][]*common.AvailablePresign
	lock  *sync.RWMutex
}

func NewAvailPresignManager(db db.Database) *AvailPresignManager {
	return &AvailPresignManager{
		db:        db,
		available: make(map[string][]*common.AvailablePresign),
		inUse:     make(map[string][]*common.AvailablePresign),
		lock:      &sync.RWMutex{},
	}
}

func (m *AvailPresignManager) Load() error {
	pidStrings, workIds, indexes, err := m.db.GetAvailablePresignShortForm()
	if err != nil {
		return err
	}

	m.lock.Lock()
	for i, pidString := range pidStrings {
		arr := m.available[pidString]
		if arr == nil {
			arr = make([]*common.AvailablePresign, 0)
		}

		ap := &common.AvailablePresign{
			PidsString: pidString,
			Pids:       strings.Split(pidString, ","),
			WorkId:     workIds[i],
			BatchIndex: indexes[i],
		}
		arr = append(arr, ap)
		m.available[pidString] = arr
	}
	m.lock.Unlock()

	return nil
}

func (m *AvailPresignManager) GetAvailablePresigns(batchSize int, n int, pids []*tss.PartyID) ([]*presign.LocalPresignData, []*tss.PartyID) {
	selectedPidstring := ""
	var aps []*common.AvailablePresign

	m.lock.Lock()
	for pidString, arr := range m.available {
		if len(arr) >= batchSize && len(arr[0].Pids) == n {
			// We found this available pid string.
			selectedPidstring = pidString
			aps = arr[:batchSize]

			// Remove this available presigns from the list.
			if batchSize < len(arr)-1 {
				m.available[pidString] = arr[batchSize+1:]
			} else {
				m.available[pidString] = make([]*common.AvailablePresign, 0)
			}
			if len(m.available[pidString]) == 0 {
				delete(m.available, pidString)
			}

			// Update the inUse list
			inUse := m.inUse[selectedPidstring]
			if inUse == nil {
				m.inUse[selectedPidstring] = make([]*common.AvailablePresign, 0)
			}
			inUse = append(inUse, aps...)
			m.inUse[selectedPidstring] = inUse
			break
		}
	}
	m.lock.Unlock()

	if selectedPidstring == "" {
		return make([]*presign.LocalPresignData, 0), make([]*tss.PartyID, 0)
	}

	presigns := m.loadPresignData(aps)

	// Get selected pids
	pidStrings := strings.Split(selectedPidstring, ",")
	selectedPids := make([]*tss.PartyID, len(selectedPidstring))

	for i, pidString := range pidStrings {
		for _, p := range pids {
			if p.Id == pidString {
				selectedPids[i] = p
				break
			}
		}
	}

	return presigns, selectedPids
}

func (m *AvailPresignManager) loadPresignData(aps []*common.AvailablePresign) []*presign.LocalPresignData {
	batchIndexMap := m.groupBatchIndexByWorkId(aps)

	result := make([]*presign.LocalPresignData, 0)
	for workId, batchIndexes := range batchIndexMap {
		loaded, err := m.db.LoadPresign(workId, batchIndexes)
		if err != nil {
			utils.LogError("Cannot load presign for workId", workId)
			continue
		}

		result = append(result, loaded...)
	}
	return result
}

// groupBatchIndexByWorkId iterates through a list of presigns and group them by workid.
func (m *AvailPresignManager) groupBatchIndexByWorkId(aps []*common.AvailablePresign) map[string][]int {
	batchIndexMap := make(map[string][]int)
	for i, ap := range aps {
		if batchIndexMap[ap.WorkId] != nil {
			continue
		}

		arr := make([]int, 0)
		for j := i; j < len(aps); j++ {
			if aps[j].WorkId == ap.WorkId {
				arr = append(arr, aps[j].BatchIndex)
			}
		}
		batchIndexMap[ap.WorkId] = arr
	}

	return batchIndexMap
}

func (m *AvailPresignManager) updateUsage(pidsString string, aps []*common.AvailablePresign, used bool) {
	m.lock.Lock()

	// 1. This presign set has been used, remove them from presignsInUse set.
	arr := m.inUse[pidsString]
	if arr == nil {
		utils.LogError("Cannot find presign in use set with pids", pidsString)
		m.lock.Unlock()
		return
	}

	newInUse := make([]*common.AvailablePresign, 0)
	for _, inUse := range arr {
		found := false
		for _, ap := range aps {
			if inUse.WorkId == ap.WorkId && inUse.BatchIndex == ap.BatchIndex {
				found = true
				break
			}
		}
		if !found {
			newInUse = append(newInUse, inUse)
		}
	}

	if len(newInUse) == 0 {
		delete(m.inUse, pidsString)
	} else {
		m.inUse[pidsString] = newInUse
	}

	if !used {
		// This presign set is not used. Move all of its element back to the available pool.
		arr := m.available[pidsString]
		if arr == nil {
			arr = make([]*common.AvailablePresign, 0)
		}
		arr = append(aps, arr...)
		m.available[pidsString] = arr
	}
	m.lock.Unlock()

	if used {
		batchIndexMap := m.groupBatchIndexByWorkId(aps)

		for workId, batchIndexes := range batchIndexMap {
			err := m.db.UpdatePresignStatus(workId, batchIndexes)
			if err != nil {
				utils.LogError("Failed to update presign staus with work id", workId)
			}
		}
	}
}
