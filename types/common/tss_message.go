package common

import (
	"encoding/json"
	"github.com/sisu-network/lib/log"

	"github.com/sisu-network/tss-lib/tss"
)

func NewTssMessage(from, to, workId string, msgs []tss.Message, round string) (*TssMessage, error) {
	// Serialize tss messages
	updateMessages := make([]*UpdateMessage, len(msgs))

	for i, msg := range msgs {
		msgsBytes, routing, err := msg.WireBytes()
		if err != nil {
			log.Error("error when get wire bytes from tss message: ", err)
			return nil, err
		}

		serialized, err := json.Marshal(routing)
		if err != nil {
			log.Error("error when serialized routing info: ", err)
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

func NewAvailabilityResponseMessage(from, to, workId string, answer AvailabilityResponseMessage_ANSWER, maxJob int) *TssMessage {
	msg := baseMessage(TssMessage_AVAILABILITY_RESPONSE, from, to, workId)
	msg.AvailabilityResponseMessage = &AvailabilityResponseMessage{
		Answer: AvailabilityResponseMessage_YES,
		MaxJob: int32(maxJob),
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

func NewRequestMessage(from, to, workId, msgKey string) *TssMessage {
	msg := baseMessage(TssMessage_ASK_MESSAGE_REQUEST, from, to, workId)
	msg.AskRequestMessage = &AskRequestMessage{
		MsgKey: msgKey,
	}

	return msg
}

func NewAckKeygenDoneMessage(from, to, workId string) *TssMessage {
	msg := baseMessage(TssMessage_ACK_DONE, from, to, workId)
	msg.AckDoneMessage = &AckDoneMessage{
		AckType: AckDoneMessage_KEYGEN,
	}
	return msg
}

func NewAckPresignDoneMessage(from, to, workId string) *TssMessage {
	msg := baseMessage(TssMessage_ACK_DONE, from, to, workId)
	msg.AckDoneMessage = &AckDoneMessage{
		AckType: AckDoneMessage_PRESIGN,
	}
	return msg
}

func NewAckSigningDoneMessage(from, to, workId string) *TssMessage {
	msg := baseMessage(TssMessage_ACK_DONE, from, to, workId)
	msg.AckDoneMessage = &AckDoneMessage{
		AckType: AckDoneMessage_SIGNING,
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
