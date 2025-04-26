package test

import (
	"github.com/AlexsanderHamir/memory_context/pool"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPoolConcurrency(t *testing.T) {

	t.Run("read blockers sync", func(t *testing.T) {
		availableItems := 500
		numGoroutines := 1000
		attempts := 5

		config := createConfig(t, availableItems, availableItems, attempts) // doesn't grow
		readBlockersTest(t, config, numGoroutines, availableItems, false)
	})

	t.Run("read blockers async", func(t *testing.T) {
		availableItems := 500
		numGoroutines := 1000
		attempts := 5

		config := createConfig(t, availableItems, availableItems, attempts) // doesn't grow
		readBlockersTest(t, config, numGoroutines, availableItems, true)
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
