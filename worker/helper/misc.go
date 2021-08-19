package helper

import "github.com/sisu-network/tss-lib/tss"

func GetFromPid(fromString string, pids []*tss.PartyID) *tss.PartyID {
	for _, pid := range pids {
		if pid.Id == fromString {
			return pid
		}
	}

	return nil
}
