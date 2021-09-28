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
	Keygen1   = "KGRound1Message"
	Keygen21  = "KGRound2Message1"
	Keygen22  = "KGRound2Message2"
	Keygen3   = "KGRound3Message"
	Presign11 = "PresignRound1Message1"
	Presign12 = "PresignRound1Message2"
	Presign2  = "PresignRound2Message"
	Presign3  = "PresignRound3Message"
	Presign4  = "PresignRound4Message"
	Sign1     = "SignRound1Message"
)

func GetMsgRound(content tss.MessageContent) (string, error) {
	switch content.(type) {
	case *keygen.KGRound1Message:
		return Keygen1, nil

	case *keygen.KGRound2Message1:
		return Keygen21, nil

	case *keygen.KGRound2Message2:
		return Keygen22, nil

	case *keygen.KGRound3Message:
		return Keygen3, nil

	case *presign.PresignRound1Message1:
		return Presign11, nil

	case *presign.PresignRound1Message2:
		return Presign12, nil

	case *presign.PresignRound2Message:
		return Presign2, nil

	case *presign.PresignRound3Message:
		return Presign3, nil

	case *presign.PresignRound4Message:
		return Presign4, nil

	case *signing.SignRound1Message:
		return Sign1, nil

	default:
		return "", errors.New("unknown round")
	}
}

func NextRound(jobType wTypes.WorkType, curRound string) string {
	switch jobType {
	case wTypes.EcdsaKeygen:
		switch curRound {
		case Keygen1:
			return Keygen21
		case Keygen21:
			return Keygen22
		case Keygen22:
			return Keygen3
		}

	case wTypes.EcdsaPresign:
		switch curRound {
		case Presign11:
			return Presign12
		case Presign12:
			return Presign2
		case Presign2:
			return Presign3
		case Keygen3:
			return Presign4
		}
	}

	return curRound
}
