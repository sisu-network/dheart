package common

import (
	"encoding/json"

	"github.com/sisu-network/tss-lib/tss"
)

func NewTssMessage(from string, to string, msgs []tss.Message, round string) (*TssMessage, error) {
	// Serialize tss messages
	updateMessages := make([]*UpdateMessage, len(msgs))

	for i, msg := range msgs {
		msgsBytes, routing, err := msg.WireBytes()
		if err != nil {
			return nil, err
		}

		serialized, err := json.Marshal(routing)

		updateMessages[i] = &UpdateMessage{
			Data:                     msgsBytes,
			SerializedMessageRouting: serialized,
			Round:                    round,
		}
	}

	return &TssMessage{
		From:           from,
		To:             to,
		UpdateMessages: updateMessages,
	}, nil
}

func (msg *TssMessage) IsBroadcast() bool {
	return msg.To == ""
}
