package core

import (
	"strings"
	"sync"

	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/utils"
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

func (m *AvailPresignManager) GetUnavailablePresigns(sentNodes map[string]*tss.PartyID, allPids []*tss.PartyID) []*tss.PartyID {
	// Nodes that are chosen.
	pids := make([]string, 0, len(m.inUse))
	for _, v := range m.inUse {
		if len(v) > 0 {
			pids = append(pids, v[0].Pids...)
		}
	}

	// Nodes that are chosen but don't send messages.
	missingIDs := make(map[string]struct{}, 0)
	for _, pid := range pids {
		if _, found := sentNodes[pid]; !found {
			missingIDs[pid] = struct{}{}
		}
	}

	missing := make([]*tss.PartyID, 0, len(missingIDs))
	for id := range missingIDs {
		for _, pid := range allPids {
			if pid.Id == id {
				missing = append(missing, pid)
			}
		}
	}

	return missing
}

func (m *AvailPresignManager) Load() error {
	presignIds, pidStrings, err := m.db.GetAvailablePresignShortForm()
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
			PresignId:  presignIds[i],
			PidsString: pidString,
			Pids:       strings.Split(pidString, ","),
		}
		arr = append(arr, ap)
		m.available[pidString] = arr
	}
	m.lock.Unlock()

	return nil
}

func (m *AvailPresignManager) GetAvailablePresigns(batchSize int, n int, pids []*tss.PartyID) ([]string, []*tss.PartyID) {
	selectedPidstring := ""
	var selectedAps []*common.AvailablePresign

	m.lock.Lock()
	for pidString, apArr := range m.available {
		if len(apArr) >= batchSize && len(apArr[0].Pids) == n {
			// We found this available pid string.
			selectedPidstring = pidString
			selectedAps = apArr[:batchSize]

			// Remove this available presigns from the list.
			if batchSize < len(apArr)-1 {
				m.available[pidString] = apArr[batchSize+1:]
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
			inUse = append(inUse, selectedAps...)
			m.inUse[selectedPidstring] = inUse
			break
		}
	}
	m.lock.Unlock()

	if selectedPidstring == "" {
		return []string{}, []*tss.PartyID{}
	}

	// Get selected pids
	presignIds := make([]string, len(selectedAps))
	for i, ap := range selectedAps {
		presignIds[i] = ap.PresignId
	}

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

	return presignIds, selectedPids
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
			if inUse.PresignId == ap.PresignId {
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
		presignIds := make([]string, len(aps))
		for i, ap := range aps {
			presignIds[i] = ap.PresignId
		}

		err := m.db.UpdatePresignStatus(presignIds)
		if err != nil {
			utils.LogError("Failed to update presign staus, err =", err)
		}
	}
}
