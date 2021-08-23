package core

import (
	"testing"

	"github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/dheart/worker/types"
)

func TestEngine(t *testing.T) {
	n := 3

	pIDs := helper.GeneratePartyIds(n)
	savedData := helper.LoadKeygenSavedData(n)
	// errCh := make(chan error)
	outCh := make(chan *common.TssMessage)
	engines := make([]*Engine, n)
	workId := "presign0"

	for i := 0; i < n; i++ {
		engines[i] = NewEngine(pIDs[i], helper.NewTestDispatcher(outCh))
	}

	// Start all engines
	for i := 0; i < n; i++ {
		request := types.NewPresignRequest(workId, pIDs, *savedData[i])
		engines[i].AddRequest(request)
	}
}
