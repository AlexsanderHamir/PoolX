package pool

import (
	"context"
	"time"
)

// PoolConfig defines the interface for pool configuration
type PoolConfig interface {
	GetInitialCapacity() int
	GetHardLimit() int
	GetGrowth() *growthParameters
	GetShrink() *shrinkParameters
	GetFastPath() *fastPathParameters
	GetRingBufferConfig() *RingBufferConfig
	IsVerbose() bool
}

// GrowthParameters defines the interface for growth configuration
type GrowthParameters interface {
	GetExponentialThresholdFactor() float64
	GetGrowthPercent() float64
	GetFixedGrowthFactor() float64
}

// ShrinkParameters defines the interface for shrink configuration
type ShrinkParameters interface {
	GetEnforceCustomConfig() bool
	GetAggressivenessLevel() AggressivenessLevel
	GetCheckInterval() time.Duration
	GetIdleThreshold() time.Duration
	GetMinIdleBeforeShrink() int
	GetShrinkCooldown() time.Duration
	GetMinUtilizationBeforeShrink() float64
	GetStableUnderutilizationRounds() int
	GetShrinkPercent() float64
	GetMaxConsecutiveShrinks() int
	GetMinCapacity() int
}

// FastPathParameters defines the interface for fast path configuration
type FastPathParameters interface {
	GetBufferSize() int
	GetGrowthEventsTrigger() int
	GetShrinkEventsTrigger() int
	GetFillAggressiveness() float64
	GetRefillPercent() float64
	GetGrowth() *growthParameters
	GetShrink() *shrinkParameters
	IsEnableChannelGrowth() bool
}

// RingBufferConfigInterface defines the interface for ring buffer configuration
type RingBufferConfigInterface interface {
	IsBlocking() bool
	GetReadTimeout() time.Duration
	GetWriteTimeout() time.Duration
	GetCancelContext() context.Context
}
