package common

import (
	"encoding/json"

	"github.com/sisu-network/tss-lib/tss"
)

func NewTssMessage(from, to, workId string, msgs []tss.Message, round string) (*TssMessage, error) {
	// Serialize tss messages
	updateMessages := make([]*UpdateMessage, len(msgs))

	for i, msg := range msgs {
		msgsBytes, routing, err := msg.WireBytes()
		if err != nil {
			return nil, err
		}

		serialized, err := json.Marshal(routing)
		if err != nil {
			return nil, err
		}

		updateMessages[i] = &UpdateMessage{
			Data:                     msgsBytes,
			SerializedMessageRouting: serialized,
			Round:                    round,
		}
	}

	msg := baseMessage(TssMessage_UPDATE_MESSAGES, from, to, workId)
	msg.UpdateMessages = updateMessages

	return msg, nil
}

func NewAvailabilityRequestMessage(from, to, workId string) *TssMessage {
	msg := baseMessage(TssMessage_AVAILABILITY_REQUEST, from, to, workId)
	return msg
}

func NewAvailabilityResponseMessage(from, to, workId string, answer AvailabilityResponseMessage_ANSWER) *TssMessage {
	msg := baseMessage(TssMessage_AVAILABILITY_RESPONSE, from, to, workId)
	msg.AvailabilityResponseMessage = &AvailabilityResponseMessage{
		Answer: AvailabilityResponseMessage_YES,
	}

	return msg
}

func NewPreExecOutputMessage(from, to, workId string, success bool, presignIds []string, pids []*tss.PartyID) *TssMessage {
	msg := baseMessage(TssMessage_PRE_EXEC_OUTPUT, from, to, workId)

	// get all pid strings
	s := make([]string, len(pids))
	for i, p := range pids {
		s[i] = p.Id
	}

	msg.PreExecOutputMessage = &PreExecOutputMessage{
		Success:    success,
		Pids:       s,
		PresignIds: presignIds,
	}

	return msg
}

func baseMessage(typez TssMessage_Type, from, to, workId string) *TssMessage {
	return &TssMessage{
		Type:   typez,
		From:   from,
		To:     to,
		WorkId: workId,
	}
}

func (msg *TssMessage) IsBroadcast() bool {
	return msg.To == ""
}
