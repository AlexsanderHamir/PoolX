package test

import (
	"testing"
	"time"

	"github.com/AlexsanderHamir/PoolX/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Negative or zero values should be ignored
func TestAllocationStrategyBasic(t *testing.T) {
	tests := []struct {
		name          string
		allocPercent  int
		allocAmount   int
		initialCap    int
		hardLimit     int
		expectedError bool
	}{
		{
			name:          "valid allocation strategy",
			allocPercent:  50,
			allocAmount:   10,
			initialCap:    100,
			hardLimit:     200,
			expectedError: false,
		},
		{
			name:          "zero alloc percent",
			allocPercent:  0,
			allocAmount:   10,
			initialCap:    100,
			hardLimit:     200,
			expectedError: false,
		},
		{
			name:          "negative alloc percent",
			allocPercent:  -1,
			allocAmount:   10,
			initialCap:    100,
			hardLimit:     200,
			expectedError: false,
		},
		{
			name:          "zero alloc amount",
			allocPercent:  50,
			allocAmount:   0,
			initialCap:    100,
			hardLimit:     200,
			expectedError: false,
		},
		{
			name:          "negative alloc amount",
			allocPercent:  50,
			allocAmount:   -1,
			initialCap:    100,
			hardLimit:     200,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := pool.NewPoolConfigBuilder().
				SetInitialCapacity(tt.initialCap).
				SetHardLimit(tt.hardLimit).
				SetAllocationStrategy(tt.allocPercent, tt.allocAmount).
				Build()

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestAllocationStrategyBehavior(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(10).
		SetHardLimit(100).
		SetAllocationStrategy(50, 5). // 50% allocation percent, 5 objects per allocation
		SetMinShrinkCapacity(10).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	// Test initial state
	stats := p.GetPoolStatsSnapshot()
	assert.Equal(t, 10, stats.CurrentCapacity)
	assert.Equal(t, uint64(0), stats.ObjectsInUse)
	assert.Equal(t, uint64(0), stats.TotalGets)
	assert.Equal(t, 5, stats.ObjectsCreated) // Initial allocation of 50% of capacity (10 * 50% = 5)
	assert.Equal(t, 0, stats.ObjectsDestroyed)

	// Get objects to trigger allocation
	objects := make([]*TestObject, 20)
	for i := range 20 {
		objects[i], err = p.Get()
		require.NoError(t, err)
		require.NotNil(t, objects[i])
	}

	// Verify allocation behavior
	stats = p.GetPoolStatsSnapshot()
	assert.True(t, p.IsGrowth())
	assert.Equal(t, uint64(20), stats.ObjectsInUse) // Should have 20 objects in use
	assert.Equal(t, uint64(20), stats.TotalGets)    // Should have 20 total gets
	assert.Equal(t, 20, stats.ObjectsCreated)       // Should have created 20 objects (initial 5 + 3 allocations of 5 each)
	assert.Equal(t, 0, stats.ObjectsDestroyed)      // No objects should be destroyed yet

	// Return all objects
	for _, obj := range objects {
		err := p.Put(obj)
		require.NoError(t, err)
	}

	// Verify no additional objects were created or destroyed
	stats = p.GetPoolStatsSnapshot()
	assert.Equal(t, uint64(0), stats.ObjectsInUse)                        // All objects should be returned
	assert.Equal(t, uint64(20), stats.TotalGets)                          // Total gets should remain the same
	assert.Equal(t, uint64(20), stats.FastReturnHit+stats.FastReturnMiss) // All objects should be returned
	assert.Equal(t, 20, stats.ObjectsCreated)                             // No new objects should be created
	assert.Equal(t, 0, stats.ObjectsDestroyed)                            // No objects should be destroyed
}

func TestAllocationStrategyWithShrink(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(50).
		SetHardLimit(100).
		SetAllocationStrategy(50, 5). // 50% allocation percent, 5 objects per allocation
		SetMinShrinkCapacity(10).
		SetShrinkCheckInterval(10 * time.Millisecond).
		SetShrinkCooldown(10 * time.Millisecond).
		SetMinUtilizationBeforeShrink(90).  // Shrink if utilization below 10%
		SetStableUnderutilizationRounds(1). // Only need 1 round of underutilization
		SetShrinkPercent(50).               // Shrink by 50%
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	// Test initial state
	stats := p.GetPoolStatsSnapshot()
	assert.Equal(t, 25, stats.ObjectsCreated) // Initial allocation of 50% of capacity (50 * 50% = 25)
	assert.Equal(t, 0, stats.ObjectsDestroyed)

	for range 10 {
		obj, err := p.Get()
		require.NoError(t, err)
		require.NotNil(t, obj)

		err = p.Put(obj)
		require.NoError(t, err)
	}

	// Wait for shrink to occur
	time.Sleep(100 * time.Millisecond)

	// Verify shrink and allocation behavior
	stats = p.GetPoolStatsSnapshot()
	assert.True(t, p.IsShrunk())                                          // Pool should have shrunk
	assert.Equal(t, uint64(0), stats.ObjectsInUse)                        // All objects should be returned
	assert.Equal(t, uint64(10), stats.TotalGets)                          // Should have 10 total gets
	assert.Equal(t, uint64(10), stats.FastReturnHit+stats.FastReturnMiss) // All objects should be returned
	assert.Equal(t, 25, stats.ObjectsCreated)                             // No new objects should be created
}
