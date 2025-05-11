package pool

import (
	"time"
)

type AggressivenessLevel int

const (
	defaultAggressiveness             AggressivenessLevel = AggressivenessBalanced
	AggressivenessDisabled            AggressivenessLevel = 0
	AggressivenessConservative        AggressivenessLevel = 1
	AggressivenessBalanced            AggressivenessLevel = 2
	AggressivenessAggressive          AggressivenessLevel = 3
	AggressivenessVeryAggressive      AggressivenessLevel = 4
	AggressivenessExtreme             AggressivenessLevel = 5
	defaultExponentialThresholdFactor                     = 1.0 // 100%
	defaultGrowthFactor                                   = 0.5 // 50%
	defaultFixedGrowthFactor                              = 0.1 // 10%
	defaultfillAggressiveness                             = 80
	fillAggressivenessExtreme                             = 100
	defaultRefillPercent                                  = 20
	defaultMinCapacity                                    = 32
	defaultPoolCapacity                                   = 64
	defaultL1MinCapacity                                  = defaultPoolCapacity // L1 doesn't go below its initial capacity
	defaultHardLimit                                      = 10_000
	defaultGrowthEventsTrigger                            = 3
	defaultShrinkEventsTrigger                            = 3
	defaultPreReadBlockHookAttempts                       = 3
	defaultEnableChannelGrowth                            = true
	defaultEnableStats                                    = false
	Block                                                 = false
	RTimeout                                              = 0
	WTimeout                                              = 0
)

var defaultShrinkMap = map[AggressivenessLevel]*shrinkDefaults{
	AggressivenessDisabled: {
		interval:      0,
		cooldown:      0,
		utilization:   0,
		underutilized: 0,
		percent:       0,
		maxShrinks:    0,
	},
	AggressivenessConservative: {
		interval:      15 * time.Minute,
		cooldown:      30 * time.Minute,
		utilization:   15,
		underutilized: 5,
		percent:       5,
		maxShrinks:    1,
	},
	AggressivenessBalanced: {
		interval:      10 * time.Minute,
		cooldown:      20 * time.Minute,
		utilization:   25,
		underutilized: 4,
		percent:       15,
		maxShrinks:    2,
	},
	AggressivenessAggressive: {
		interval:      5 * time.Minute,
		cooldown:      10 * time.Minute,
		utilization:   35,
		underutilized: 3,
		percent:       30,
		maxShrinks:    3,
	},
	AggressivenessVeryAggressive: {
		interval:      2 * time.Minute,
		cooldown:      5 * time.Minute,
		utilization:   45,
		underutilized: 2,
		percent:       50,
		maxShrinks:    4,
	},
	AggressivenessExtreme: {
		interval:      1 * time.Minute,
		cooldown:      2 * time.Minute,
		utilization:   55,
		underutilized: 1,
		percent:       70,
		maxShrinks:    5,
	},
}

var defaultGrowthParameters = &growthParameters{
	thresholdFactor:        defaultExponentialThresholdFactor,
	bigGrowthFactor:        defaultGrowthFactor,
	controlledGrowthFactor: defaultFixedGrowthFactor,
}

var defaultShrinkParameters = &shrinkParameters{
	aggressivenessLevel: defaultAggressiveness,
}

var defaultFastPath = &fastPathParameters{
	initialSize:              defaultPoolCapacity,
	fillAggressiveness:       defaultfillAggressiveness,
	refillPercent:            defaultRefillPercent,
	enableChannelGrowth:      defaultEnableChannelGrowth,
	growthEventsTrigger:      defaultGrowthEventsTrigger,
	shrinkEventsTrigger:      defaultShrinkEventsTrigger,
	preReadBlockHookAttempts: defaultPreReadBlockHookAttempts,
	growth:                   defaultGrowthParameters,
	shrink:                   defaultShrinkParameters,
}

var defaultAllocationStrategy = &AllocationStrategy{
	AllocPercent: 30,
	AllocAmount:  30,
}
