package db

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

func TestSqlDatabase_SaveKeygenData(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	sqlDatabase := SqlDatabase{
		db: db,
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	chain := "chain-0"
	wordId := "work0"
	pids := []*tss.PartyID{{
		MessageWrapper_PartyID: &tss.MessageWrapper_PartyID{
			Id: "party-0",
		},
	}}
	presignData := []*keygen.LocalPartySaveData{
		{
			H1j: []*big.Int{big.NewInt(10)},
		},
	}

	json, err := json.Marshal(presignData[0])
	assert.NoError(t, err)

	mock.ExpectExec("INSERT INTO keygen").
		WithArgs(chain, wordId, "party-0", 0, json).
		WillReturnResult(sqlmock.NewResult(1, 1)).
		WillReturnError(nil)

	assert.NoError(t, sqlDatabase.SaveKeygenData(chain, wordId, pids, presignData))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSqlDatabase_LoadKeygenData(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	sqlDatabase := SqlDatabase{
		db: db,
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	chain := "chain-0"
	workId := "work0"

	data := keygen.LocalPartySaveData{
		Ks: []*big.Int{big.NewInt(10)},
	}

	json, err := json.Marshal(data)
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"keygen_output"}).AddRow(json)
	mock.ExpectQuery("SELECT keygen_output FROM keygen WHERE chain=\\? AND work_id=\\? AND batch_index=0").
		WithArgs(chain, workId).
		WillReturnRows(rows).
		WillReturnError(nil)

	got, err := sqlDatabase.LoadKeygenData(chain, workId)
	assert.NoError(t, err)
	assert.EqualValues(t, data, *got)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSqlDatabase_SavePresignData(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	sqlDatabase := SqlDatabase{
		db: db,
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	chain := "chain-0"
	wordId := "work0"
	pids := []*tss.PartyID{{
		MessageWrapper_PartyID: &tss.MessageWrapper_PartyID{
			Id: "party-0",
		},
	}}
	presignData := []*presign.LocalPresignData{
		{
			W: big.NewInt(10),
			K: big.NewInt(10),
		},
	}

	json, err := json.Marshal(presignData[0])
	assert.NoError(t, err)

	mock.ExpectExec("INSERT INTO presign").
		WithArgs("work0-0", chain, wordId, "party-0", 0, PresignStatusNotUsed, json).
		WillReturnResult(sqlmock.NewResult(1, 1)).
		WillReturnError(nil)

	assert.NoError(t, sqlDatabase.SavePresignData(chain, wordId, pids, presignData))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSqlDatabase_LoadPresign(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	sqlDatabase := SqlDatabase{
		db: db,
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	presignData := []*presign.LocalPresignData{
		{
			W: big.NewInt(10),
			K: big.NewInt(10),
		},
		{
			W: big.NewInt(10),
			K: big.NewInt(10),
		},
	}

	json1, err := json.Marshal(&presignData[0])
	assert.NoError(t, err)

	json2, err := json.Marshal(&presignData[1])
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"presign_output"}).AddRow(json1).AddRow(json2)
	presignIDs := []string{"presign0", "presign1"}

	mock.ExpectQuery("SELECT presign_output FROM presign WHERE presign_id IN \\(\\?, \\?\\) ORDER BY created_time DESC").
		WithArgs(presignIDs[0], presignIDs[1]).
		WillReturnRows(rows).
		WillReturnError(nil)

	got, err := sqlDatabase.LoadPresign(presignIDs)
	assert.NoError(t, err)
	assert.EqualValues(t, presignData, got)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSqlDatabase_SavePreparams(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	sqlDatabase := SqlDatabase{
		db: db,
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	chain := "chain-0"
	preparams := keygen.LocalPreParams{
		P: big.NewInt(10),
		Q: big.NewInt(10),
	}

	preparamsJS, err := json.Marshal(&preparams)
	assert.NoError(t, err)

	mock.ExpectExec("INSERT INTO preparams").
		WithArgs(chain, preparamsJS).
		WillReturnResult(sqlmock.NewResult(1, 1)).
		WillReturnError(nil)

	assert.NoError(t, sqlDatabase.SavePreparams(chain, &preparams))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSqlDatabase_LoadPreparams(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	sqlDatabase := SqlDatabase{
		db: db,
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	chain := "chain-0"
	preparams := keygen.LocalPreParams{
		P: big.NewInt(10),
		Q: big.NewInt(10),
	}

	preparamsJS, err := json.Marshal(&preparams)
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"preparams"}).AddRow(preparamsJS)
	mock.ExpectQuery("SELECT preparams FROM preparams WHERE chain=\\?").
		WithArgs(chain).
		WillReturnRows(rows).
		WillReturnError(nil)

	got, err := sqlDatabase.LoadPreparams(chain)
	assert.NoError(t, err)
	assert.EqualValues(t, preparams, *got)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSqlDatabase_GetAvailablePresignShortForm(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	sqlDatabase := SqlDatabase{
		db: db,
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	rows := sqlmock.NewRows([]string{"presign_id", "pids_string"}).AddRow("presign0", "1,2,3")
	mock.ExpectQuery("SELECT presign_id, pids_string FROM presign WHERE status='not_used'").
		WillReturnRows(rows).
		WillReturnError(nil)

	presignIDs, pidStrings, err := sqlDatabase.GetAvailablePresignShortForm()
	assert.NoError(t, err)
	assert.Len(t, presignIDs, 1)
	assert.Equal(t, "presign0", presignIDs[0])
	assert.Len(t, pidStrings, 1)
	assert.Equal(t, "1,2,3", pidStrings[0])

	assert.NoError(t, mock.ExpectationsWereMet())
}
