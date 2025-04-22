package configs

import (
	"time"

	"github.com/AlexsanderHamir/memory_context/pool"
)

// This are just made up examples, you should adjust the configs based on your specific workload.

// creates a configuration optimized for high-throughput workloads.
// This configuration is designed for scenarios where:
// - High concurrent access is expected
// - Low latency is critical
// - Memory usage can be more aggressive
// - Fast object reuse is important
func CreateHighThroughputConfig() pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		// Basic pool settings:
		// - Initial capacity: 128 objects (ring buffer and fast path if not set otherwise)
		// - Max capacity: 5000 objects
		// - Verbose logging enabled
		// - Channel growth enabled for dynamic resizing
		SetPoolBasicConfigs(128, 5000, true, true).
		// Ring buffer settings:
		// - Blocking mode enabled for better throughput
		// - No read/write specific timeouts (0)
		// - 5 second general timeout for both operations
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*5).
		// Ring buffer growth strategy:
		// - Exponential growth until 150% of current capacity
		// - 85% growth rate in exponential mode
		// - 20% fixed growth after exponential phase
		SetRingBufferGrowthConfigs(150.0, 0.85, 1.2).
		// Ring buffer shrink strategy:
		// - Check every 2 seconds
		// - Consider idle after 5 seconds from last get call
		// - 1 second cooldown between shrinks
		// - Minimum 2 idle checks before shrinking (how many times the pool can be idle before shrinking)
		// - 5 stable underutilization rounds (how many times the pool can be underutilized before shrinking)
		// - Minimum allowed to shrink to: 15 objects
		// - Maximum allowed consecutive shrinks: 3
		// - Shrink when utilization below 30%
		// - Shrink by 50% when triggered
		SetRingBufferShrinkConfigs(time.Second*2, time.Second*5, time.Second*1, 2, 5, 15, 3, 0.3, 0.5).
		// Fast path (L1 cache) settings:
		// - Initial size: 128 objects
		// - Grow after 4 growth events (ring buffer growth events)
		// - Shrink after 4 shrink events (ring buffer shrink events)
		// - Maximum fill aggressiveness (1.0) - 100%
		// - Refill when 20% empty
		SetFastPathBasicConfigs(128, 4, 4, 1.0, 0.20).
		// Fast path growth strategy:
		// - Exponential growth until 100% of current capacity
		// - 50% fixed growth after exponential phase
		// - 85% growth rate in exponential mode
		SetFastPathGrowthConfigs(100.0, 1.5, 0.85).
		// Fast path shrink strategy:
		// - Shrink by 70% when triggered
		// - Minimum 5 objects
		SetFastPathShrinkConfigs(0.7, 5).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

// creates a configuration optimized for memory-constrained environments
// with conservative growth and aggressive shrinking.
func CreateMemoryConstrainedConfig() pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		SetPoolBasicConfigs(32, 500, false, true).
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*15).
		SetRingBufferGrowthConfigs(12.0, 0.5, 0.8).
		SetRingBufferShrinkConfigs(time.Second*10, time.Second*30, time.Second*5, 10, 10, 20, 25, 0.2, 0.3).
		SetFastPathBasicConfigs(32, 1, 1, 0.8, 0.10).
		SetFastPathGrowthConfigs(10.0, 1.0, 0.5).
		SetFastPathShrinkConfigs(0.4, 5).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

// creates a configuration optimized for low-latency applications
// with minimal object creation overhead and fast access patterns.
func CreateLowLatencyConfig() pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		SetPoolBasicConfigs(256, 10000, false, true).
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*2).
		SetRingBufferGrowthConfigs(200.0, 1.0, 2.0).
		SetRingBufferShrinkConfigs(time.Second*5, time.Second*10, time.Second*2, 3, 3, 10, 15, 0.8, 0.3).
		SetFastPathBasicConfigs(256, 2, 2, 1.0, 0.30).
		SetFastPathGrowthConfigs(1500.0, 2.0, 1.0).
		SetFastPathShrinkConfigs(0.5, 10).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

// creates a configuration optimized for batch processing workloads
// with predictable object usage patterns and less frequent object creation.
func CreateBatchProcessingConfig() pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		SetPoolBasicConfigs(64, 2000, false, true).
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*10).
		SetRingBufferGrowthConfigs(100.0, 0.7, 1.0).
		SetRingBufferShrinkConfigs(time.Second*5, time.Second*15, time.Second*3, 5, 5, 15, 20, 0.5, 0.4).
		SetFastPathBasicConfigs(64, 3, 3, 0.9, 0.15).
		SetFastPathGrowthConfigs(800.0, 1.2, 0.7).
		SetFastPathShrinkConfigs(0.6, 8).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

// creates a configuration optimized for real-time systems
// with minimal latency and predictable performance.
func CreateRealTimeConfig() pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		SetPoolBasicConfigs(512, 20000, false, true).
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*1).
		SetRingBufferGrowthConfigs(300.0, 1.5, 3.0).
		SetRingBufferShrinkConfigs(time.Second*3, time.Second*8, time.Second*1, 2, 2, 8, 12, 0.9, 0.2).
		SetFastPathBasicConfigs(512, 1, 1, 1.0, 0.40).
		SetFastPathGrowthConfigs(2000.0, 3.0, 1.5).
		SetFastPathShrinkConfigs(0.3, 15).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}

// creates a configuration optimized for general-purpose applications
// with a good balance between performance and resource usage.
func CreateBalancedConfig() pool.PoolConfig {
	poolConfig, err := pool.NewPoolConfigBuilder().
		SetPoolBasicConfigs(96, 3000, false, true).
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*8).
		SetRingBufferGrowthConfigs(120.0, 0.8, 1.5).
		SetRingBufferShrinkConfigs(time.Second*4, time.Second*12, time.Second*2, 4, 4, 12, 18, 0.7, 0.4).
		SetFastPathBasicConfigs(96, 3, 3, 0.95, 0.25).
		SetFastPathGrowthConfigs(1000.0, 1.8, 1.0).
		SetFastPathShrinkConfigs(0.5, 10).
		Build()
	if err != nil {
		panic(err)
	}
	return poolConfig
}
