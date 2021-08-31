package db

import (
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

type Database interface {
	FindPresignDataByPids(count int, pids tss.SortedPartyIDs) []*presign.LocalPresignData
}

// SqlDatabase implements Database interface.
type SqlDatabase struct {
}

func NewDatabase() Database {
	return &SqlDatabase{}
}

func (database *SqlDatabase) FindPresignDataByPids(count int, pids tss.SortedPartyIDs) []*presign.LocalPresignData {
	return nil
}
