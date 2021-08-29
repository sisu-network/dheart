package common

import (
	"encoding/json"

	"github.com/sisu-network/tss-lib/tss"
)

func NewTssGetOnlineNodesRequest(from, to, workId string) *TssMessage {
	msg := &TssMessage{}

	// msg.Type = TssMessage_GET_ONLINE_NODES_REQUEST
	msg.From = from
	msg.To = to
	msg.WorkId = workId

	return msg
}

func NewTssUpdateMessages(from, to, workId string, msgs []tss.Message, round string) (*TssMessage, error) {
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
		WorkId:         workId,
		UpdateMessages: updateMessages,
	}, nil
}

func (msg *TssMessage) IsBroadcast() bool {
	return msg.To == ""
}
