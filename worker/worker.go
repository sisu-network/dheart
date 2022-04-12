package worker

import (
	commonTypes "github.com/sisu-network/dheart/types/common"
	"github.com/sisu-network/tss-lib/tss"
)

type Worker interface {
	// Start runs this worker. The cached messages are list (could be empty) of messages sent to
	// this node before this worker starts.
	Start(cachedMsgs []*commonTypes.TssMessage) error

	// GetWorkId returns the work id of the current worker.
	GetWorkId() string

	// GetPartyId returns party id of the current node.
	GetPartyId() string

	// ProcessNewMessage receives new message from network and update current tss round.
	ProcessNewMessage(tssMsg *commonTypes.TssMessage) error

	// GetCulprits ...
	GetCulprits() []*tss.PartyID

	GetPartyMap() map[string]*tss.PartyID

	// Stop stops the worker and cleans all the resources
	Stop()
}
