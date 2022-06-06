package worker

import (
	libCommon "github.com/sisu-network/tss-lib/common"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	ecsigning "github.com/sisu-network/tss-lib/ecdsa/signing"
)

func GetEcKeygenOutputs(results []*JobResult) []*keygen.LocalPartySaveData {
	outputs := make([]*keygen.LocalPartySaveData, len(results))
	for i := range results {
		outputs[i] = results[i].EcKeygen
	}

	return outputs
}

func GetEcPresignOutputs(results []*JobResult) []*ecsigning.SignatureData_OneRoundData {
	outputs := make([]*ecsigning.SignatureData_OneRoundData, len(results))
	for i := range results {
		outputs[i] = results[i].EcPresign
	}
	return outputs
}

func GetEcSigningOutputs(results []*JobResult) []*libCommon.ECSignature {
	outputs := make([]*libCommon.ECSignature, len(results))
	for i := range results {
		outputs[i] = results[i].EcSigning.Signature
	}
	return outputs
}
