package worker

import (
	libCommon "github.com/sisu-network/tss-lib/common"

	commonTypes "github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
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

	GetPresignOutputs(presignIds []string) []*presign.LocalPresignData

	OnNodeNotSelected(request *types.WorkRequest)

	OnWorkFailed(request *types.WorkRequest)

	OnWorkKeygenFinished(request *types.WorkRequest, data []*keygen.LocalPartySaveData)

	OnWorkPresignFinished(request *types.WorkRequest, selectedPids []*tss.PartyID, data []*presign.LocalPresignData)

	OnWorkSigningFinished(request *types.WorkRequest, data []*libCommon.ECSignature)
}
