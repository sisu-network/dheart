package core

import (
	"crypto"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/sisu-network/dheart/worker/types"
	"github.com/sisu-network/lib/log"
)

func GetKeysignWorkId(workType types.WorkType, txs [][]byte, block int64, chain string) string {
	var prefix string
	switch workType {
	case types.EcSigning:
		prefix = "ecdsa_signing"
	case types.EdSigning:
		prefix = "eddsa_signing"
	default:
		log.Critical("Invalid keygen work type, workType =", workType)
		return ""
	}

	digester := crypto.MD5.New()
	for _, tx := range txs {
		fmt.Fprint(digester, tx)
	}
	hash := hex.EncodeToString(digester.Sum(nil))

	return prefix + "-" + strconv.FormatInt(block, 10) + "-" + chain + "-" + hash
}
