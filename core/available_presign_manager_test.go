package core

import (
	"testing"

	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/stretchr/testify/assert"
)

func getMokDbForAvailManager(presignPids, workIds []string, batchIndes []int) db.Database {
	return &helper.MockDatabase{
		GetAvailablePresignShortFormFunc: func() ([]string, []string, []int, error) {
			return presignPids, workIds, batchIndes, nil
		},

		LoadPresignFunc: func(workId string, batchIndexes []int) ([]*presign.LocalPresignData, error) {
			count := 0
			for _, w := range workIds {
				if w == workId {
					count++
				}
			}

			ret := make([]*presign.LocalPresignData, count)
			return ret, nil
		},
	}
}

func TestAvailManagerHappyCase(t *testing.T) {
	expectedWorkId := "work1"
	selectedPids := "2,3,5"

	workIds := []string{"work0", "work0", "work1", "work1", "work1", "work2"}
	presignPids := []string{"1,2,4", "1,2,4", "2,3,5", "2,3,5", "2,3,5", "3,4,5"}
	batchIndexes := []int{0, 1, 0, 1, 2, 0}

	mockDb := getMokDbForAvailManager(presignPids, workIds, batchIndexes)

	pids := []string{"2", "3", "4", "5", "6", "7"}
	partyIds := getPartyIdsFromStrings(pids)

	availManager := NewAvailPresignManager(mockDb)
	availManager.Load()
	assert.Equal(t, 3, len(availManager.available))

	presignIds, _ := availManager.GetAvailablePresigns(3, 3, partyIds)
	assert.Equal(t, 3, len(presignIds))

	assert.Equal(t, 1, len(availManager.inUse))
	assert.Equal(t, 2, len(availManager.available))

	// Update status
	aps := make([]*common.AvailablePresign, 3)
	for i := 0; i < len(aps); i++ {
		aps[i] = &common.AvailablePresign{
			WorkId:     expectedWorkId,
			BatchIndex: i,
		}
	}

	availManager.updateUsage(selectedPids, aps, true)
	assert.Equal(t, 0, len(availManager.inUse))
	assert.Equal(t, 2, len(availManager.available))
}

func TestAvailManagerNotFound(t *testing.T) {
	workIds := []string{"work0", "work0", "work1", "work1", "work2"}
	presignPids := []string{"1,2,4", "1,2,4", "2,3,5", "2,3,5", "3,4,5"}
	batchIndexes := []int{0, 1, 0, 1, 2, 0}

	mockDb := getMokDbForAvailManager(presignPids, workIds, batchIndexes)

	pids := []string{"2", "3", "4", "5", "6", "7"}
	partyIds := getPartyIdsFromStrings(pids)

	availManager := NewAvailPresignManager(mockDb)
	availManager.Load()
	assert.Equal(t, 3, len(availManager.available))

	presignIds, _ := availManager.GetAvailablePresigns(3, 3, partyIds)
	assert.Equal(t, 0, len(presignIds))

	assert.Equal(t, 0, len(availManager.inUse))
	assert.Equal(t, 3, len(availManager.available))
}

func TestAvailManagerPresignNotUsed(t *testing.T) {
	expectedWorkId := "work1"
	selectedPids := "2,3,5"

	workIds := []string{"work0", "work0", "work1", "work1", "work1", "work2"}
	presignPids := []string{"1,2,4", "1,2,4", "2,3,5", "2,3,5", "2,3,5", "3,4,5"}
	batchIndexes := []int{0, 1, 0, 1, 2, 0}

	mockDb := getMokDbForAvailManager(presignPids, workIds, batchIndexes)

	pids := []string{"2", "3", "4", "5", "6", "7"}
	partyIds := getPartyIdsFromStrings(pids)

	availManager := NewAvailPresignManager(mockDb)
	availManager.Load()

	presignIds, _ := availManager.GetAvailablePresigns(3, 3, partyIds)
	assert.Equal(t, 3, len(presignIds))

	// Update status
	aps := make([]*common.AvailablePresign, 3)
	for i := 0; i < len(aps); i++ {
		aps[i] = &common.AvailablePresign{
			WorkId:     expectedWorkId,
			BatchIndex: i,
		}
	}

	availManager.updateUsage(selectedPids, aps, false)
	assert.Equal(t, 0, len(availManager.inUse))
	assert.Equal(t, 3, len(availManager.available))
}
