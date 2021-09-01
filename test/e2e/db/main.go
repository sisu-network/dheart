package main

import (
	"fmt"

	"github.com/sisu-network/dheart/db"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

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

	// TODO: read data
	ids, _, _, err := database.GetAvailablePresignShortForm()
	if err != nil {
		panic(err)
	}

	if len(ids) != 2 {
		panic(fmt.Errorf("Length of rows should be 2. Actual: %d", len(ids)))
	}

	// Remove the row from database.
	err = (database.(*db.SqlDatabase)).DeletePresignWork(workId)
	if err != nil {
		panic(err)
	}

	utils.LogVerbose("Test passed")
}

func main() {
	config := &db.SqlDbConfig{
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

	testInsertingPresignData(database)
}
