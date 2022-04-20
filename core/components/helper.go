package components

import (
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/worker/helper"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
)

func GetMokDbForAvailManager(presignPids, pids []string) db.Database {
	return &helper.MockDatabase{
		GetAvailablePresignShortFormFunc: func() ([]string, []string, error) {
			return presignPids, pids, nil
		},

		LoadPresignFunc: func(presignIds []string) ([]*presign.LocalPresignData, error) {
			return make([]*presign.LocalPresignData, len(presignIds)), nil
		},
	}
}
