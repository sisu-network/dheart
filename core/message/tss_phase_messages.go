package message

import (
	"errors"

	wTypes "github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/ecdsa/signing"
	"github.com/sisu-network/tss-lib/tss"
)

const (
	Keygen1 = iota + 1
	Keygen2
	Keygen3
	Presign1
	Presign2
	Presign3
	Presign4
	Sign1
)

func GetMsgRound(content tss.MessageContent) (uint32, error) {
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

func NextRound(jobType wTypes.WorkType, curRound uint32) uint32 {
	switch jobType {
	case wTypes.EcdsaKeygen:
		switch curRound {
		case Keygen1:
			return Keygen2
		case Keygen2:
			return Keygen3
		}

	case wTypes.EcdsaPresign:
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
