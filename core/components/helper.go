package components

import (
	"github.com/sisu-network/dheart/db"
	ecsigning "github.com/sisu-network/tss-lib/ecdsa/signing"
)

func GetMokDbForAvailManager(presignPids, pids []string) db.Database {
	return &db.MockDatabase{
		GetAvailablePresignShortFormFunc: func() ([]string, []string, error) {
			return presignPids, pids, nil
		},

		LoadPresignFunc: func(presignIds []string) ([]*ecsigning.SignatureData_OneRoundData, error) {
			return make([]*ecsigning.SignatureData_OneRoundData, len(presignIds)), nil
		},
	}
}
