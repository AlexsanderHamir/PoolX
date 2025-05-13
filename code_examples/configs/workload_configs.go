package configs

import (
	"time"

	"github.com/AlexsanderHamir/PoolX/v2/pool"
)

type Example struct {
	ID   int
	Name string
	Data []byte
}

func CreateHighThroughputConfig() *pool.PoolConfig[*Example] {
	poolConfig, err := pool.NewPoolConfigBuilder[*Example]().
		// Basic pool settings:
		// - Initial capacity: 1000 objects
		// - Max capacity: 1000 objects
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(1000, 1000, true).
		// Ring buffer settings:
		// - Blocking mode enabled for better throughput
		// - No read/write specific timeouts (0)
		// - 3 second general timeout for both operations
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*3).
		// Ring buffer growth strategy:
		// - Big growth threshold: 1 (100% of current capacity)
		// - Big growth rate: 1 (100% growth)
		// - Controlled growth rate: 0.01 (1% of current capacity)
		SetRingBufferGrowthConfigs(1, 1, 0.01).
		// Ring buffer shrink strategy:
		// - Check every 5 seconds
		// - Consider idle after 10 seconds from last get call
		// - Minimum 3 idle checks before shrinking
		// - Minimum allowed to shrink to: 50 objects
		// - Maximum allowed consecutive shrinks: 5
		// - Shrink when utilization below 40%
		// - Shrink by 30% when triggered
		SetRingBufferShrinkConfigs(time.Second*5, time.Second*10, 3, 50, 5, 40, 30).
		// Fast path (L1 cache) settings:
		// - Initial size: 200 objects
		// - Grow after 1 growth event
		// - Shrink after 1 shrink event
		// - Fill aggressiveness: 100%
		// - Refill when 3% empty
		SetFastPathBasicConfigs(200, 1, 1, 100, 3).
		// Fast path growth strategy:
		// - Exponential growth threshold: 100 (100x initial capacity)
		// - Controlled growth rate: 3 (300% of current capacity)
		// - Big growth rate: 0.5 (50% of current capacity)
		SetFastPathGrowthConfigs(100, 3, 0.5).
		// Fast path shrink strategy:
		// - Shrink by 40% when triggered
		// - Minimum 20 objects
		SetFastPathShrinkConfigs(40, 20).
		// Allocation strategy:
		// - 100% allocation percent
		// - 100 objects per allocation
		SetAllocationStrategy(100, 100).
		Build()
	if err != nil {
		panic(err)
	}

	return poolConfig
}

func CreateMemoryConstrainedConfig() *pool.PoolConfig[*Example] {
	poolConfig, err := pool.NewPoolConfigBuilder[*Example]().
		// Basic pool settings:
		// - Initial capacity: 16 objects (minimal for memory-constrained systems)
		// - Max capacity: 1000 objects (strict limit for memory-constrained systems)
		// - Verbose logging disabled to save memory
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(16, 1000, true).
		// Ring buffer settings:
		// - Blocking mode enabled for better memory management
		// - No read/write specific timeouts (0)
		// - 30 second general timeout for both operations
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*30).
		// Ring buffer growth strategy:
		// - Exponential growth until 100% of current capacity
		// - 30% growth rate in exponential mode
		// - 20% fixed growth after exponential phase
		SetRingBufferGrowthConfigs(100, 30, 20).
		// Ring buffer shrink strategy:
		// - Check every 15 seconds
		// - Consider idle after 60 seconds from last get call
		// - 10 second cooldown between shrinks
		// - Minimum 5 idle checks before shrinking
		// - 8 stable underutilization rounds
		// - Minimum allowed to shrink to: 8 objects
		// - Maximum allowed consecutive shrinks: 3
		// - Shrink when utilization below 20%
		// - Shrink by 40% when triggered
		SetRingBufferShrinkConfigs(time.Second*15, time.Second*60, 5, 8, 8, 20, 40).
		// Fast path (L1 cache) settings:
		// - Initial size: 16 objects
		// - Grow after 2 growth events
		// - Shrink after 2 shrink events
		// - Moderate fill aggressiveness (0.7) - 70%
		// - Refill when 50% empty
		SetFastPathBasicConfigs(16, 2, 2, 70, 50).
		// Fast path growth strategy:
		// - Exponential growth until 100% of current capacity
		// - 30% fixed growth after exponential phase
		// - 40% growth rate in exponential mode
		SetFastPathGrowthConfigs(100, 130, 40).
		// Fast path shrink strategy:
		// - Shrink by 50% when triggered
		// - Minimum 4 objects
		SetFastPathShrinkConfigs(50, 4).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

