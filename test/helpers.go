package test

import (
	"fmt"
	"memctx/contexts"
	"memctx/pool"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
func createCustomConfig(t *testing.T) pool.PoolConfig {
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
		Build()
	require.NoError(t, err)

	return config
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
func createTestPool(t *testing.T, config pool.PoolConfig) *pool.Pool[*TestObject] {
	allocator := func() *TestObject {
		return &TestObject{Value: 42}
	}

	cleaner := func(obj *TestObject) {
		obj.Value = 0
	}

	internalConfig := pool.ToInternalConfig(config)
	p, err := pool.NewPool(internalConfig, allocator, cleaner, reflect.TypeOf(&TestObject{}))
	require.NoError(t, err)
	return p
}

func runNilReturnTest(t *testing.T, p *pool.Pool[*TestObject]) {
	objects := make([]*TestObject, 100)
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

func runBlockingTest(t *testing.T, p *pool.Pool[*TestObject]) {
	objects := make([]*TestObject, 20)
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

func runConcurrentBlockingTest(t *testing.T, p *pool.Pool[*TestObject], numGoroutines int) {
	// First fill up the pool to its hard limit
	objects := make([]*TestObject, 20)
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
func testInitialGrowth(t *testing.T, p *pool.Pool[*TestObject], numObjects int) []*TestObject {
	objects := make([]*TestObject, numObjects)
	for i := range numObjects {
		objects[i] = p.Get()
		assert.NotNil(t, objects[i])
		assert.Equal(t, 42, objects[i].Value)
	}
	assert.True(t, p.IsGrowth())
	return objects
}

// testShrinkBehavior tests the shrink behavior of the pool
func testShrinkBehavior(t *testing.T, p *pool.Pool[*TestObject], objects []*TestObject) {
	for _, obj := range objects {
		p.Put(obj)
	}

	time.Sleep(100 * time.Millisecond)
	assert.True(t, p.IsShrunk())
}

// testHardLimitBlocking tests the hard limit and blocking behavior of the pool
func testHardLimitBlocking(t *testing.T, p *pool.Pool[*TestObject], numObjects int) []*TestObject {
	// First get all objects up to hard limit
	objects := make([]*TestObject, numObjects)
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

func cleanupPoolObjects(p *pool.Pool[*TestObject], objects []*TestObject) {
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

type testSetup struct {
	cm      *contexts.ContextManager
	ctx     *contexts.MemoryContext
	ctxType contexts.MemoryContextType
}

func setupTest(t *testing.T) *testSetup {
	cm := contexts.NewContextManager(nil)
	t.Cleanup(func() { cm.Close() })

	ctxType := contexts.MemoryContextType("test")
	config := contexts.MemoryContextConfig{
		ContextType: ctxType,
	}

	ctx, exists, err := cm.GetOrCreateContext(ctxType, config)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	if exists {
		t.Error("Expected context to be created")
	}

	return &testSetup{
		cm:      cm,
		ctx:     ctx,
		ctxType: ctxType,
	}
}

func createTestPoolConfig(t *testing.T, initialCapacity, hardLimit int, blocking bool) pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(initialCapacity).
		SetHardLimit(hardLimit).
		SetMinShrinkCapacity(initialCapacity).
		SetRingBufferBlocking(blocking).
		Build()
	require.NoError(t, err)

	return poolConfig
}

func TestMemoryContextCreation(t *testing.T) {
	setup := setupTest(t)

	// Verify context exists
	if setup.ctx == nil {
		t.Error("Expected context to be created")
	}
}

func TestInvalidPoolCreation(t *testing.T) {
	setup := setupTest(t)

	// Test with nil allocator
	err := setup.ctx.CreatePool(reflect.TypeOf(&TestObject{}), nil, nil, nil)
	if err == nil {
		t.Error("Expected error when creating pool with nil allocator")
	}

	// Test with non-pointer type
	poolConfig, err := pool.NewPoolConfigBuilder().Build()
	if err != nil {
		t.Fatalf("Failed to create pool config: %v", err)
	}
	err = setup.ctx.CreatePool(reflect.TypeOf(TestObject{}), poolConfig, func() any { return TestObject{} }, nil)
	if err == nil {
		t.Error("Expected error when creating pool with non-pointer type")
	}
}

func TestValidPoolCreation(t *testing.T) {
	setup := setupTest(t)
	poolConfig := createTestPoolConfig(t, 10, 100, false)

	allocator := func() any {
		return &TestObject{Value: 42}
	}

	cleaner := func(obj any) {
		if to, ok := obj.(*TestObject); ok {
			to.Value = 0
		}
	}

	err := setup.ctx.CreatePool(reflect.TypeOf(&TestObject{}), poolConfig, allocator, cleaner)
	if err != nil {
		t.Fatalf("Failed to create valid pool: %v", err)
	}

	// Verify pool exists
	poolObj := setup.ctx.GetPool(reflect.TypeOf(&TestObject{}))
	if poolObj == nil {
		t.Error("Expected pool to exist after creation")
	}
}

// createTestPools creates and configures all the test pools in the given context
func createTestPools(t *testing.T, ctx *contexts.MemoryContext) {
	// Create processor pool
	processorPoolConfig := createTestPoolConfig(t, 5, 20, false)
	processorAllocator := func() any {
		return &TestObject{Value: 42}
	}
	processorCleaner := func(obj any) {
		if to, ok := obj.(*TestObject); ok {
			to.Value = 0
		}
	}

	// Create buffer pool
	bufferPoolConfig := createTestPoolConfig(t, 10, 30, false)
	bufferAllocator := func() any {
		return &TestBuffer{
			Data: make([]byte, 1024),
			Size: 1024,
		}
	}
	bufferCleaner := func(obj any) {
		if tb, ok := obj.(*TestBuffer); ok {
			tb.Data = nil
			tb.Size = 0
		}
	}

	// Create metadata pool
	metadataPoolConfig := createTestPoolConfig(t, 8, 25, false)
	metadataAllocator := func() any {
		return &TestMetadata{
			Timestamp: time.Now(),
			Priority:  0,
			Tags:      make([]string, 0),
		}
	}
	metadataCleaner := func(obj any) {
		if tm, ok := obj.(*TestMetadata); ok {
			tm.Timestamp = time.Time{}
			tm.Priority = 0
			tm.Tags = nil
		}
	}

	// Create all pools
	err := ctx.CreatePool(reflect.TypeOf(&TestObject{}), processorPoolConfig, processorAllocator, processorCleaner)
	require.NoError(t, err)

	err = ctx.CreatePool(reflect.TypeOf(&TestBuffer{}), bufferPoolConfig, bufferAllocator, bufferCleaner)
	require.NoError(t, err)

	err = ctx.CreatePool(reflect.TypeOf(&TestMetadata{}), metadataPoolConfig, metadataAllocator, metadataCleaner)
	require.NoError(t, err)
}

// processJob handles a single job using resources from the pools
func processJob(t *testing.T, ctx *contexts.MemoryContext, jobID int) *Job {
	// Acquire all resources needed for a job
	processor := ctx.Acquire(reflect.TypeOf(&TestObject{}))
	buffer := ctx.Acquire(reflect.TypeOf(&TestBuffer{}))
	metadata := ctx.Acquire(reflect.TypeOf(&TestMetadata{}))

	// Verify resources were acquired
	assert.NotNil(t, processor, "Expected to acquire processor")
	assert.NotNil(t, buffer, "Expected to acquire buffer")
	assert.NotNil(t, metadata, "Expected to acquire metadata")

	// Create a complete job with all resources
	job := &Job{
		ID:        jobID,
		Processor: processor.(*TestObject),
		Buffer:    buffer.(*TestBuffer),
		Metadata:  metadata.(*TestMetadata),
	}

	// Simulate work using all resources
	job.Metadata.Timestamp = time.Now()
	job.Metadata.Priority = jobID % 3
	job.Metadata.Tags = append(job.Metadata.Tags, "test")
	job.Processor.Value = jobID
	copy(job.Buffer.Data, []byte("test data"))

	// Release all resources back to their pools
	processorSuccess := ctx.Release(reflect.TypeOf(&TestObject{}), processor)
	bufferSuccess := ctx.Release(reflect.TypeOf(&TestBuffer{}), buffer)
	metadataSuccess := ctx.Release(reflect.TypeOf(&TestMetadata{}), metadata)

	// Verify successful release
	assert.True(t, processorSuccess, "Expected successful processor release")
	assert.True(t, bufferSuccess, "Expected successful buffer release")
	assert.True(t, metadataSuccess, "Expected successful metadata release")

	return job
}

// worker processes jobs from the jobs channel and sends results to the results channel
func worker(t *testing.T, ctx *contexts.MemoryContext, jobs <-chan int, results chan<- *Job, wg *sync.WaitGroup) {
	defer wg.Done()
	for jobID := range jobs {
		job := processJob(t, ctx, jobID)
		results <- job
	}
}

func testConcurrentAcquireRelease(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "test_context",
	})

	// blocking mode otherwise it will return nil
	poolConfig := createTestPoolConfig(t, 8, 8, true)
	allocator := func() any {
		return &TestObject{Value: 42}
	}
	cleaner := func(obj any) {
		if to, ok := obj.(*TestObject); ok {
			to.Value = 0
		}
	}

	err := ctx.CreatePool(reflect.TypeOf(&TestObject{}), poolConfig, allocator, cleaner)
	require.NoError(t, err)

	const numGoroutines = 50
	const operationsPerGoroutine = 1000
	var wg sync.WaitGroup
	var successCount int32

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range operationsPerGoroutine {
				obj := ctx.Acquire(reflect.TypeOf(&TestObject{}))
				if obj != nil {
					if success := ctx.Release(reflect.TypeOf(&TestObject{}), obj); success {
						atomic.AddInt32(&successCount, 1)
					}
				}
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int32(numGoroutines*operationsPerGoroutine), successCount, "Expected all operations to succeed")
}

func testConcurrentChildCreation(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "test_context",
	})

	const numGoroutines = 20
	var wg sync.WaitGroup
	var successCount int32

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ctx.CreateChild()
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int32(numGoroutines), successCount, "Expected all child creations to succeed")
}

func testConcurrentPoolOperationsWithChildren(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "test_context",
	})

	poolConfig := createTestPoolConfig(t, 8, 8, true)
	allocator := func() any {
		return &TestObject{Value: 42}
	}

	cleaner := func(obj any) {
		if to, ok := obj.(*TestObject); ok {
			to.Value = 0
		}
	}

	err := ctx.CreatePool(reflect.TypeOf(&TestObject{}), poolConfig, allocator, cleaner)
	require.NoError(t, err)

	const numChildren = 5
	const numGoroutines = 10
	const operationsPerGoroutine = 50
	var wg sync.WaitGroup

	for range numChildren {
		_, err := ctx.CreateChild()
		require.NoError(t, err)
	}

	for _, child := range ctx.GetChildren() {
		for range numGoroutines {
			wg.Add(1)
			go func(c *contexts.MemoryContext) {
				defer wg.Done()
				for range operationsPerGoroutine {
					obj := c.Acquire(reflect.TypeOf(&TestObject{}))
					require.NotNil(t, obj, "Failed to acquire object from pool")
					success := c.Release(reflect.TypeOf(&TestObject{}), obj)
					require.True(t, success, "Failed to release object back to pool")
				}
			}(child)
		}
	}

	wg.Wait()
}

