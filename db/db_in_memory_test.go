package db

import "testing"

func TestInMemory_Peers(t *testing.T) {
	testPeers(t, true)
}
