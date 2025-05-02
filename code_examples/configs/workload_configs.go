package configs

import (
	"time"

	"github.com/AlexsanderHamir/PoolX/src/pool"
)

func CreateHighThroughputConfig() *pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		// Basic pool settings:
		// - Initial capacity: 256 objects (common for high-throughput systems)
		// - Max capacity: 10000 objects (reasonable upper limit for most systems)
		// - Verbose logging enabled for monitoring
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(256, 10000, true, true, false).
		// Ring buffer settings:
		// - Blocking mode enabled for better throughput
		// - No read/write specific timeouts (0)
		// - 2 second general timeout for both operations
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*2).
		// Ring buffer growth strategy:
		// - Exponential growth until 200% of current capacity
		// - 75% growth rate in exponential mode
		// - 50% fixed growth after exponential phase
		SetRingBufferGrowthConfigs(200.0, 0.75, 1.5).
		// Ring buffer shrink strategy:
		// - Check every 5 seconds
		// - Consider idle after 10 seconds from last get call
		// - 2 second cooldown between shrinks
		// - Minimum 3 idle checks before shrinking
		// - 5 stable underutilization rounds
		// - Minimum allowed to shrink to: 50 objects
		// - Maximum allowed consecutive shrinks: 5
		// - Shrink when utilization below 40%
		// - Shrink by 30% when triggered
		SetRingBufferShrinkConfigs(time.Second*5, time.Second*10, time.Second*2, 3, 5, 50, 5, 0.4, 0.3).
		// Fast path (L1 cache) settings:
		// - Initial size: 256 objects
		// - Grow after 3 growth events
		// - Shrink after 3 shrink events
		// - Maximum fill aggressiveness (1.0) - 100%
		// - Refill when 30% empty
		SetFastPathBasicConfigs(256, 3, 3, 1.0, 0.30).
		// Fast path growth strategy:
		// - Exponential growth until 150% of current capacity
		// - 50% fixed growth after exponential phase
		// - 75% growth rate in exponential mode
		SetFastPathGrowthConfigs(150.0, 1.5, 0.75).
		// Fast path shrink strategy:
		// - Shrink by 40% when triggered
		// - Minimum 20 objects
		SetFastPathShrinkConfigs(0.4, 20).
		SetEnableStats(true).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

func CreateMemoryConstrainedConfig() *pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		// Basic pool settings:
		// - Initial capacity: 16 objects (minimal for memory-constrained systems)
		// - Max capacity: 1000 objects (strict limit for memory-constrained systems)
		// - Verbose logging disabled to save memory
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(16, 1000, false, true, false).
		// Ring buffer settings:
		// - Blocking mode enabled for better memory management
		// - No read/write specific timeouts (0)
		// - 30 second general timeout for both operations
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*30).
		// Ring buffer growth strategy:
		// - Exponential growth until 100% of current capacity
		// - 30% growth rate in exponential mode
		// - 20% fixed growth after exponential phase
		SetRingBufferGrowthConfigs(100.0, 0.3, 1.2).
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
		SetRingBufferShrinkConfigs(time.Second*15, time.Second*60, time.Second*10, 5, 8, 8, 3, 0.2, 0.4).
		// Fast path (L1 cache) settings:
		// - Initial size: 16 objects
		// - Grow after 2 growth events
		// - Shrink after 2 shrink events
		// - Moderate fill aggressiveness (0.7) - 70%
		// - Refill when 50% empty
		SetFastPathBasicConfigs(16, 2, 2, 0.7, 0.50).
		// Fast path growth strategy:
		// - Exponential growth until 100% of current capacity
		// - 30% fixed growth after exponential phase
		// - 40% growth rate in exponential mode
		SetFastPathGrowthConfigs(100.0, 1.3, 0.4).
		// Fast path shrink strategy:
		// - Shrink by 50% when triggered
		// - Minimum 4 objects
		SetFastPathShrinkConfigs(0.5, 4).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

func CreateLowLatencyConfig() *pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		// Basic pool settings:
		// - Initial capacity: 512 objects (larger for low latency)
		// - Max capacity: 20000 objects (higher limit for burst handling)
		// - Verbose logging disabled to minimize overhead
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(512, 20000, false, true, false).
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
		SetRingBufferShrinkConfigs(time.Second*3, time.Second*5, time.Second*1, 2, 3, 32, 3, 0.3, 0.2).
		// Fast path (L1 cache) settings:
		// - Initial size: 512 objects
		// - Grow after 1 growth event
		// - Shrink after 1 shrink event
		// - Maximum fill aggressiveness (1.0) - 100%
		// - Refill when 20% empty
		SetFastPathBasicConfigs(512, 1, 1, 1.0, 0.20).
		// Fast path growth strategy:
		// - Exponential growth until 200% of current capacity
		// - 100% fixed growth after exponential phase
		// - 100% growth rate in exponential mode
		SetFastPathGrowthConfigs(200.0, 2.0, 1.0).
		// Fast path shrink strategy:
		// - Shrink by 30% when triggered
		// - Minimum 16 objects
		SetFastPathShrinkConfigs(0.3, 16).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

