package message

import (
	"errors"

	wtypes "github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/ecdsa/signing"
	"github.com/sisu-network/tss-lib/tss"
	"google.golang.org/protobuf/proto"
)

type Round int

const (
	EcKeygen1 Round = iota + 1
	EcKeygen2
	EcKeygen3
	EcPresign1
	EcPresign2
	EcPresign3
	EcPresign4
	EcPresign5
	EcPresign6
	EcPresign7
	EcSigning1

	// Eddsa
	EdKeygen1
	EdKeygen2
	EdSigning1
	EdSigning2
	EdSigning3
)

func GetMsgRound(content tss.MessageContent) (Round, error) {
	switch content.(type) {
	case *keygen.KGRound1Message:
		return EcKeygen1, nil
	case *keygen.KGRound2Message1, *keygen.KGRound2Message2:
		return EcKeygen2, nil
	case *keygen.KGRound3Message:
		return EcKeygen3, nil
	case *presign.PresignRound1Message1, *presign.PresignRound1Message2:
		return EcPresign1, nil
	case *presign.PresignRound2Message:
		return EcPresign2, nil
	case *presign.PresignRound3Message:
		return EcPresign3, nil
	case *presign.PresignRound4Message:
		return EcPresign4, nil
	case *presign.PresignRound5Message:
		return EcPresign5, nil
	case *presign.PresignRound6Message:
		return EcPresign6, nil
	case *presign.PresignRound7Message:
		return EcPresign7, nil
	case *signing.SignRound1Message:
		return EcSigning1, nil

	default:
		return 0, errors.New("unknown round")
	}
}

func GetMessageCountByWorkType(jobType wtypes.WorkType) int {
	switch jobType {
	case wtypes.EcKeygen:
		return 4
	case wtypes.EcPresign:
		return 8
	case wtypes.EcSigning:
		return 1
	case wtypes.EdKeygen:
		return 3
	case wtypes.EdSigning:
		return 3
	default:
		log.Error("Unsupported work type: ", jobType.String())
		return 0
	}
}

func GetMessagesByWorkType(jobType wtypes.WorkType) []string {
	switch jobType {
	case wtypes.EcKeygen:
		return []string{
			string(proto.MessageName(&keygen.KGRound1Message{})),
			string(proto.MessageName(&keygen.KGRound2Message1{})),
			string(proto.MessageName(&keygen.KGRound2Message2{})),
			string(proto.MessageName(&keygen.KGRound3Message{})),
		}
	case wtypes.EcPresign:
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
	case wtypes.EcSigning:
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
