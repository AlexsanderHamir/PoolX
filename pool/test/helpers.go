package test

import (
	"context"
	"fmt"
	"memctx/pool"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testObject is a simple test object used in pool tests
type testObject struct {
	value int
}

// DefaultConfigValues stores all default configuration values
type DefaultConfigValues struct {
	InitialCapacity                    int
	HardLimit                          int
	GrowthPercent                      float64
	FixedGrowthFactor                  float64
	ExponentialThresholdFactor         float64
	ShrinkAggressiveness               pool.AggressivenessLevel
	ShrinkCheckInterval                time.Duration
	IdleThreshold                      time.Duration
	MinIdleBeforeShrink                int
	ShrinkCooldown                     time.Duration
	MinUtilizationBeforeShrink         float64
	StableUnderutilizationRounds       int
	ShrinkPercent                      float64
	MinShrinkCapacity                  int
	MaxConsecutiveShrinks              int
	InitialSize                        int
	FillAggressiveness                 float64
	RefillPercent                      float64
	EnableChannelGrowth                bool
	GrowthEventsTrigger                int
	FastPathGrowthPercent              float64
	FastPathExponentialThresholdFactor float64
	FastPathFixedGrowthFactor          float64
	ShrinkEventsTrigger                int
	FastPathShrinkAggressiveness       pool.AggressivenessLevel
	FastPathShrinkPercent              float64
	FastPathShrinkMinCapacity          int
	Verbose                            bool
	RingBufferBlocking                 bool
	ReadTimeout                        time.Duration
	WriteTimeout                       time.Duration
}

// storeDefaultConfigValues stores all default configuration values
func storeDefaultConfigValues(config pool.PoolConfig) DefaultConfigValues {
	return DefaultConfigValues{
		InitialCapacity:                    config.GetInitialCapacity(),
		HardLimit:                          config.GetHardLimit(),
		GrowthPercent:                      config.GetGrowth().GetGrowthPercent(),
		FixedGrowthFactor:                  config.GetGrowth().GetFixedGrowthFactor(),
		ExponentialThresholdFactor:         config.GetGrowth().GetExponentialThresholdFactor(),
		ShrinkAggressiveness:               config.GetShrink().GetAggressivenessLevel(),
		ShrinkCheckInterval:                config.GetShrink().GetCheckInterval(),
		IdleThreshold:                      config.GetShrink().GetIdleThreshold(),
		MinIdleBeforeShrink:                config.GetShrink().GetMinIdleBeforeShrink(),
		ShrinkCooldown:                     config.GetShrink().GetShrinkCooldown(),
		MinUtilizationBeforeShrink:         config.GetShrink().GetMinUtilizationBeforeShrink(),
		StableUnderutilizationRounds:       config.GetShrink().GetStableUnderutilizationRounds(),
		ShrinkPercent:                      config.GetShrink().GetShrinkPercent(),
		MinShrinkCapacity:                  config.GetShrink().GetMinCapacity(),
		MaxConsecutiveShrinks:              config.GetShrink().GetMaxConsecutiveShrinks(),
		InitialSize:                        config.GetFastPath().GetInitialSize(),
		FillAggressiveness:                 config.GetFastPath().GetFillAggressiveness(),
		RefillPercent:                      config.GetFastPath().GetRefillPercent(),
		EnableChannelGrowth:                config.GetFastPath().IsEnableChannelGrowth(),
		GrowthEventsTrigger:                config.GetFastPath().GetGrowthEventsTrigger(),
		FastPathGrowthPercent:              config.GetFastPath().GetGrowth().GetGrowthPercent(),
		FastPathExponentialThresholdFactor: config.GetFastPath().GetGrowth().GetExponentialThresholdFactor(),
		FastPathFixedGrowthFactor:          config.GetFastPath().GetGrowth().GetFixedGrowthFactor(),
		ShrinkEventsTrigger:                config.GetFastPath().GetShrinkEventsTrigger(),
		FastPathShrinkAggressiveness:       config.GetFastPath().GetShrink().GetAggressivenessLevel(),
		FastPathShrinkPercent:              config.GetFastPath().GetShrink().GetShrinkPercent(),
		FastPathShrinkMinCapacity:          config.GetFastPath().GetShrink().GetMinCapacity(),
		Verbose:                            config.IsVerbose(),
		RingBufferBlocking:                 config.GetRingBufferConfig().IsBlocking(),
		ReadTimeout:                        config.GetRingBufferConfig().GetReadTimeout(),
		WriteTimeout:                       config.GetRingBufferConfig().GetWriteTimeout(),
	}
}

// createCustomConfig creates a custom configuration with different values
func createCustomConfig(t *testing.T) (pool.PoolConfig, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(10000000028182820).
		SetGrowthPercent(0.5).
		SetFixedGrowthFactor(1.0).
		SetGrowthExponentialThresholdFactor(4.0).
		SetShrinkCheckInterval(2 * time.Second).
		SetIdleThreshold(5 * time.Second).
		SetMinIdleBeforeShrink(29).
		SetShrinkCooldown(10 * time.Second).
		SetMinUtilizationBeforeShrink(0.321).
		SetStableUnderutilizationRounds(3121).
		SetShrinkPercent(0.25121).
		SetMinShrinkCapacity(10121).
		SetMaxConsecutiveShrinks(3121).
		SetFastPathInitialSize(50121).
		SetFastPathFillAggressiveness(0.8121).
		SetFastPathRefillPercent(0.2121).
		SetFastPathEnableChannelGrowth(false).
		SetFastPathGrowthEventsTrigger(5121).
		SetFastPathGrowthPercent(0.6121).
		SetFastPathExponentialThresholdFactor(3.0121).
		SetFastPathFixedGrowthFactor(0.8121).
		SetFastPathShrinkEventsTrigger(4121).
		SetFastPathShrinkAggressiveness(pool.AggressivenessAggressive).
		SetFastPathShrinkPercent(0.3121).
		SetFastPathShrinkMinCapacity(20121).
		SetVerbose(true).
		SetRingBufferBlocking(true).
		WithTimeOut(5 * time.Second).
		SetRingBufferReadTimeout(5 * time.Second).
		SetRingBufferWriteTimeout(5 * time.Second).
		SetRingBufferCancel(ctx).
		Build()
	require.NoError(t, err)

	return config, cancel
}

// verifyCustomValuesDifferent verifies that custom values are different from default values
func verifyCustomValuesDifferent(t *testing.T, original DefaultConfigValues, custom pool.PoolConfig) {
	assert.NotEqual(t, original.InitialCapacity, custom.GetInitialCapacity())
	assert.NotEqual(t, original.HardLimit, custom.GetHardLimit())
	assert.NotEqual(t, original.GrowthPercent, custom.GetGrowth().GetGrowthPercent())
	assert.NotEqual(t, original.FixedGrowthFactor, custom.GetGrowth().GetFixedGrowthFactor())
	assert.NotEqual(t, original.ExponentialThresholdFactor, custom.GetGrowth().GetExponentialThresholdFactor())
	assert.NotEqual(t, original.ShrinkAggressiveness, custom.GetShrink().GetAggressivenessLevel())
	assert.NotEqual(t, original.ShrinkCheckInterval, custom.GetShrink().GetCheckInterval())
	assert.NotEqual(t, original.IdleThreshold, custom.GetShrink().GetIdleThreshold())
	assert.NotEqual(t, original.MinIdleBeforeShrink, custom.GetShrink().GetMinIdleBeforeShrink())
	assert.NotEqual(t, original.ShrinkCooldown, custom.GetShrink().GetShrinkCooldown())
	assert.NotEqual(t, original.MinUtilizationBeforeShrink, custom.GetShrink().GetMinUtilizationBeforeShrink())
	assert.NotEqual(t, original.StableUnderutilizationRounds, custom.GetShrink().GetStableUnderutilizationRounds())
	assert.NotEqual(t, original.ShrinkPercent, custom.GetShrink().GetShrinkPercent())
	assert.NotEqual(t, original.MinShrinkCapacity, custom.GetShrink().GetMinCapacity())
	assert.NotEqual(t, original.MaxConsecutiveShrinks, custom.GetShrink().GetMaxConsecutiveShrinks())
	assert.NotEqual(t, original.InitialSize, custom.GetFastPath().GetInitialSize())
	assert.NotEqual(t, original.FillAggressiveness, custom.GetFastPath().GetFillAggressiveness())
	assert.NotEqual(t, original.RefillPercent, custom.GetFastPath().GetRefillPercent())
	assert.NotEqual(t, original.EnableChannelGrowth, custom.GetFastPath().IsEnableChannelGrowth())
	assert.NotEqual(t, original.GrowthEventsTrigger, custom.GetFastPath().GetGrowthEventsTrigger())
	assert.NotEqual(t, original.FastPathGrowthPercent, custom.GetFastPath().GetGrowth().GetGrowthPercent())
	assert.NotEqual(t, original.FastPathExponentialThresholdFactor, custom.GetFastPath().GetGrowth().GetExponentialThresholdFactor())
	assert.NotEqual(t, original.FastPathFixedGrowthFactor, custom.GetFastPath().GetGrowth().GetFixedGrowthFactor())
	assert.NotEqual(t, original.ShrinkEventsTrigger, custom.GetFastPath().GetShrinkEventsTrigger())
	assert.NotEqual(t, original.FastPathShrinkAggressiveness, custom.GetFastPath().GetShrink().GetAggressivenessLevel())
	assert.NotEqual(t, original.FastPathShrinkPercent, custom.GetFastPath().GetShrink().GetShrinkPercent())
	assert.NotEqual(t, original.FastPathShrinkMinCapacity, custom.GetFastPath().GetShrink().GetMinCapacity())
	assert.NotEqual(t, original.Verbose, custom.IsVerbose())
	assert.NotEqual(t, original.RingBufferBlocking, custom.GetRingBufferConfig().IsBlocking())
	assert.NotEqual(t, original.ReadTimeout, custom.GetRingBufferConfig().GetReadTimeout())
	assert.NotEqual(t, original.WriteTimeout, custom.GetRingBufferConfig().GetWriteTimeout())
}

// createHardLimitTestConfig creates a configuration for hard limit testing
func createHardLimitTestConfig(t *testing.T, blocking bool) pool.PoolConfig {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(10).
		SetHardLimit(20).
		SetMinShrinkCapacity(10).
		SetRingBufferBlocking(blocking).
		SetVerbose(blocking). // Only verbose for blocking test
		Build()
	require.NoError(t, err)
	return config
}

// createTestPool creates a pool with the given configuration
func createTestPool(t *testing.T, config pool.PoolConfig) *pool.Pool[*testObject] {
	allocator := func() *testObject {
		return &testObject{value: 42}
	}

	cleaner := func(obj *testObject) {
		obj.value = 0
	}

	internalConfig := pool.ToInternalConfig(config)
	p, err := pool.NewPool(internalConfig, allocator, cleaner)
	require.NoError(t, err)
	return p
}

func runNilReturnTest(t *testing.T, p *pool.Pool[*testObject]) {
	objects := make([]*testObject, 100)
	var nilCount int
	for i := range 100 {
		objects[i] = p.Get()
		if objects[i] == nil {
			nilCount++
		}
	}

	fmt.Println("nilCount", nilCount)
	assert.Equal(t, 80, nilCount)

	// Clean up non-nil objects
	for _, obj := range objects {
		if obj != nil {
			p.Put(obj)
		}
	}
}

func runBlockingTest(t *testing.T, p *pool.Pool[*testObject]) {
	objects := make([]*testObject, 20)
	for i := range 20 {
		objects[i] = p.Get()
		require.NotNil(t, objects[i])
	}

	done := make(chan bool)
	go func() {
		obj := p.Get()
		require.NotNil(t, obj)
		done <- true
	}()

	time.Sleep(100 * time.Millisecond)

	p.Put(objects[0])

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("goroutine did not unblock after returning an object")
	}

	// Clean up remaining objects
	for i := 1; i < len(objects); i++ {
		p.Put(objects[i])
	}
}

