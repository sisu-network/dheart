package core

import (
	dtypes "github.com/sisu-network/dheart/types"
	htypes "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/tss"
)

//---/

type MockEngineCallback struct {
	OnWorkKeygenFinishedFunc  func(result *dtypes.KeygenResult)
	OnWorkSigningFinishedFunc func(request *types.WorkRequest, result *htypes.KeysignResult)
	OnWorkFailedFunc          func(request *types.WorkRequest, culprits []*tss.PartyID)
}

func (cb *MockEngineCallback) OnWorkKeygenFinished(result *dtypes.KeygenResult) {
	if cb.OnWorkKeygenFinishedFunc != nil {
		cb.OnWorkKeygenFinishedFunc(result)
	}
}

func (cb *MockEngineCallback) OnWorkSigningFinished(request *types.WorkRequest, result *htypes.KeysignResult) {
	if cb.OnWorkSigningFinishedFunc != nil {
		cb.OnWorkSigningFinishedFunc(request, result)
	}
}

func (cb *MockEngineCallback) OnWorkFailed(request *types.WorkRequest, culprits []*tss.PartyID) {
	if cb.OnWorkFailedFunc != nil {
		cb.OnWorkFailedFunc(request, culprits)
	}
}

func (cb *MockEngineCallback) OnNodeNotSelected(workId string) {
	// Do nothing.
}
