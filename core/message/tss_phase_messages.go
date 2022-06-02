package message

import (
	"errors"

	wtypes "github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	ecsigning "github.com/sisu-network/tss-lib/ecdsa/signing"
	"github.com/sisu-network/tss-lib/tss"
	"google.golang.org/protobuf/proto"
)

type Round int

const (
	EcKeygen1 Round = iota + 1
	EcKeygen2
	EcKeygen3
	EcSigning1
	EcSigning2
	EcSigning3
	EcSigning4
	EcSigning5
	EcSigning6
	EcSigning7

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
	case *ecsigning.SignRound1Message1, *ecsigning.SignRound1Message2:
		return EcSigning1, nil
	case *ecsigning.SignRound2Message:
		return EcSigning1, nil
	case *ecsigning.SignRound3Message:
		return EcSigning3, nil
	case *ecsigning.SignRound4Message:
		return EcSigning4, nil
	case *ecsigning.SignRound5Message:
		return EcSigning5, nil
	case *ecsigning.SignRound6Message:
		return EcSigning6, nil
	case *ecsigning.SignRound7Message:
		return EcSigning7, nil

	default:
		return 0, errors.New("unknown round")
	}
}

func GetMessageCountByWorkType(jobType wtypes.WorkType, isPresign bool) int {
	switch jobType {
	case wtypes.EcKeygen:
		return 4
	case wtypes.EcSigning:
		if isPresign {
			return 1
		}

		return 7
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

	case wtypes.EcSigning:
		return []string{
			string(proto.MessageName(&ecsigning.SignRound1Message1{})),
			string(proto.MessageName(&ecsigning.SignRound1Message2{})),
			string(proto.MessageName(&ecsigning.SignRound2Message{})),
			string(proto.MessageName(&ecsigning.SignRound3Message{})),
			string(proto.MessageName(&ecsigning.SignRound4Message{})),
			string(proto.MessageName(&ecsigning.SignRound5Message{})),
			string(proto.MessageName(&ecsigning.SignRound6Message{})),
			string(proto.MessageName(&ecsigning.SignRound7Message{})),
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
		string(proto.MessageName(&ecsigning.SignRound1Message2{})),
		string(proto.MessageName(&ecsigning.SignRound3Message{})),
		string(proto.MessageName(&ecsigning.SignRound4Message{})),
		string(proto.MessageName(&ecsigning.SignRound5Message{})),
		string(proto.MessageName(&ecsigning.SignRound6Message{})),
		string(proto.MessageName(&ecsigning.SignRound7Message{})):

		return true
	default:
		return false
	}
}