func runConcurrentBlockingTest(t *testing.T, p *pool.Pool[*testObject], numGoroutines int) {
	// First fill up the pool to its hard limit
	objects := make([]*testObject, 20)
	for i := range 20 {
		objects[i] = p.Get()
		require.NotNil(t, objects[i])
	}

	// Create a channel to track completion of all goroutines
	done := make(chan bool, numGoroutines)
	start := make(chan struct{}) // Used to synchronize goroutine start

	// Launch multiple goroutines that will try to get objects
	for i := range numGoroutines {
		go func(id int) {
			<-start        // Wait for the start signal
			obj := p.Get() // should remain blocked
			require.NotNil(t, obj)
			done <- true
		}(i)
	}

	// Give some time for goroutines to start and block
	time.Sleep(100 * time.Millisecond)
	close(start) // Signal all goroutines to start trying to get objects
	time.Sleep(100 * time.Millisecond)

	// Start returning objects one by one
	for i := range numGoroutines {
		p.Put(objects[i])
		select {
		case <-done:
			// Successfully unblocked a goroutine
		case <-time.After(1 * time.Second):
			t.Fatalf("goroutine %d did not unblock after returning an object", i)
		}
	}

	// Return remaining objects
	for i := numGoroutines; i < len(objects); i++ {
		p.Put(objects[i])
	}
}

