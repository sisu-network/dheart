package core

import (
	"fmt"
	"testing"

	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/stretchr/testify/assert"
)

func getMokDbForAvailManager(presignPids, pids []string) db.Database {
	return &helper.MockDatabase{
		GetAvailablePresignShortFormFunc: func() ([]string, []string, error) {
			return presignPids, pids, nil
		},

		LoadPresignFunc: func(presignIds []string) ([]*presign.LocalPresignData, error) {
			return make([]*presign.LocalPresignData, len(presignIds)), nil
		},
	}
}

func TestAvailManagerHappyCase(t *testing.T) {
	selectedPids := "2,3,5"

	presignPids := []string{"work0-0", "work0-1", "work1-0", "work1-1", "work1-2", "work2-0"}
	pids := []string{"1,2,4", "1,2,4", "2,3,5", "2,3,5", "2,3,5", "3,4,5"}

	mockDb := getMokDbForAvailManager(presignPids, pids)

	allPids := []string{"2", "3", "4", "5", "6", "7"}
	partyIds := getPartyIdsFromStrings(allPids)

	availManager := NewAvailPresignManager(mockDb)
	availManager.Load()
	assert.Equal(t, 3, len(availManager.available))

	// Get and consumes 3 presigns
	presignIds, _ := availManager.GetAvailablePresigns(3, 3, partyIds)
	assert.Equal(t, 3, len(presignIds))

	// We should have 1 pid string in use (2,3,5) and 2 available pid string: (1,2,3) and (3,4,5)
	assert.Equal(t, 1, len(availManager.inUse))
	assert.Equal(t, 2, len(availManager.available))

	// Update status
	selectedAps := make([]*common.AvailablePresign, 3)
	for i := 0; i < len(selectedAps); i++ {
		selectedAps[i] = &common.AvailablePresign{
			PresignId:  fmt.Sprintf("%s-%d", "work1", i),
			PidsString: selectedPids,
		}
	}

	availManager.updateUsage(selectedPids, selectedAps, true)
	assert.Equal(t, 0, len(availManager.inUse))
	assert.Equal(t, 2, len(availManager.available))
}

func TestAvailManagerNotFound(t *testing.T) {
	presignPids := []string{"work0-0", "work0-1", "work1-0", "work1-1", "work2-0"}
	pids := []string{"1,2,4", "1,2,4", "2,3,5", "2,3,5", "3,4,5"}

	mockDb := getMokDbForAvailManager(presignPids, pids)

	appPids := []string{"2", "3", "4", "5", "6", "7"}
	partyIds := getPartyIdsFromStrings(appPids)

	availManager := NewAvailPresignManager(mockDb)
	availManager.Load()
	assert.Equal(t, 3, len(availManager.available))

	presignIds, _ := availManager.GetAvailablePresigns(3, 3, partyIds)
	assert.Equal(t, 0, len(presignIds))

	assert.Equal(t, 0, len(availManager.inUse))
	assert.Equal(t, 3, len(availManager.available))
}

func TestAvailManagerPresignNotUsed(t *testing.T) {
	selectedPids := "2,3,5"

	presignPids := []string{"work0-0", "work0-1", "work1-0", "work1-1", "work1-2", "work2-0"}
	pids := []string{"1,2,4", "1,2,4", "2,3,5", "2,3,5", "2,3,5", "3,4,5"}
	mockDb := getMokDbForAvailManager(presignPids, pids)
	partyIds := getPartyIdsFromStrings([]string{"2", "3", "4", "5", "6", "7"})

	availManager := NewAvailPresignManager(mockDb)
	availManager.Load()

	presignIds, _ := availManager.GetAvailablePresigns(3, 3, partyIds)
	assert.Equal(t, 3, len(presignIds))

	// Update status
	selectedAps := make([]*common.AvailablePresign, 3)
	for i := 0; i < len(selectedAps); i++ {
		selectedAps[i] = &common.AvailablePresign{
			PresignId:  fmt.Sprintf("%s-%d", "work1", i),
			PidsString: selectedPids,
		}
	}

	availManager.updateUsage(selectedPids, selectedAps, false)
	assert.Equal(t, 0, len(availManager.inUse))
	assert.Equal(t, 3, len(availManager.available))
}
