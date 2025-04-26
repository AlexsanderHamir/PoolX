package test

import (
	"testing"

	"github.com/AlexsanderHamir/memory_context/src/pool"
	"github.com/stretchr/testify/require"
)

func TestPoolConcurrency(t *testing.T) {

	t.Run("Growth Blocked (read blockers accounting)", func(t *testing.T) {
		availableItems := 500
		numGoroutines := 1000
		attempts := 5

		config := createConfig(t, availableItems, availableItems, attempts, false)

		t.Run("sync mode", func(t *testing.T) {
			readBlockersTest(t, config, numGoroutines, availableItems, false)
		})

		t.Run("async mode", func(t *testing.T) {
			readBlockersTest(t, config, numGoroutines, availableItems, true)
		})
	})

	t.Run("Growth Allowed + small Hard Limit (high contention)", func(t *testing.T) {
		hardLimit := 150
		numGoroutines := 300
		attempts := 5
		initial := 1

		config := createConfig(t, hardLimit, initial, attempts, false)
		hardLimitTest(t, config, numGoroutines, hardLimit, true)
	})

}

// There's no items in the ring buffer, all of them go to the L1 cache if it fits
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
