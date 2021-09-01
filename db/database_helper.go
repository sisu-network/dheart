package db

import (
	"sort"
	"strings"

	"github.com/sisu-network/tss-lib/tss"
)

func getPidString(pids []*tss.PartyID) string {
	idArr := make([]string, 0)
	for _, p := range pids {
		idArr = append(idArr, p.Id)
	}
	// Sort the id array
	sort.Strings(idArr)

	return strings.Join(idArr, ",")
}

func getQueryQuestionMark(rowCount, fieldCount int) string {
	s := ""

	for i := 0; i < rowCount; i++ {
		q := ""
		for j := 0; j < fieldCount; j++ {
			q = q + "?"
			if j < fieldCount-1 {
				q = q + ", "
			}
		}

		s = s + "(" + q + ")"
		if i < rowCount-1 {
			s = s + ", "
		}
	}

	return s
}
