package message

import (
	"errors"

	"github.com/rs/zerolog/log"
	wtypes "github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/ecdsa/signing"
	"github.com/sisu-network/tss-lib/tss"
	"google.golang.org/protobuf/proto"
)

type Round int

const (
	Keygen1 Round = iota + 1
	Keygen2
	Keygen3
	Presign1
	Presign2
	Presign3
	Presign4
	Presign5
	Presign6
	Presign7
	Sign1
)

func GetMsgRound(content tss.MessageContent) (Round, error) {
	switch content.(type) {
	case *keygen.KGRound1Message:
		return Keygen1, nil
	case *keygen.KGRound2Message1, *keygen.KGRound2Message2:
		return Keygen2, nil
	case *keygen.KGRound3Message:
		return Keygen3, nil
	case *presign.PresignRound1Message1, *presign.PresignRound1Message2:
		return Presign1, nil
	case *presign.PresignRound2Message:
		return Presign2, nil
	case *presign.PresignRound3Message:
		return Presign3, nil
	case *presign.PresignRound4Message:
		return Presign4, nil
	case *presign.PresignRound5Message:
		return Presign5, nil
	case *presign.PresignRound6Message:
		return Presign6, nil
	case *presign.PresignRound7Message:
		return Presign7, nil
	case *signing.SignRound1Message:
		return Sign1, nil

	default:
		return 0, errors.New("unknown round")
	}
}

func GetMessageCountByWorkType(jobType wtypes.WorkType) int {
	switch jobType {
	case wtypes.EcdsaKeygen:
		return 4
	case wtypes.EcdsaPresign:
		return 8
	case wtypes.EcdsaSigning:
		return 1
	default:
		log.Error("Unsupported work type: ", jobType.String())
		return 0
	}
}

func GetMessagesByWorkType(jobType wtypes.WorkType) []string {
	switch jobType {
	case wtypes.EcdsaKeygen:
		return []string{
			string(proto.MessageName(&keygen.KGRound1Message{})),
			string(proto.MessageName(&keygen.KGRound2Message1{})),
			string(proto.MessageName(&keygen.KGRound2Message2{})),
			string(proto.MessageName(&keygen.KGRound3Message{})),
		}
	case wtypes.EcdsaPresign:
		return []string{

			string(proto.MessageName(&presign.PresignRound1Message1{})),
			string(proto.MessageName(&presign.PresignRound1Message2{})),
			string(proto.MessageName(&presign.PresignRound2Message{})),
			string(proto.MessageName(&presign.PresignRound3Message{})),
			string(proto.MessageName(&presign.PresignRound4Message{})),
			string(proto.MessageName(&presign.PresignRound5Message{})),
			string(proto.MessageName(&presign.PresignRound6Message{})),
			string(proto.MessageName(&presign.PresignRound7Message{})),
		}
	case wtypes.EcdsaSigning:
		return []string{
			string(proto.MessageName(&signing.SignRound1Message{})),
		}
	}

	return make([]string, 0)
}

// IsBroadcastMessage return true if it's broadcast message
func IsBroadcastMessage(msgType string) bool {
	switch msgType {
	case string(proto.MessageName(&keygen.KGRound1Message{})),
		string(proto.MessageName(&keygen.KGRound2Message2{})),
		string(proto.MessageName(&keygen.KGRound3Message{})),
		string(proto.MessageName(&presign.PresignRound1Message2{})),
		string(proto.MessageName(&presign.PresignRound3Message{})),
		string(proto.MessageName(&presign.PresignRound4Message{})),
		string(proto.MessageName(&presign.PresignRound5Message{})),
		string(proto.MessageName(&presign.PresignRound6Message{})),
		string(proto.MessageName(&presign.PresignRound7Message{})),
		string(proto.MessageName(&signing.SignRound1Message{})):

		return true
	default:
		return false
	}
}
