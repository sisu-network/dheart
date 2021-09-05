package common

import "github.com/sisu-network/tss-lib/ecdsa/presign"

type AvailablePresign struct {
	PresignId  string
	PidsString string
	Pids       []string
	Output     *presign.LocalPresignData
}
