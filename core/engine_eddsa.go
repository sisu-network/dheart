package core

import (
	"fmt"
	"math/big"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/sisu-network/dheart/types"
	htypes "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
	wtypes "github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	edkeygen "github.com/sisu-network/tss-lib/eddsa/keygen"
	edsigning "github.com/sisu-network/tss-lib/eddsa/signing"
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

func (engine *defaultEngine) onEdSigningFinished(request *wtypes.WorkRequest, data []*edsigning.SignatureData) {
	log.Info("Signing finished for Eddsa workId ", request.WorkId)

	signatures := make([][]byte, len(data))
	for i := range data {
		signatures[i] = data[i].Signature.Signature
	}

	myKeygen := request.EdSigningInput
	pubkey := edwards.NewPublicKey(myKeygen.EDDSAPub.X(), myKeygen.EDDSAPub.Y())
	if !edwards.Verify(pubkey, request.Messages[0], new(big.Int).SetBytes(data[0].Signature.R),
		new(big.Int).SetBytes(data[0].Signature.S)) {
	} else {
		fmt.Println("Signature Success!!", len(data[0].Signature.R), len(data[0].Signature.S))
	}

	result := &htypes.KeysignResult{
		Outcome:    htypes.OutcomeSuccess,
		Signatures: signatures,
	}

	engine.callback.OnWorkSigningFinished(request, result)
}
