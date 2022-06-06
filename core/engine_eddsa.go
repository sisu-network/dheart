package core

import (
	"github.com/sisu-network/dheart/worker/types"
	edkeygen "github.com/sisu-network/tss-lib/eddsa/keygen"
)

func (engine *defaultEngine) onEdKeygenFinished(request *types.WorkRequest, outputs *edkeygen.LocalPartySaveData) {
}
