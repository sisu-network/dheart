package tools

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCircularQueue(t *testing.T) {
	q := NewCircularQueue(5)

	for i := 1; i <= 5; i++ {
		q.Add(fmt.Sprintf("%d", i), i)
	}

	for i := 1; i <= 5; i++ {
		require.Equal(t, i, q.Get(fmt.Sprintf("%d", i)))
	}
	require.Equal(t, nil, q.Get("6"))

	// Insert another element to overwrite the first one
	q.Add("6", 6)
	require.Equal(t, 6, q.Get("6"))
	require.Equal(t, nil, q.Get("1"))
}
