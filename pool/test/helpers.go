package test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/PoolX/pool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Job struct {
	ID        int
	Processor *TestObject
	Buffer    *TestBuffer
	Metadata  *TestMetadata
}

type TestObject struct {
	Value int
}

type TestBuffer struct {
	Data []byte
	Size int
}

type TestMetadata struct {
	Priority  int
	Tags      []string
	Timestamp time.Time
}

type TestJob struct {
	ID        int
	Buffer    TestBuffer
	Metadata  TestMetadata
	CreatedAt time.Time
}

// DefaultConfigValues stores all default configuration values
type DefaultConfigValues struct {
	InitialCapacity                    int
	HardLimit                          int
	GrowthFactor                       int
	FixedGrowthFactor                  int
	ExponentialThresholdFactor         int
	ShrinkAggressiveness               pool.AggressivenessLevel
	ShrinkCheckInterval                time.Duration
	IdleThreshold                      time.Duration
	MinIdleBeforeShrink                int
	ShrinkCooldown                     time.Duration
	MinUtilizationBeforeShrink         int
	StableUnderutilizationRounds       int
	ShrinkPercent                      int
	MinShrinkCapacity                  int
	MaxConsecutiveShrinks              int
	InitialSize                        int
	FillAggressiveness                 int
	RefillPercent                      int
	EnableChannelGrowth                bool
	GrowthEventsTrigger                int
	FastPathGrowthFactor               int
	FastPathExponentialThresholdFactor int
	FastPathFixedGrowthFactor          int
	ShrinkEventsTrigger                int
	FastPathShrinkAggressiveness       pool.AggressivenessLevel
	FastPathShrinkPercent              int
	FastPathShrinkMinCapacity          int
	Verbose                            bool
	RingBufferBlocking                 bool
	ReadTimeout                        time.Duration
	WriteTimeout                       time.Duration
}

// storeDefaultConfigValues stores all default configuration values
func storeDefaultConfigValues(config *pool.PoolConfig) DefaultConfigValues {
	return DefaultConfigValues{
		InitialCapacity:                    config.GetInitialCapacity(),
		HardLimit:                          config.GetHardLimit(),
		GrowthFactor:                       config.GetGrowth().GetGrowthFactor(),
		FixedGrowthFactor:                  config.GetGrowth().GetFixedGrowthFactor(),
		ExponentialThresholdFactor:         config.GetGrowth().GetExponentialThresholdFactor(),
		ShrinkAggressiveness:               config.GetShrink().GetAggressivenessLevel(),
		ShrinkCheckInterval:                config.GetShrink().GetCheckInterval(),
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
		FastPathGrowthFactor:               config.GetFastPath().GetGrowth().GetGrowthFactor(),
		FastPathExponentialThresholdFactor: config.GetFastPath().GetGrowth().GetExponentialThresholdFactor(),
		FastPathFixedGrowthFactor:          config.GetFastPath().GetGrowth().GetFixedGrowthFactor(),
		ShrinkEventsTrigger:                config.GetFastPath().GetShrinkEventsTrigger(),
		FastPathShrinkAggressiveness:       config.GetFastPath().GetShrink().GetAggressivenessLevel(),
		FastPathShrinkPercent:              config.GetFastPath().GetShrink().GetShrinkPercent(),
		FastPathShrinkMinCapacity:          config.GetFastPath().GetShrink().GetMinCapacity(),
		RingBufferBlocking:                 config.GetRingBufferConfig().IsBlocking(),
		ReadTimeout:                        config.GetRingBufferConfig().GetReadTimeout(),
		WriteTimeout:                       config.GetRingBufferConfig().GetWriteTimeout(),
	}
}

// createCustomConfig creates a custom configuration with different values
func createCustomConfig(t *testing.T) *pool.PoolConfig {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(101210).
		SetHardLimit(10000000028182820).
		SetGrowthFactor(52).
		SetFixedGrowthFactor(100).
		SetGrowthExponentialThresholdFactor(400).
		SetShrinkCheckInterval(2 * time.Second).
		SetShrinkCooldown(10 * time.Second).
		SetMinUtilizationBeforeShrink(32).
		SetStableUnderutilizationRounds(3121).
		SetShrinkPercent(25).
		SetMinShrinkCapacity(10121).
		SetMaxConsecutiveShrinks(3121).
		SetFastPathInitialSize(50121).
		SetFastPathFillAggressiveness(800).
		SetFastPathRefillPercent(2121).
		SetFastPathEnableChannelGrowth(false).
		SetFastPathGrowthEventsTrigger(5121).
		SetFastPathGrowthFactor(6121).
		SetFastPathExponentialThresholdFactor(300).
		SetFastPathFixedGrowthFactor(800).
		SetFastPathShrinkEventsTrigger(4121).
		SetFastPathShrinkAggressiveness(pool.AggressivenessLevel(1)).
		SetFastPathShrinkPercent(31).
		SetFastPathShrinkMinCapacity(20121).
		SetRingBufferBlocking(true).
		SetRingBufferTimeout(5 * time.Second).
		Build()
	require.NoError(t, err)

	return config
}

