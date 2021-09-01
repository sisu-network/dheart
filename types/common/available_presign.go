package common

import "github.com/sisu-network/tss-lib/ecdsa/presign"

type AvailablePresign struct {
	Pids       []string
	WorkId     string
	BatchIndex int
	Output     *presign.LocalPresignData
}
