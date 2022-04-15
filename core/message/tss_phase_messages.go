package message

import (
	"errors"

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

	case *signing.SignRound1Message:
		return Sign1, nil

	default:
		return 0, errors.New("unknown round")
	}
}

func NextRound(jobType wtypes.WorkType, curRound Round) Round {
	switch jobType {
	case wtypes.EcdsaKeygen:
		switch curRound {
		case Keygen1:
			return Keygen2
		case Keygen2:
			return Keygen3
		}

	case wtypes.EcdsaPresign:
		switch curRound {
		case Presign1:
			return Presign2
		case Presign2:
			return Presign3
		case Presign3:
			return Presign4
		}
	}

	return curRound
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
		}
	case wtypes.EcdsaSigning:
		return []string{
			string(proto.MessageName(&signing.SignRound1Message{})),
		}
	}

	return make([]string, 0)
}

// GetAllMessageTypesByRound gets all messages for a Dheart round
// Use ConvertTSSRoundToDheartRound to convert from tss to Dheart round first
func GetAllMessageTypesByRound(round Round) []string {
	switch round {
	case Keygen1:
		return []string{
			string(proto.MessageName(&keygen.KGRound1Message{})),
		}
	case Keygen2:
		return []string{
			string(proto.MessageName(&keygen.KGRound2Message1{})),
			string(proto.MessageName(&keygen.KGRound2Message2{})),
		}
	case Keygen3:
		return []string{
			string(proto.MessageName(&keygen.KGRound3Message{})),
		}
	case Presign1:
		return []string{
			string(proto.MessageName(&presign.PresignRound1Message1{})),
			string(proto.MessageName(&presign.PresignRound1Message2{})),
		}
	case Presign2:
		return []string{
			string(proto.MessageName(&presign.PresignRound2Message{})),
		}
	case Presign3:
		return []string{
			string(proto.MessageName(&presign.PresignRound3Message{})),
		}
	case Presign4:
		return []string{
			string(proto.MessageName(&presign.PresignRound4Message{})),
		}
	case Sign1:
		return []string{
			string(proto.MessageName(&signing.SignRound1Message{})),
		}
	default:
		return []string{}
	}
}

func ConvertTSSRoundToDheartRound(tssRound int, roundType wtypes.WorkType) Round {
	switch roundType {
	case wtypes.EcdsaKeygen:
		return Round(tssRound)
	case wtypes.EcdsaPresign:
		// 3 is number of keygen rounds
		return Round(tssRound + 3)
	case wtypes.EcdsaSigning:
		// 7 is number of keygen + presign rounds
		return Round(tssRound + 7)
	default:
		return Round(0)
	}
}

// IsBroadcastMessage return true if it's broadcast message
func IsBroadcastMessage(msgType string) bool {
	switch msgType {
	case string(proto.MessageName(&keygen.KGRound1Message{})),
		string(proto.MessageName(&keygen.KGRound2Message2{})),
		string(proto.MessageName(&keygen.KGRound3Message{})),
		string(proto.MessageName(&presign.PresignRound1Message2{})),
		string(proto.MessageName(&presign.PresignRound3Message{})),
		string(proto.MessageName(&presign.PresignRound4Message{})):

		return true
	default:
		return false
	}
}
