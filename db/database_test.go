package db

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	ecsigning "github.com/sisu-network/tss-lib/ecdsa/signing"
	"github.com/sisu-network/tss-lib/tss"

	libchain "github.com/sisu-network/lib/chain"
)

// TODO: Replace all db mock by in-memory db.
func TestSqlDatabase_SaveEcKeygen(t *testing.T) {
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

	assert.NoError(t, sqlDatabase.SaveEcKeygen(chain, wordId, pids, presignData))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSqlDatabase_LoadEcKeygen(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	sqlDatabase := SqlDatabase{
		db: db,
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	data := keygen.LocalPartySaveData{
		Ks: []*big.Int{big.NewInt(10)},
	}

	json, err := json.Marshal(data)
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"keygen_output"}).AddRow(json)
	mock.ExpectQuery("SELECT keygen_output FROM keygen WHERE key_type=\\? AND batch_index=0").
		WithArgs(libchain.KEY_TYPE_ECDSA).
		WillReturnRows(rows).
		WillReturnError(nil)

	got, err := sqlDatabase.LoadEcKeygen(libchain.KEY_TYPE_ECDSA)
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

	wordId := "work0"
	pids := []*tss.PartyID{{
		MessageWrapper_PartyID: &tss.MessageWrapper_PartyID{
			Id: "party-0",
		},
	}}
	presignData := []*ecsigning.SignatureData_OneRoundData{
		{
			PartyId: "partyId",
		},
	}

	json, err := json.Marshal(presignData[0])
	assert.NoError(t, err)

	mock.ExpectExec("INSERT INTO presign").
		WithArgs("work0-0", wordId, "party-0", 0, PresignStatusNotUsed, json).
		WillReturnResult(sqlmock.NewResult(1, 1)).
		WillReturnError(nil)

	assert.NoError(t, sqlDatabase.SavePresignData(wordId, pids, presignData))
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

	presignData := []*ecsigning.SignatureData_OneRoundData{
		{
			PartyId: "partyId1",
		},
		{
			PartyId: "partyId2",
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

func TestSqlDatabase_LoadPresignStatus(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	sqlDatabase := SqlDatabase{
		db: db,
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	rows := sqlmock.NewRows([]string{"status"}).AddRow("used").AddRow("not_used")
	presignIDs := []string{"presign0", "presign1"}

	mock.ExpectQuery("SELECT status FROM presign WHERE presign_id IN \\(\\?, \\?\\) ORDER BY created_time DESC").
		WithArgs(presignIDs[0], presignIDs[1]).
		WillReturnRows(rows).
		WillReturnError(nil)

	got, err := sqlDatabase.LoadPresignStatus(presignIDs)
	assert.NoError(t, err)
	assert.EqualValues(t, []string{"used", "not_used"}, got)

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

	preparams := keygen.LocalPreParams{
		P: big.NewInt(10),
		Q: big.NewInt(10),
	}

	preparamsJS, err := json.Marshal(&preparams)
	assert.NoError(t, err)

	mock.ExpectExec("INSERT INTO preparams").
		WithArgs(libchain.KEY_TYPE_ECDSA, preparamsJS).
		WillReturnResult(sqlmock.NewResult(1, 1)).
		WillReturnError(nil)

	assert.NoError(t, sqlDatabase.SavePreparams(&preparams))
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

	preparams := keygen.LocalPreParams{
		P: big.NewInt(10),
		Q: big.NewInt(10),
	}

	preparamsJS, err := json.Marshal(&preparams)
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"preparams"}).AddRow(preparamsJS)
	mock.ExpectQuery("SELECT preparams FROM preparams WHERE key_type=\\?").
		WithArgs(libchain.KEY_TYPE_ECDSA).
		WillReturnRows(rows).
		WillReturnError(nil)

	got, err := sqlDatabase.LoadPreparams()
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
