package worker

import (
	libCommon "github.com/sisu-network/tss-lib/common"

	commonTypes "github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/types"
	eckeygen "github.com/sisu-network/tss-lib/ecdsa/keygen"
	ecpresign "github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

type Worker interface {
	// Start runs this worker. The cached messages are list (could be empty) of messages sent to
	// this node before this worker starts.
	Start(cachedMsgs []*commonTypes.TssMessage) error

	// GetPartyId returns party id of the current node.
	GetPartyId() string

	// ProcessNewMessage receives new message from network and update current tss round.
	ProcessNewMessage(tssMsg *commonTypes.TssMessage) error

	// GetCulprits ...
	GetCulprits() []*tss.PartyID

	// Stop stops the worker and cleans all the resources
	Stop()
}

// A callback for the caller to receive updates from this worker. We use callback instead of Go
// channel to avoid creating too many channels.
type WorkerCallback interface {
	// GetAvailablePresigns returns a list of presign output that will be used for signing. The presign's
	// party ids should match the pids params passed into the function.
	GetAvailablePresigns(batchSize int, n int, allPids map[string]*tss.PartyID) ([]string, []*tss.PartyID)

	GetPresignOutputs(presignIds []string) []*ecpresign.LocalPresignData

	OnNodeNotSelected(request *types.WorkRequest)

	OnWorkFailed(request *types.WorkRequest)

	OnWorkerResult(request *types.WorkRequest, result *WorkerResult)
}

type WorkerResult struct {
	Success        bool
	IsNodeSelected bool
	Request        *types.WorkRequest
	SelectedPids   []*tss.PartyID

	// Ecdsa
	EcKeygenData  []*eckeygen.LocalPartySaveData
	EcPresignData []*ecpresign.LocalPresignData
	EcSigningData []*libCommon.ECSignature

	// Eddsa
}
