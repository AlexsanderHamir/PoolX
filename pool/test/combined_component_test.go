package test

import (
	"testing"

	"github.com/AlexsanderHamir/PoolX/pool"
	"github.com/stretchr/testify/require"
)

func TestPoolConcurrency(t *testing.T) {
	t.Run("Growth Blocked", func(t *testing.T) {
		availableItems := 500
		numGoroutines := 1000
		attempts := 5
		testGrowthBlocked(t, availableItems, numGoroutines, attempts)
	})

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

func testGrowthBlocked(t *testing.T, availableItems, numGoroutines, attempts int) {
	config := createConfig(t, availableItems, availableItems, attempts, false)

	t.Run("sync mode", func(t *testing.T) {
		readBlockersTest(t, config, numGoroutines, availableItems, false)
	})

	t.Run("async mode", func(t *testing.T) {
		readBlockersTest(t, config, numGoroutines, availableItems, true)
	})
}

// There's no items in the ring buffer, all of them go to the L1 cache first
// so we access the slow path directly which triggers the pre-read block hook.
func TestPreReadBlockHook(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(2).
		SetMinShrinkCapacity(2).
		SetRingBufferBlocking(true).
		SetHardLimit(2).
		SetPreReadBlockHookAttempts(3).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)

	obj, err := p.SlowPath()
	require.NoError(t, err)
	require.NotNil(t, obj)

}
