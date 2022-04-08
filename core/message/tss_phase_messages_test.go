package message

import (
	"testing"

	wTypes "github.com/sisu-network/dheart/worker/types"
	"github.com/stretchr/testify/require"
)

func TestGetAllMessageTypesByRound(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"KGRound1Message"}, GetAllMessageTypesByRound(Keygen1))
	require.Equal(t, []string{"KGRound2Message1", "KGRound2Message2"}, GetAllMessageTypesByRound(Keygen2))
	require.Equal(t, []string{"KGRound3Message"}, GetAllMessageTypesByRound(Keygen3))
	require.Equal(t, []string{"PresignRound1Message1", "PresignRound1Message2"}, GetAllMessageTypesByRound(Presign1))
	require.Equal(t, []string{"PresignRound2Message"}, GetAllMessageTypesByRound(Presign2))
	require.Equal(t, []string{"PresignRound3Message"}, GetAllMessageTypesByRound(Presign3))
	require.Equal(t, []string{"PresignRound4Message"}, GetAllMessageTypesByRound(Presign4))
	require.Equal(t, []string{"SignRound1Message"}, GetAllMessageTypesByRound(Sign1))
}

func TestConvertTSSRoundToDheartRound(t *testing.T) {
	t.Parallel()

	require.Equal(t, Keygen1, ConvertTSSRoundToDheartRound(1, wTypes.EcdsaKeygen))
	require.Equal(t, Keygen2, ConvertTSSRoundToDheartRound(2, wTypes.EcdsaKeygen))
	require.Equal(t, Keygen3, ConvertTSSRoundToDheartRound(3, wTypes.EcdsaKeygen))
	require.Equal(t, Presign1, ConvertTSSRoundToDheartRound(1, wTypes.EcdsaPresign))
	require.Equal(t, Presign2, ConvertTSSRoundToDheartRound(2, wTypes.EcdsaPresign))
	require.Equal(t, Presign3, ConvertTSSRoundToDheartRound(3, wTypes.EcdsaPresign))
	require.Equal(t, Presign4, ConvertTSSRoundToDheartRound(4, wTypes.EcdsaPresign))
	require.Equal(t, Sign1, ConvertTSSRoundToDheartRound(1, wTypes.EcdsaSigning))
}