// verifyCustomValuesDifferent verifies that custom values are different from default values
func verifyCustomValuesDifferent(t *testing.T, original DefaultConfigValues, custom *pool.PoolConfig) {
	assert.NotEqual(t, original.InitialCapacity, custom.GetInitialCapacity())
	assert.NotEqual(t, original.HardLimit, custom.GetHardLimit())
	assert.NotEqual(t, original.GrowthFactor, custom.GetGrowth().GetGrowthFactor())
	assert.NotEqual(t, original.FixedGrowthFactor, custom.GetGrowth().GetFixedGrowthFactor())
	assert.NotEqual(t, original.ExponentialThresholdFactor, custom.GetGrowth().GetExponentialThresholdFactor())
	assert.NotEqual(t, original.ShrinkCheckInterval, custom.GetShrink().GetCheckInterval())
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
	assert.NotEqual(t, original.FastPathGrowthFactor, custom.GetFastPath().GetGrowth().GetGrowthFactor())
	assert.NotEqual(t, original.FastPathExponentialThresholdFactor, custom.GetFastPath().GetGrowth().GetExponentialThresholdFactor())
	assert.NotEqual(t, original.FastPathFixedGrowthFactor, custom.GetFastPath().GetGrowth().GetFixedGrowthFactor())
	assert.NotEqual(t, original.ShrinkEventsTrigger, custom.GetFastPath().GetShrinkEventsTrigger())
	assert.NotEqual(t, original.FastPathShrinkAggressiveness, custom.GetFastPath().GetShrink().GetAggressivenessLevel())
	assert.NotEqual(t, original.FastPathShrinkPercent, custom.GetFastPath().GetShrink().GetShrinkPercent())
	assert.NotEqual(t, original.FastPathShrinkMinCapacity, custom.GetFastPath().GetShrink().GetMinCapacity())
	assert.NotEqual(t, original.RingBufferBlocking, custom.GetRingBufferConfig().IsBlocking())
	assert.NotEqual(t, original.ReadTimeout, custom.GetRingBufferConfig().GetReadTimeout())
	assert.NotEqual(t, original.WriteTimeout, custom.GetRingBufferConfig().GetWriteTimeout())
}

// createHardLimitTestConfig creates a configuration for hard limit testing
func createHardLimitTestConfig(t *testing.T, blocking bool) *pool.PoolConfig {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(10).
		SetHardLimit(20).
		SetMinShrinkCapacity(10).
		SetRingBufferBlocking(blocking).
		Build()
	require.NoError(t, err)
	return config
}

// createTestPool creates a pool with the given configuration
func createTestPool(t *testing.T, config *pool.PoolConfig) *pool.Pool[*TestObject] {
	allocator := func() *TestObject {
		return &TestObject{Value: 42}
	}

	cleaner := func(obj *TestObject) {
		obj.Value = 0
	}

	p, err := pool.NewPool(config, allocator, cleaner)
	require.NoError(t, err)
	return p.(*pool.Pool[*TestObject])
}

func runNilReturnTest(t *testing.T, p *pool.Pool[*TestObject]) {
	objects := make([]*TestObject, 100)

	var nilCount int
	for i := range 100 {
		objects[i], _ = p.Get()
		if objects[i] == nil {
			nilCount++
		}
	}

	assert.Equal(t, 80, nilCount)

	for _, obj := range objects {
		if obj != nil {
			p.Put(obj)
		}
	}
}

