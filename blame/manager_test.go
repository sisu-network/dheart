package blame

import (
	"sort"
	"testing"

	"github.com/sisu-network/tss-lib/tss"
	"github.com/stretchr/testify/assert"
)

func TestManager_GetRoundCulprits(t *testing.T) {
	t.Parallel()

	allParties := map[string]*tss.PartyID{
		"0": {
			MessageWrapper_PartyID: &tss.MessageWrapper_PartyID{Id: "0"},
		},
		"1": {
			MessageWrapper_PartyID: &tss.MessageWrapper_PartyID{Id: "1"},
		},
		"2": {
			MessageWrapper_PartyID: &tss.MessageWrapper_PartyID{Id: "2"},
		},
	}
	manager := NewManager()
	manager.AddSender("work", 0, "0")
	manager.AddCulpritByRound("work", 0, []*tss.PartyID{allParties["1"]})

	culprits := manager.GetRoundCulprits("work", 0, allParties)
	assert.Len(t, culprits, 2)
	ids := []string{culprits[0].Id, culprits[1].Id}
	sort.Strings(ids)
	assert.Equal(t, []string{"1", "2"}, ids)
}
