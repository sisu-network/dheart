package core

import (
	cryptoec "crypto/ecdsa"

	"github.com/ethereum/go-ethereum/crypto"
	htypes "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/tss"
)

func (engine *defaultEngine) onEcKeygenFinished(request *types.WorkRequest, output *keygen.LocalPartySaveData) {
	log.Info("Keygen finished for type ", request.KeygenType)

	pkX, pkY := output.ECDSAPub.X(), output.ECDSAPub.Y()
	publicKeyECDSA := cryptoec.PublicKey{
		Curve: tss.EC(tss.EcdsaScheme),
		X:     pkX,
		Y:     pkY,
	}
	address := crypto.PubkeyToAddress(publicKeyECDSA).Hex()
	publicKeyBytes := crypto.FromECDSAPub(&publicKeyECDSA)

	log.Verbose("publicKeyBytes length = ", len(publicKeyBytes))

	// Make a callback and start next work.
	result := htypes.KeygenResult{
		KeyType:     request.KeygenType,
		PubKeyBytes: publicKeyBytes,
		Outcome:     htypes.OutcomeSuccess,
		Address:     address,
	}

	engine.callback.OnWorkKeygenFinished(&result)
}

func (engine *defaultEngine) onEcSigningFinished(request *types.WorkRequest, data []*libCommon.ECSignature) {
	log.Info("Signing finished for Ecdsa workId ", request.WorkId)

	signatures := make([][]byte, len(data))
	for i := range data {
		signatures[i] = append(signatures[i], data[i].SignatureRecovery[0])
	}

	result := &htypes.KeysignResult{
		Outcome:    htypes.OutcomeSuccess,
		Signatures: signatures,
	}

	for _, sig := range signatures {
		log.Debugf("Signature = %s, length signature = ", string(sig), len(sig))
	}

	engine.callback.OnWorkSigningFinished(request, result)
}
