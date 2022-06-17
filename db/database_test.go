package db

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	ecsigning "github.com/sisu-network/tss-lib/ecdsa/signing"
	"github.com/sisu-network/tss-lib/tss"
)

func TestSqlDatabase_SaveEcKeygen(t *testing.T) {
	t.Parallel()

	dbConfig := config.GetLocalhostDbConfig()
	dbConfig.Schema = "dheart"
	dbConfig.InMemory = true

	dbInstance := NewDatabase(&dbConfig)
	dbInstance.Init()

	pids := []*tss.PartyID{{
		MessageWrapper_PartyID: &tss.MessageWrapper_PartyID{
			Id: "party-0",
		},
	}}
	err := dbInstance.SaveEcKeygen("ecdsa", "keygen0", pids, &keygen.LocalPartySaveData{
		LocalPreParams: keygen.LocalPreParams{
			P: big.NewInt(10),
			Q: big.NewInt(20),
		},
	})
	require.Nil(t, err)

	keygenOutput, err := dbInstance.LoadEcKeygen("ecdsa")
	require.Nil(t, err)
	require.NotNil(t, keygenOutput)

	require.Equal(t, keygenOutput.LocalPreParams.P, big.NewInt(10))
	require.Equal(t, keygenOutput.LocalPreParams.Q, big.NewInt(20))
}

func TestSqlDatabase_SavePresignData(t *testing.T) {
	t.Parallel()

	dbConfig := config.GetLocalhostDbConfig()
	dbConfig.Schema = "dheart"
	dbConfig.InMemory = true

	dbInstance := NewDatabase(&dbConfig)
	dbInstance.Init()

	pids := []*tss.PartyID{{
		MessageWrapper_PartyID: &tss.MessageWrapper_PartyID{
			Id: "party0",
		},
	}}
	mockKi := []byte("mockKI")
	presignData := []*ecsigning.SignatureData_OneRoundData{
		{
			PartyId: "party0",
			KI:      mockKi,
		},
	}

	err := dbInstance.SavePresignData("presign", pids, presignData)
	require.Nil(t, err)

	presigns, err := dbInstance.LoadPresign([]string{"presign-0"})
	require.Nil(t, err)
	require.Equal(t, 1, len(presigns))
	require.Equal(t, mockKi, presigns[0].KI)

	availPresigns, loadedPids, err := dbInstance.GetAvailablePresignShortForm()
	require.Nil(t, err)
	require.Equal(t, []string{"presign-0"}, availPresigns)
	require.Equal(t, []string{"party0"}, loadedPids)
}

func TestSqlDatabase_LoadPresignStatus(t *testing.T) {
	t.Parallel()

	dbConfig := config.GetLocalhostDbConfig()
	dbConfig.Schema = "dheart"
	dbConfig.InMemory = true

	dbInstance := NewDatabase(&dbConfig)
	dbInstance.Init()

	pids := []*tss.PartyID{{
		MessageWrapper_PartyID: &tss.MessageWrapper_PartyID{
			Id: "party0",
		},
	}}
	mockKi := []byte("mockKI")
	presignData := []*ecsigning.SignatureData_OneRoundData{
		{
			PartyId: "party0",
			KI:      mockKi,
		},
	}

	err := dbInstance.SavePresignData("presign", pids, presignData)
	require.Nil(t, err)

	availPresigns, _, err := dbInstance.GetAvailablePresignShortForm()
	require.Nil(t, err)
	require.Equal(t, 1, len(availPresigns))

	err = dbInstance.UpdatePresignStatus([]string{"presign-0"})
	require.Nil(t, err)

	availPresigns, _, err = dbInstance.GetAvailablePresignShortForm()
	require.Nil(t, err)
	require.Equal(t, 0, len(availPresigns))
}

func TestSqlDatabase_SavePreparams(t *testing.T) {
	t.Parallel()

	dbConfig := config.GetLocalhostDbConfig()
	dbConfig.Schema = "dheart"
	dbConfig.InMemory = true

	dbInstance := NewDatabase(&dbConfig)
	dbInstance.Init()

	err := dbInstance.SavePreparams(&keygen.LocalPreParams{
		P: big.NewInt(10),
		Q: big.NewInt(20),
	})
	require.Nil(t, err)

	preparams, err := dbInstance.LoadPreparams()
	require.Nil(t, err)
	require.Equal(t, big.NewInt(10), preparams.P)
	require.Equal(t, big.NewInt(20), preparams.Q)
}