// createComprehensiveTestConfig creates a configuration for comprehensive testing
func createComprehensiveTestConfig(t *testing.T) pool.PoolConfig {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(2).
		SetGrowthPercent(0.5).
		SetFixedGrowthFactor(1.0).
		SetMinShrinkCapacity(1).
		SetShrinkCheckInterval(10 * time.Millisecond).
		SetIdleThreshold(20 * time.Millisecond).
		SetMinIdleBeforeShrink(1).
		SetShrinkCooldown(10 * time.Millisecond).
		SetMinUtilizationBeforeShrink(0.1).
		SetStableUnderutilizationRounds(1).
		SetShrinkPercent(0.5).
		SetMaxConsecutiveShrinks(5).
		SetHardLimit(10).
		SetRingBufferBlocking(true).
		Build()
	require.NoError(t, err)
	return config
}

// testInitialGrowth tests the initial growth behavior of the pool
func testInitialGrowth(t *testing.T, p *pool.Pool[*testObject], numObjects int) []*testObject {
	objects := make([]*testObject, numObjects)
	for i := range numObjects {
		objects[i] = p.Get()
		assert.NotNil(t, objects[i])
		assert.Equal(t, 42, objects[i].value)
	}
	assert.True(t, p.IsGrowth())
	return objects
}

