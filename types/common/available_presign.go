package common

import ecsigning "github.com/sisu-network/tss-lib/ecdsa/signing"

type AvailablePresign struct {
	PresignId  string
	PidsString string
	Pids       []string
	Output     *ecsigning.SignatureData_OneRoundData
}
