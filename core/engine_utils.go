package core

import (
	"crypto"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/sisu-network/dheart/worker/types"
)

func getWorkId(workType types.WorkType, block int64, chain string, index int, nodes []*Node) string {
	var prefix string
	switch workType {
	case types.ECDSA_KEYGEN:
		prefix = "ecdsa_keygen"
	case types.ECDSA_PRESIGN:
		prefix = "ecdsa_presign"
	case types.ECDSA_SIGNING:
		prefix = "ecdsa_signing"
	case types.EDDSA_KEYGEN:
		prefix = "eddsa_keygen"
	case types.EDDSA_PRESIGN:
		prefix = "eddsa_presign"
	case types.EDDSA_SIGNING:
		prefix = "eddsa_signing"
	}

	digester := crypto.MD5.New()
	for _, node := range nodes {
		fmt.Fprint(digester, node.PartyId.Id)
		fmt.Fprint(digester, node)
	}
	hash := hex.EncodeToString(digester.Sum(nil))

	return prefix + "-" + strconv.FormatInt(block, 10) + "-" + chain + "-" + strconv.FormatInt(int64(index), 10) + "-" + hash
}
