package utils

import (
	"sort"
	"strings"

	"github.com/sisu-network/tss-lib/tss"
)

func GetPidString(pids []*tss.PartyID) string {
	idArr := make([]string, 0)
	for _, p := range pids {
		idArr = append(idArr, p.Id)
	}
	// Sort the id array
	sort.Strings(idArr)

	return strings.Join(idArr, ",")
}

func GetPidsArray(pids []*tss.PartyID) []string {
	idArr := make([]string, 0)
	for _, p := range pids {
		idArr = append(idArr, p.Id)
	}
	// Sort the id array
	sort.Strings(idArr)

	return idArr
}

func GetPartyIdFromString(pid string, allParties []*tss.PartyID) *tss.PartyID {
	for _, p := range allParties {
		if p.Id == pid {
			return p
		}
	}

	return nil
}
