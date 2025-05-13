package test

import (
	"testing"

	"github.com/AlexsanderHamir/PoolX/v2/pool"
	"github.com/stretchr/testify/require"
)

func TestPoolConcurrency(t *testing.T) {
	t.Run("Growth Allowed", func(t *testing.T) {
		runs := 40
		initial := 64
		hardLimit := 1000
		numGoroutines := 2000
		attempts := 5

		testGrowth(t, runs, hardLimit, initial, attempts, numGoroutines)
	})

	t.Run("Growth Allowed + High Contention", func(t *testing.T) {
		runs := 40
		initial := 1
		hardLimit := 1
		attempts := 1
		numGoroutines := 2000

		testGrowth(t, runs, hardLimit, initial, attempts, numGoroutines)
	})

	t.Run("Growth Allowed + High hard limit", func(t *testing.T) {
		runs := 40
		initial := 1
		hardLimit := 10000
		attempts := 3
		numGoroutines := 2000

		testGrowth(t, runs, hardLimit, initial, attempts, numGoroutines)
	})

}

// There's no items in the ring buffer, all of them go to the L1 cache first
// so we access the slow path directly which triggers the pre-read block hook.
func TestPreReadBlockHook(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder[*TestObject]().
		SetInitialCapacity(2).
		SetMinShrinkCapacity(2).
		SetRingBufferBlocking(true).
		SetHardLimit(2).
		SetPreReadBlockHookAttempts(3).
		SetAllocationStrategy(100, 2).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)

	obj, err := p.SlowPathGet()
	require.NoError(t, err)
	require.NotNil(t, obj)

}