func CreateLowLatencyConfig() *pool.PoolConfig[*Example] {
	poolConfig, err := pool.NewPoolConfigBuilder[*Example]().
		// Basic pool settings:
		// - Initial capacity: 512 objects (larger for low latency)
		// - Max capacity: 20000 objects (higher limit for burst handling)
		// - Verbose logging disabled to minimize overhead
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(512, 20000, true).
		// Ring buffer settings:
		// - Blocking mode enabled for better latency
		// - No read/write specific timeouts (0)
		// - 1 second general timeout for both operations
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*1).
		// Ring buffer growth strategy:
		// - Exponential growth until 300% of current capacity
		// - 100% growth rate in exponential mode
		// - 100% fixed growth after exponential phase
		SetRingBufferGrowthConfigs(300.0, 1.0, 2.0).
		// Ring buffer shrink strategy:
		// - Check every 3 seconds
		// - Consider idle after 5 seconds from last get call
		// - 1 second cooldown between shrinks
		// - Minimum 2 idle checks before shrinking
		// - 3 stable underutilization rounds
		// - Minimum allowed to shrink to: 32 objects
		// - Maximum allowed consecutive shrinks: 3
		// - Shrink when utilization below 30%
		// - Shrink by 20% when triggered
		SetRingBufferShrinkConfigs(time.Second*3, time.Second*5, 2, 32, 3, 30, 20).
		// Fast path (L1 cache) settings:
		// - Initial size: 512 objects
		// - Grow after 1 growth event
		// - Shrink after 1 shrink event
		// - Maximum fill aggressiveness (1.0) - 100%
		// - Refill when 20% empty
		SetFastPathBasicConfigs(512, 1, 1, 100, 20).
		// Fast path growth strategy:
		// - Exponential growth until 200% of current capacity
		// - 100% fixed growth after exponential phase
		// - 100% growth rate in exponential mode
		SetFastPathGrowthConfigs(200, 200, 100).
		// Fast path shrink strategy:
		// - Shrink by 30% when triggered
		// - Minimum 16 objects
		SetFastPathShrinkConfigs(30, 16).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

func CreateBatchProcessingConfig() *pool.PoolConfig[*Example] {
	poolConfig, err := pool.NewPoolConfigBuilder[*Example]().
		// Basic pool settings:
		// - Initial capacity: 128 objects (balanced for batch processing)
		// - Max capacity: 5000 objects (reasonable for batch workloads)
		// - Verbose logging disabled to save resources
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(128, 5000, true).
		// Ring buffer settings:
		// - Blocking mode enabled for better throughput
		// - No read/write specific timeouts (0)
		// - 5 second general timeout for both operations
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*5).
		// Ring buffer growth strategy:
		// - Exponential growth until 150% of current capacity
		// - 50% growth rate in exponential mode
		// - 30% fixed growth after exponential phase
		SetRingBufferGrowthConfigs(150, 50, 30).
		// Ring buffer shrink strategy:
		// - Check every 10 seconds
		// - Consider idle after 30 seconds from last get call
		// - 5 second cooldown between shrinks
		// - Minimum 4 idle checks before shrinking
		// - 6 stable underutilization rounds
		// - Minimum allowed to shrink to: 32 objects
		// - Maximum allowed consecutive shrinks: 4
		// - Shrink when utilization below 40%
		// - Shrink by 30% when triggered
		SetRingBufferShrinkConfigs(time.Second*10, time.Second*30, 5, 32, 4, 40, 30).
		// Fast path (L1 cache) settings:
		// - Initial size: 128 objects
		// - Grow after 2 growth events
		// - Shrink after 2 shrink events
		// - High fill aggressiveness (0.9) - 90%
		// - Refill when 30% empty
		SetFastPathBasicConfigs(128, 2, 2, 90, 30).
		// Fast path growth strategy:
		// - Exponential growth until 150% of current capacity
		// - 50% fixed growth after exponential phase
		// - 60% growth rate in exponential mode
		SetFastPathGrowthConfigs(150, 150, 60).
		// Fast path shrink strategy:
		// - Shrink by 40% when triggered
		// - Minimum 16 objects
		SetFastPathShrinkConfigs(40, 16).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

