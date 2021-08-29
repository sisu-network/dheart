package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"

	"github.com/sisu-network/tss-lib/tss"
)

func ChooseLeader(block int64, chain string, index int, parties []*tss.PartyID) *tss.PartyID {
	keyStore := make(map[string]int)
	sortedHashes := make([]string, len(parties))

	extra := strconv.FormatInt(block, 10) + chain + strconv.FormatInt(int64(index), 10)
	for i, party := range parties {
		sum := sha256.Sum256([]byte(party.Id + extra))
		encodedSum := hex.EncodeToString(sum[:])
		keyStore[encodedSum] = i
		sortedHashes[i] = encodedSum
	}

	sort.Strings(sortedHashes)

	return parties[keyStore[sortedHashes[0]]]
}