func CreateBatchProcessingConfig() *pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		// Basic pool settings:
		// - Initial capacity: 128 objects (balanced for batch processing)
		// - Max capacity: 5000 objects (reasonable for batch workloads)
		// - Verbose logging disabled to save resources
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(128, 5000, false, true, false).
		// Ring buffer settings:
		// - Blocking mode enabled for better throughput
		// - No read/write specific timeouts (0)
		// - 5 second general timeout for both operations
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*5).
		// Ring buffer growth strategy:
		// - Exponential growth until 150% of current capacity
		// - 50% growth rate in exponential mode
		// - 30% fixed growth after exponential phase
		SetRingBufferGrowthConfigs(150.0, 0.5, 1.3).
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
		SetRingBufferShrinkConfigs(time.Second*10, time.Second*30, time.Second*5, 4, 6, 32, 4, 0.4, 0.3).
		// Fast path (L1 cache) settings:
		// - Initial size: 128 objects
		// - Grow after 2 growth events
		// - Shrink after 2 shrink events
		// - High fill aggressiveness (0.9) - 90%
		// - Refill when 30% empty
		SetFastPathBasicConfigs(128, 2, 2, 0.9, 0.30).
		// Fast path growth strategy:
		// - Exponential growth until 150% of current capacity
		// - 50% fixed growth after exponential phase
		// - 60% growth rate in exponential mode
		SetFastPathGrowthConfigs(150.0, 1.5, 0.6).
		// Fast path shrink strategy:
		// - Shrink by 40% when triggered
		// - Minimum 16 objects
		SetFastPathShrinkConfigs(0.4, 16).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

func CreateRealTimeConfig() *pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		// Basic pool settings:
		// - Initial capacity: 1024 objects (large for real-time systems)
		// - Max capacity: 50000 objects (very high for burst handling)
		// - Verbose logging disabled to minimize overhead
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(1024, 50000, false, true, false).
		// Ring buffer settings:
		// - Blocking mode enabled for better latency
		// - No read/write specific timeouts (0)
		// - 500ms general timeout for both operations
		SetRingBufferBasicConfigs(true, 0, 0, time.Millisecond*500).
		// Ring buffer growth strategy:
		// - Exponential growth until 400% of current capacity
		// - 150% growth rate in exponential mode
		// - 100% fixed growth after exponential phase
		SetRingBufferGrowthConfigs(400.0, 1.5, 2.0).
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
		SetRingBufferShrinkConfigs(time.Second*1, time.Second*3, time.Millisecond*500, 2, 2, 64, 2, 0.2, 0.1).
		// Fast path (L1 cache) settings:
		// - Initial size: 1024 objects
		// - Grow after 1 growth event
		// - Shrink after 1 shrink event
		// - Maximum fill aggressiveness (1.0) - 100%
		// - Refill when 10% empty
		SetFastPathBasicConfigs(1024, 1, 1, 1.0, 0.10).
		// Fast path growth strategy:
		// - Exponential growth until 300% of current capacity
		// - 150% fixed growth after exponential phase
		// - 150% growth rate in exponential mode
		SetFastPathGrowthConfigs(300.0, 2.5, 1.5).
		// Fast path shrink strategy:
		// - Shrink by 20% when triggered
		// - Minimum 32 objects
		SetFastPathShrinkConfigs(0.2, 32).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

func CreateBalancedConfig() *pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		// Basic pool settings:
		// - Initial capacity: 192 objects (balanced for general use)
		// - Max capacity: 8000 objects (reasonable upper limit)
		// - Verbose logging disabled to save resources
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(192, 8000, false, true, false).
		// Ring buffer settings:
		// - Blocking mode enabled for better throughput
		// - No read/write specific timeouts (0)
		// - 3 second general timeout for both operations
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*3).
		// Ring buffer growth strategy:
		// - Exponential growth until 150% of current capacity
		// - 60% growth rate in exponential mode
		// - 40% fixed growth after exponential phase
		SetRingBufferGrowthConfigs(150.0, 0.6, 1.4).
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
		SetRingBufferShrinkConfigs(time.Second*5, time.Second*15, time.Second*2, 3, 4, 24, 3, 0.3, 0.25).
		// Fast path (L1 cache) settings:
		// - Initial size: 192 objects
		// - Grow after 2 growth events
		// - Shrink after 2 shrink events
		// - High fill aggressiveness (0.95) - 95%
		// - Refill when 25% empty
		SetFastPathBasicConfigs(192, 2, 2, 0.95, 0.25).
		// Fast path growth strategy:
		// - Exponential growth until 150% of current capacity
		// - 60% fixed growth after exponential phase
		// - 70% growth rate in exponential mode
		SetFastPathGrowthConfigs(150.0, 1.6, 0.7).
		// Fast path shrink strategy:
		// - Shrink by 35% when triggered
		// - Minimum 12 objects
		SetFastPathShrinkConfigs(0.35, 12).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}