func runBlockingTest(t *testing.T, p *pool.Pool[*TestObject]) {
	objects := make([]*TestObject, 20)
	var err error

	for i := range 20 {
		objects[i], err = p.Get()
		require.NoError(t, err)
		require.NotNil(t, objects[i])
	}

	done := make(chan bool)
	go func() {
		obj, err := p.Get()
		require.NoError(t, err)
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

}

func runConcurrentBlockingTest(t *testing.T, p *pool.Pool[*TestObject], numGoroutines int) {
	// First fill up the pool to its hard limit
	objects := make([]*TestObject, 20)
	var err error
	for i := range 20 {
		objects[i], err = p.Get()
		require.NoError(t, err)
		require.NotNil(t, objects[i])
	}

	// Create a channel to track completion of all goroutines
	done := make(chan bool, numGoroutines)
	start := make(chan struct{}) // Used to synchronize goroutine start

	// Launch multiple goroutines that will try to get objects
	for i := range numGoroutines {
		go func(id int) {
			<-start             // Wait for the start signal
			obj, err := p.Get() // should remain blocked
			require.NoError(t, err)
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

func testInvalidConfig(t *testing.T, name string, configFunc func() (*pool.PoolConfig, error)) {
	t.Run(name, func(t *testing.T) {
		config, err := configFunc()
		assert.Error(t, err)
		assert.Nil(t, config)
	})
}

func testValidConfig(t *testing.T, name string, configFunc func() (*pool.PoolConfig, error)) {
	t.Run(name, func(t *testing.T) {
		config, err := configFunc()
		assert.NoError(t, err)
		assert.NotNil(t, config)
	})
}

func ValidateCapacity(p *pool.Pool[*TestObject], stats *pool.PoolStatsSnapshot) error {
	ringCap := p.RingBufferCapacity()
	if ringCap != stats.CurrentCapacity {
		return fmt.Errorf("current capacity (%d) does not match ring buffer capacity (%d)", stats.CurrentCapacity, ringCap)
	}

	return nil
}

func readBlockersTest(t *testing.T, config *pool.PoolConfig, numGoroutines, availableItems int, async bool) {
	p := createTestPool(t, config)

	objects := make(chan *TestObject, availableItems)
	completed := make([]bool, numGoroutines)

	var wg sync.WaitGroup

	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(idx int) {
			defer wg.Done()
			obj, err := p.Get()
			assert.NoError(t, err)
			assert.NotNil(t, obj)

			objects <- obj
			completed[idx] = true
		}(i)
	}

	time.Sleep(2 * time.Second)
	expectedBlocked := numGoroutines - availableItems
	assert.Equal(t, expectedBlocked, p.GetBlockedReaders())

	go func() {
		wg.Wait()
		close(objects)
	}()

	for obj := range objects {
		if async {
			go func() {
				time.Sleep(time.Duration(rand.Intn(90)+10) * time.Millisecond)
				err := p.Put(obj)
				assert.NoError(t, err)
			}()
		} else {
			require.NotNil(t, obj, "Object is nil")
			err := p.Put(obj)
			assert.NoError(t, err)
		}
	}

	for i := range completed {
		assert.True(t, completed[i], "Goroutine %d did not complete", i)
	}

	assert.Equal(t, 0, p.GetBlockedReaders())
}

func createConfig(t *testing.T, hardLimit, initial, attempts int, verbose bool) *pool.PoolConfig {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(initial).
		SetMinShrinkCapacity(initial). // prevent shrinking
		SetRingBufferBlocking(true).   // prevent nil returns
		SetHardLimit(hardLimit).
		SetPreReadBlockHookAttempts(attempts).
		Build()
	require.NoError(t, err)
	return config
}

func hardLimitTest(t *testing.T, config *pool.PoolConfig, numGoroutines, hardLimit int, async bool) {
	p := createTestPool(t, config)
	objects := make(chan *TestObject, hardLimit)

	var readers sync.WaitGroup
	readers.Add(numGoroutines)
	for i := range numGoroutines {
		go func(idx int) {
			defer readers.Done()
			obj, err := p.Get()
			assert.NoError(t, err)
			assert.NotNil(t, obj)

			objects <- obj
		}(i)
	}

	go func() {
		readers.Wait()
		close(objects)
	}()

	var writers sync.WaitGroup
	for obj := range objects {

		if async {
			writers.Add(1)
			go func() {
				defer writers.Done()
				time.Sleep(time.Duration(rand.Intn(5)+1) * time.Millisecond)
				err := p.Put(obj)
				assert.NoError(t, err)
			}()
		} else {
			err := p.Put(obj)
			assert.NoError(t, err)
		}
	}

	writers.Wait()
	assert.Equal(t, 0, p.GetBlockedReaders())
	poolSnapshot := p.GetPoolStatsSnapshot()
	assert.NoError(t, poolSnapshot.Validate(numGoroutines))
}

func testGrowth(t *testing.T, runs int, hardLimit, initial, attempts, numGoroutines int) {
	for i := range runs {
		t.Run(fmt.Sprintf("run-%d", i+1), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			done := make(chan struct{})
			go func() {
				defer close(done)

				config := createConfig(t, hardLimit, initial, attempts, false)
				hardLimitTest(t, config, numGoroutines, hardLimit, true)
			}()

			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("subtest timed out, possible deadlock")
			}
		})
	}
}
