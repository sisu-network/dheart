package worker

// func ChooseLeader(workId string, parties []*tss.PartyID) *tss.PartyID {
// 	keyStore := make(map[string]int)
// 	sortedHashes := make([]string, len(parties))

// 	for i, party := range parties {
// 		sum := sha256.Sum256([]byte(party.Id + workId))
// 		encodedSum := hex.EncodeToString(sum[:])

// 		keyStore[encodedSum] = i
// 		sortedHashes[i] = encodedSum
// 	}

// 	sort.Strings(sortedHashes)

// 	return parties[keyStore[sortedHashes[0]]]
// }
