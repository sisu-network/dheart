package worker

import (
	"fmt"
	"sync"
	"testing"

	"github.com/sisu-network/dheart/core/cache"
	"github.com/sisu-network/dheart/core/components"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/require"
)

func getDb(index int) db.Database {
	dbConfig := config.GetLocalhostDbConfig()
	dbConfig.Schema = fmt.Sprintf("dheart%d", index)
	dbConfig.InMemory = true
	dbInstance := db.NewDatabase(&dbConfig)

	return dbInstance
}

func TestPreworkSelection_SuccessfulSeleciton(t *testing.T) {
	n := 4
	pIDs := GetTestPartyIds(n)

	workId := "eddsaKeygen"
	selections := make([]*PreworkSelection, n)

	// Mock message routing
	dispatcher := &MockMessageDispatcher{
		BroadcastMessageFunc: func(pIDs []*tss.PartyID, tssMessage *common.TssMessage) {
			for _, selection := range selections {
				if selection.myPid.Id != tssMessage.From {
					selection.ProcessNewMessage(tssMessage)
				}
			}
		},

		UnicastMessageFunc: func(dest *tss.PartyID, tssMessage *common.TssMessage) {
			for _, selection := range selections {
				if selection.myPid.Id == dest.Id {
					selection.ProcessNewMessage(tssMessage)
					break
				}
			}
		},
	}

	done := &sync.WaitGroup{}
	done.Add(n)

	for i := 0; i < n; i++ {
		dbInstance := getDb(i)
		// Mock callback when selection finishes.
		cb := func(result SelectionResult) {
			require.True(t, result.Success)
			done.Done()
		}

		selections[i] = NewPreworkSelection(
			types.NewEdKeygenRequest(workId, pIDs, 2),
			pIDs,
			pIDs[i],
			dbInstance,
			cache.NewMessageCache(),
			dispatcher,
			components.NewAvailPresignManager(dbInstance),
			config.NewDefaultTimeoutConfig(),
			cb,
		)

		selections[i].Init()
	}

	// Run the selection.
	for _, selection := range selections {
		go selection.Run(make([]*common.TssMessage, 0))
	}

	done.Wait()
}
