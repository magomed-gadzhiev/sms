// rollout_test.go
package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRollout_ZeroPercentNobodyIn(t *testing.T) {
	r := NewRollout(0)
	for i := 0; i < 1000; i++ {
		require.False(t, r.Enabled(uuid.New()))
	}
}

func TestRollout_HundredPercentEverybodyIn(t *testing.T) {
	r := NewRollout(100)
	for i := 0; i < 1000; i++ {
		require.True(t, r.Enabled(uuid.New()))
	}
}

func TestRollout_DeterministicForSameID(t *testing.T) {
	id := uuid.New()
	r := NewRollout(50)
	first := r.Enabled(id)
	for i := 0; i < 100; i++ {
		require.Equal(t, first, r.Enabled(id))
	}
}

func TestRollout_ApproxPercentage(t *testing.T) {
	r := NewRollout(25)
	in := 0
	n := 10000
	for i := 0; i < n; i++ {
		if r.Enabled(uuid.New()) {
			in++
		}
	}
	ratio := float64(in) / float64(n)
	require.InDelta(t, 0.25, ratio, 0.03, "expected ~25%%, got %.3f", ratio)
}

func TestRollout_ClampsNegativeAndOver100(t *testing.T) {
	require.False(t, NewRollout(-5).Enabled(uuid.New()))
	require.True(t, NewRollout(150).Enabled(uuid.New()))
}
