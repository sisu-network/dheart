package core

import (
	"crypto"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/dheart/worker/types"
)

func GetPresignWorkId(workType types.WorkType, nodes []*Node) string {
	var prefix string
	switch workType {
	case types.EcdsaPresign:
		prefix = "ecdsa_presign"
	case types.EddsaPresign:
		prefix = "eddsa_presign"
	default:
		utils.LogCritical("Invalid presign work type")
		return ""
	}

	digester := crypto.MD5.New()
	for _, node := range nodes {
		fmt.Fprint(digester, node.PartyId.Id)
		fmt.Fprint(digester, node)
	}
	hash := hex.EncodeToString(digester.Sum(nil))

	return prefix + "-" + hash
}

func GetKeysignWorkId(workType types.WorkType, txs [][]byte, block int64, chain string) string {
	var prefix string
	switch workType {
	case types.EcdsaSigning:
		prefix = "ecdsa_signing"
	case types.EddsaSigning:
		prefix = "eddsa_signing"
	default:
		utils.LogCritical("Invalid keygen work type, workType =", workType)
		return ""
	}

	digester := crypto.MD5.New()
	for _, tx := range txs {
		fmt.Fprint(digester, tx)
	}
	hash := hex.EncodeToString(digester.Sum(nil))

	return prefix + "-" + strconv.FormatInt(block, 10) + "-" + chain + "-" + hash
}