func testConcurrentClose(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "test_context",
	})

	poolConfig := createTestPoolConfig(t, 2, 10, false)
	allocator := func() any {
		return &TestObject{Value: 42}
	}
	cleaner := func(obj any) {
		if to, ok := obj.(*TestObject); ok {
			to.Value = 0
		}
	}

	err := ctx.CreatePool(reflect.TypeOf(&TestObject{}), poolConfig, allocator, cleaner)
	require.NoError(t, err)

	const numGoroutines = 10
	var wg sync.WaitGroup
	var closeCount int32

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ctx.Close(); err == nil {
				atomic.AddInt32(&closeCount, 1)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int32(1), closeCount, "Expected only one successful close")
}

func testOperationsAfterClose(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "test_context_after_close",
	})

	// Create a test pool
	poolConfig := createTestPoolConfig(t, 2, 10, false)
	allocator := func() any {
		return &TestObject{Value: 42}
	}
	cleaner := func(obj any) {
		if to, ok := obj.(*TestObject); ok {
			to.Value = 0
		}
	}

	err := ctx.CreatePool(reflect.TypeOf(&TestObject{}), poolConfig, allocator, cleaner)
	require.NoError(t, err)

	// Close the context
	require.NoError(t, ctx.Close())

	// Test operations after close
	assert.Nil(t, ctx.Acquire(reflect.TypeOf(&TestObject{})), "Expected nil when acquiring after close")
	assert.False(t, ctx.Release(reflect.TypeOf(&TestObject{}), &TestObject{}), "Expected false when releasing after close")
	_, err = ctx.CreateChild()
	assert.Error(t, err, "Expected error when creating child after close")
}

func testStressTest(t *testing.T) {
	ctx := contexts.NewMemoryContext(contexts.MemoryContextConfig{
		ContextType: "stress_test",
	})

	poolConfig := createTestPoolConfig(t, 2, 10, false)

	for i := range 5 {
		objType := reflect.TypeOf(&TestObject{})
		allocator := func() any {
			return &TestObject{Value: i}
		}
		cleaner := func(obj any) {
			if to, ok := obj.(*TestObject); ok {
				to.Value = 0
			}
		}
		require.NoError(t, ctx.CreatePool(objType, poolConfig, allocator, cleaner))
	}

	const numGoroutines = 100
	const duration = 2 * time.Second
	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					// Randomly choose an operation
					switch id % 4 {
					case 0:
						obj := ctx.Acquire(reflect.TypeOf(&TestObject{}))
						if obj != nil {
							ctx.Release(reflect.TypeOf(&TestObject{}), obj)
						}
					case 1:
						ctx.CreateChild()
					case 2:
						ctx.GetPool(reflect.TypeOf(&TestObject{}))
					}
				}
			}
		}(i)
	}

	time.Sleep(duration)
	close(stop)
	wg.Wait()

	assert.NoError(t, ctx.Close())
}
