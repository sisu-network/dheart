package components

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
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

func TestAvailPresignManager_HappyCase(t *testing.T) {
	t.Parallel()

	selectedPids := "2,3,5"

	presignPids := []string{"work0-0", "work0-1", "work1-0", "work1-1", "work1-2", "work2-0"}
	pids := []string{"1,2,4", "1,2,4", "2,3,5", "2,3,5", "2,3,5", "3,4,5"}

	mockDb := getMokDbForAvailManager(presignPids, pids)

	allPids := []string{"2", "3", "4", "5", "6", "7"}
	partyIds := getPartyIdsFromStrings(allPids)

	availManager := NewAvailPresignManager(mockDb).(*defaultAvailablePresigns)
	assert.NoError(t, availManager.Load())
	assert.Equal(t, 3, len(availManager.available))

	// Get and consumes 3 presigns
	presignIds, selectedPIDs := availManager.GetAvailablePresigns(3, 3, getPartyIdMap(partyIds))
	assert.Equal(t, 3, len(presignIds))
	assert.Equal(t, 3, len(selectedPIDs))

	// We should have 1 pid string in use (2,3,5) and 2 available pid strings: (1,2,3) and (3,4,5)
	assert.Equal(t, 2, len(availManager.available))

	// Update status
	selectedAps := make([]*common.AvailablePresign, 3)
	for i := 0; i < len(selectedAps); i++ {
		selectedAps[i] = &common.AvailablePresign{
			PresignId:  fmt.Sprintf("%s-%d", "work1", i),
			PidsString: selectedPids,
		}
	}
}

func TestAvailPresignManager_NotFound(t *testing.T) {
	t.Parallel()

	presignPids := []string{"work0-0", "work0-1", "work1-0", "work1-1", "work2-0"}
	pids := []string{"1,2,4", "1,2,4", "2,3,5", "2,3,5", "3,4,5"}

	mockDb := getMokDbForAvailManager(presignPids, pids)

	appPids := []string{"2", "3", "4", "5", "6", "7"}
	partyIds := getPartyIdsFromStrings(appPids)

	availManager := NewAvailPresignManager(mockDb).(*defaultAvailablePresigns)
	assert.NoError(t, availManager.Load())
	assert.Equal(t, 3, len(availManager.available))

	presignIds, _ := availManager.GetAvailablePresigns(3, 3, getPartyIdMap(partyIds))
	assert.Equal(t, 0, len(presignIds))

	assert.Equal(t, 3, len(availManager.available))
}

func TestAvailPresignManager_NotUsed(t *testing.T) {
	t.Parallel()

	selectedPids := "2,3,5"

	presignPids := []string{"work0-0", "work0-1", "work1-0", "work1-1", "work1-2", "work2-0"}
	pids := []string{"1,2,4", "1,2,4", "2,3,5", "2,3,5", "2,3,5", "3,4,5"}
	mockDb := getMokDbForAvailManager(presignPids, pids)
	partyIds := getPartyIdsFromStrings([]string{"2", "3", "4", "5", "6", "7"})

	availManager := NewAvailPresignManager(mockDb)
	assert.NoError(t, availManager.Load())

	presignIds, _ := availManager.GetAvailablePresigns(3, 3, getPartyIdMap(partyIds))
	assert.Equal(t, 3, len(presignIds))

	// Update status
	selectedAps := make([]*common.AvailablePresign, 3)
	for i := 0; i < len(selectedAps); i++ {
		selectedAps[i] = &common.AvailablePresign{
			PresignId:  fmt.Sprintf("%s-%d", "work1", i),
			PidsString: selectedPids,
		}
	}
}

func getPartyIdsFromStrings(pids []string) []*tss.PartyID {
	partyIds := make([]*tss.PartyID, len(pids))
	for i := 0; i < len(pids); i++ {
		partyIds[i] = tss.NewPartyID(pids[i], "", big.NewInt(int64(i+1)))
	}

	return partyIds
}

func getPartyIdMap(partyIds []*tss.PartyID) map[string]*tss.PartyID {
	m := make(map[string]*tss.PartyID)
	for _, partyId := range partyIds {
		m[partyId.Id] = partyId
	}

	return m
}
