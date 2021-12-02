package main

import (
	"fmt"
	"math/big"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"

	libchain "github.com/sisu-network/lib/chain"
)

func testInsertingKeygenData(database db.Database) {
	chain := "eth"
	workId := "testwork"

	pids := []*tss.PartyID{
		{
			MessageWrapper_PartyID: &tss.MessageWrapper_PartyID{
				Id: "First ID",
			},
		},
	}
	P := big.NewInt(1)
	output := []*keygen.LocalPartySaveData{
		{
			LocalPreParams: keygen.LocalPreParams{
				P: P,
			},
		},
	}

	// Write data
	err := database.SaveKeygenData(chain, workId, pids, output)
	if err != nil {
		panic(err)
	}

	// Read data and do sanity check
	loaded, err := database.LoadKeygenData(libchain.GetKeyTypeForChain(chain))
	if err != nil {
		panic(err)
	}

	if loaded.LocalPreParams.P.Cmp(P) != 0 {
		panic(fmt.Errorf("P number does not match"))
	}

	// Remove the row from database.
	err = (database.(*db.SqlDatabase)).DeleteKeygenWork(workId)
	if err != nil {
		panic(err)
	}
}

func testInsertingPresignData(database db.Database) {
	// Test inserting presignOutput
	workId := "testwork"
	pids := []*tss.PartyID{
		{
			MessageWrapper_PartyID: &tss.MessageWrapper_PartyID{
				Id: "First ID",
			},
		},
	}
	output := []*presign.LocalPresignData{
		{
			PartyIds: pids,
		},
		{
			PartyIds: pids,
		},
	}

	err := database.SavePresignData("eth", workId, pids, output)
	if err != nil {
		panic(err)
	}

	// Data data
	presignIds, _, err := database.GetAvailablePresignShortForm()
	if err != nil {
		panic(err)
	}

	if len(presignIds) != 2 {
		panic(fmt.Errorf("Length of rows should be 2. Actual: %d", len(presignIds)))
	}

	// Remove the row from database.
	err = (database.(*db.SqlDatabase)).DeletePresignWork(workId)
	if err != nil {
		panic(err)
	}

	log.Verbose("Test passed")
}

func main() {
	config := &config.DbConfig{
		Port:          3306,
		Host:          "localhost",
		Username:      "root",
		Password:      "password",
		Schema:        "dheart",
		MigrationPath: "file://../../../db/migrations/",
	}

	database := db.NewDatabase(config)
	err := database.Init()
	if err != nil {
		panic(err)
	}

	testInsertingKeygenData(database)
	testInsertingPresignData(database)
}
