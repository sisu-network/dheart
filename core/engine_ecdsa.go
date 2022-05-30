package core

import (
	cryptoec "crypto/ecdsa"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/crypto"
	htypes "github.com/sisu-network/dheart/types"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/types"
	libchain "github.com/sisu-network/lib/chain"
	"github.com/sisu-network/lib/log"
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

func (engine *defaultEngine) OnWorkKeygenFinished(request *types.WorkRequest, output []*keygen.LocalPartySaveData) {
	log.Info("Keygen finished for type ", request.KeygenType)
	// Save to database
	if err := engine.db.SaveKeygenData(request.KeygenType, request.WorkId, request.AllParties, output); err != nil {
		log.Error("error when saving keygen data", err)
	}

	pkX, pkY := output[0].ECDSAPub.X(), output[0].ECDSAPub.Y()
	publicKeyECDSA := cryptoec.PublicKey{
		Curve: tss.EC(),
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
	engine.finishWorker(request.WorkId)
	engine.startNextWork()
}

func (engine *defaultEngine) OnWorkPresignFinished(request *types.WorkRequest, pids []*tss.PartyID,
	data []*presign.LocalPresignData) {
	log.Info("Presign finished, request.WorkId = ", request.WorkId)

	engine.presignsManager.AddPresign(request.WorkId, pids, data)

	result := htypes.PresignResult{
		Outcome: htypes.OutcomeSuccess,
	}

	engine.callback.OnWorkPresignFinished(&result)

	engine.finishWorker(request.WorkId)
	engine.startNextWork()
}

func (engine *defaultEngine) OnWorkSigningFinished(request *types.WorkRequest, data []*libCommon.ECSignature) {
	log.Info("Signing finished for workId ", request.WorkId)

	signatures := make([][]byte, len(data))
	for i, sig := range data {
		r := sig.R
		s := sig.S

		if libchain.IsETHBasedChain(request.Chains[0]) {
			bitSizeInBytes := tss.EC().Params().BitSize / 8
			r = utils.PadToLengthBytesForSignature(sig.R, bitSizeInBytes)
			s = utils.PadToLengthBytesForSignature(sig.S, bitSizeInBytes)
		}

		signatures[i] = append(r, s...)
		if libchain.IsETHBasedChain(request.Chains[i]) {
			if len(signatures[i]) != 64 {
				log.Error("ETH signature length is not 65, actual length = ", len(signatures[i]),
					" msg = ", hex.EncodeToString([]byte(request.Messages[i])),
					" recovery = ", int(data[i].SignatureRecovery[0]))
			}

			signatures[i] = append(signatures[i], data[i].SignatureRecovery[0])
		}
	}

	result := &htypes.KeysignResult{
		Outcome:    htypes.OutcomeSuccess,
		Signatures: signatures,
	}

	engine.callback.OnWorkSigningFinished(request, result)

	engine.finishWorker(request.WorkId)
	engine.startNextWork()
}
