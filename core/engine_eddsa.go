package core

import (
	"encoding/hex"
	"math/big"

	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/sisu-network/dheart/types"
	htypes "github.com/sisu-network/dheart/types"
	wtypes "github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	edkeygen "github.com/sisu-network/tss-lib/eddsa/keygen"
	edsigning "github.com/sisu-network/tss-lib/eddsa/signing"
)

func (engine *defaultEngine) onEdKeygenFinished(request *wtypes.WorkRequest, output *edkeygen.LocalPartySaveData) {
	pubkey := edwards.NewPublicKey(output.EDDSAPub.X(), output.EDDSAPub.Y())

	// Make a callback and start next work.
	result := types.KeygenResult{
		KeyType:     request.KeygenType,
		PubKeyBytes: pubkey.Serialize(),
		Outcome:     types.OutcomeSuccess,
	}

	engine.callback.OnWorkKeygenFinished(&result)
}

func (engine *defaultEngine) onEdSigningFinished(request *wtypes.WorkRequest, data []*edsigning.SignatureData) {
	log.Infof("%s Signing finished for Eddsa workId %s", engine.myPid.Id[len(engine.myPid.Id)-4:],
		request.WorkId)

	signatures := make([][]byte, len(data))
	for i := range data {
		signatures[i] = data[i].Signature.Signature
	}

	myKeygen := request.EdSigningInput
	pubkey := edwards.NewPublicKey(myKeygen.EDDSAPub.X(), myKeygen.EDDSAPub.Y())
	if !edwards.Verify(pubkey, request.Messages[0], new(big.Int).SetBytes(data[0].Signature.R),
		new(big.Int).SetBytes(data[0].Signature.S)) {
		log.Critical("EDDSA signing failed, msg hex = ", hex.EncodeToString(request.Messages[0]))
	}

	result := &htypes.KeysignResult{
		Outcome:    htypes.OutcomeSuccess,
		Signatures: signatures,
	}

	engine.callback.OnWorkSigningFinished(request, result)
}