// testShrinkBehavior tests the shrink behavior of the pool
func testShrinkBehavior(t *testing.T, p *pool.Pool[*testObject], objects []*testObject) {
	for _, obj := range objects {
		p.Put(obj)
	}

	time.Sleep(100 * time.Millisecond)
	assert.True(t, p.IsShrunk())
}

// testHardLimitBlocking tests the hard limit and blocking behavior of the pool
func testHardLimitBlocking(t *testing.T, p *pool.Pool[*testObject], numObjects int) []*testObject {
	// First get all objects up to hard limit
	objects := make([]*testObject, numObjects)
	for i := range numObjects {
		objects[i] = p.Get()
		assert.NotNil(t, objects[i])
	}

	// Try to get one more object - should block
	done := make(chan bool)
	go func() {
		obj := p.Get()
		assert.NotNil(t, obj)
		done <- true
	}()

	// Wait a bit to ensure the goroutine is blocked
	time.Sleep(100 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("Get should have blocked (100ms)")
	default:
		// Expected - goroutine is blocked
	}

	// Release one object to unblock the waiting goroutine
	p.Put(objects[0])
	select {
	case <-done:
		// Success - goroutine was unblocked
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Get should have unblocked (100ms)")
	}

	return objects
}

func cleanupPoolObjects(p *pool.Pool[*testObject], objects []*testObject) {
	for _, obj := range objects {
		if obj != nil {
			p.Put(obj)
		}
	}
}

func testInvalidConfig(t *testing.T, name string, configFunc func() (pool.PoolConfig, error)) {
	t.Run(name, func(t *testing.T) {
		config, err := configFunc()
		assert.Error(t, err)
		assert.Nil(t, config)
	})
}

func testValidConfig(t *testing.T, name string, configFunc func() (pool.PoolConfig, error)) {
	t.Run(name, func(t *testing.T) {
		config, err := configFunc()
		assert.NoError(t, err)
		assert.NotNil(t, config)
	})
}
