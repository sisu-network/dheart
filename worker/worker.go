package worker

import (
	commonTypes "github.com/sisu-network/dheart/types/common"
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
}