func CreateRealTimeConfig() *pool.PoolConfig[*Example] {
	poolConfig, err := pool.NewPoolConfigBuilder[*Example]().
		// Basic pool settings:
		// - Initial capacity: 1024 objects (large for real-time systems)
		// - Max capacity: 50000 objects (very high for burst handling)
		// - Verbose logging disabled to minimize overhead
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(1024, 50000, true).
		// Ring buffer settings:
		// - Blocking mode enabled for better latency
		// - No read/write specific timeouts (0)
		// - 500ms general timeout for both operations
		SetRingBufferBasicConfigs(true, 0, 0, time.Millisecond*500).
		// Ring buffer growth strategy:
		// - Exponential growth until 400% of current capacity
		// - 150% growth rate in exponential mode
		// - 100% fixed growth after exponential phase
		SetRingBufferGrowthConfigs(400, 150, 100).
		// Ring buffer shrink strategy:
		// - Check every 1 second
		// - Consider idle after 3 seconds from last get call
		// - 500ms cooldown between shrinks
		// - Minimum 2 idle checks before shrinking
		// - 2 stable underutilization rounds
		// - Minimum allowed to shrink to: 64 objects
		// - Maximum allowed consecutive shrinks: 2
		// - Shrink when utilization below 20%
		// - Shrink by 10% when triggered
		SetRingBufferShrinkConfigs(time.Second*1, time.Second*3, 2, 64, 2, 20, 10).
		// Fast path (L1 cache) settings:
		// - Initial size: 1024 objects
		// - Grow after 1 growth event
		// - Shrink after 1 shrink event
		// - Maximum fill aggressiveness (1.0) - 100%
		// - Refill when 10% empty
		SetFastPathBasicConfigs(1024, 1, 1, 100, 10).
		// Fast path growth strategy:
		// - Exponential growth until 300% of current capacity
		// - 150% fixed growth after exponential phase
		// - 150% growth rate in exponential mode
		SetFastPathGrowthConfigs(300, 250, 150).
		// Fast path shrink strategy:
		// - Shrink by 20% when triggered
		// - Minimum 32 objects
		SetFastPathShrinkConfigs(20, 32).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

func CreateBalancedConfig() *pool.PoolConfig[*Example] {
	poolConfig, err := pool.NewPoolConfigBuilder[*Example]().
		// Basic pool settings:
		// - Initial capacity: 192 objects (balanced for general use)
		// - Max capacity: 10000 objects (reasonable upper limit)
		// - Verbose logging disabled to save resources
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(1024, 10000, true).
		// Ring buffer settings:
		// - Blocking mode enabled for better throughput
		// - No read/write specific timeouts (0)
		// - 3 second general timeout for both operations
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*3).
		// Ring buffer growth strategy:
		// - Exponential growth until 150% of current capacity
		// - 60% growth rate in exponential mode
		// - 40% fixed growth after exponential phase
		SetRingBufferGrowthConfigs(150, 60, 40).
		// Ring buffer shrink strategy:
		// - Check every 5 seconds
		// - Consider idle after 15 seconds from last get call
		// - 2 second cooldown between shrinks
		// - Minimum 3 idle checks before shrinking
		// - 4 stable underutilization rounds
		// - Minimum allowed to shrink to: 24 objects
		// - Maximum allowed consecutive shrinks: 3
		// - Shrink when utilization below 30%
		// - Shrink by 25% when triggered
		SetRingBufferShrinkConfigs(time.Second*10, time.Second*15, 2, 24, 3, 30, 25).
		// Fast path (L1 cache) settings:
		// - Initial size: 192 objects
		// - Grow after 2 growth events
		// - Shrink after 2 shrink events
		// - High fill aggressiveness (0.95) - 95%
		// - Refill when 25% empty
		SetFastPathBasicConfigs(192, 2, 2, 95, 25).
		// Fast path growth strategy:
		// - Exponential growth until 150% of current capacity
		// - 60% fixed growth after exponential phase
		// - 70% growth rate in exponential mode
		SetFastPathGrowthConfigs(150, 160, 70).
		// Fast path shrink strategy:
		// - Shrink by 35% when triggered
		// - Minimum 12 objects
		SetFastPathShrinkConfigs(35, 12).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}
