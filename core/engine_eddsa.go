package core

import (
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
	wtypes "github.com/sisu-network/dheart/worker/types"
	edkeygen "github.com/sisu-network/tss-lib/eddsa/keygen"
)

func (engine *defaultEngine) onEdKeygenFinished(request *wtypes.WorkRequest, output *edkeygen.LocalPartySaveData) {
	bz := output.EDDSAPub.Bytes()
	pubkey := edwards.NewPublicKey(output.EDDSAPub.X(), output.EDDSAPub.Y())

	// Make a callback and start next work.
	result := types.KeygenResult{
		KeyType:     request.KeygenType,
		PubKeyBytes: bz,
		Outcome:     types.OutcomeSuccess,
		Address:     utils.GetAddressFromCardanoPubkey(pubkey.Serialize()).String(),
	}

	engine.callback.OnWorkKeygenFinished(&result)
}
