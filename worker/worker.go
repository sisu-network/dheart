package worker

import (
	commonTypes "github.com/sisu-network/dheart/types/common"
)

type Worker interface {
	Start() error
	GetId() string
	ProcessNewMessage(tssMsg *commonTypes.TssMessage) error
}
