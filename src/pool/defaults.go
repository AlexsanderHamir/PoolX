package pool

import (
	"time"

	config "github.com/AlexsanderHamir/ringbuffer/config"
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
	defaultExponentialThresholdFactor                     = 100.0
	defaultGrowthPercent                                  = 0.5
	defaultFixedGrowthFactor                              = 1.5
	defaultfillAggressiveness                             = 0.8
	fillAggressivenessExtreme                             = 1.0
	defaultRefillPercent                                  = 0.2
	defaultMinCapacity                                    = 32
	defaultPoolCapacity                                   = 64
	defaultL1MinCapacity                                  = defaultPoolCapacity // L1 doesn't go below its initial capacity
	defaultHardLimit                                      = 10_000
	defaultGrowthEventsTrigger                            = 3
	defaultShrinkEventsTrigger                            = 3
	defaultPreReadBlockHookAttempts                       = 3
	defaultEnableChannelGrowth                            = true
	defaultEnableStats                                    = false
)

var defaultShrinkMap = map[AggressivenessLevel]*shrinkDefaults{
	AggressivenessDisabled: {
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	AggressivenessConservative: {
		15 * time.Minute, 30 * time.Minute, 5, 30 * time.Minute, 0.15, 5, 0.05, 1,
	},
	AggressivenessBalanced: {
		10 * time.Minute, 20 * time.Minute, 4, 20 * time.Minute, 0.25, 4, 0.15, 2,
	},
	AggressivenessAggressive: {
		5 * time.Minute, 10 * time.Minute, 3, 10 * time.Minute, 0.35, 3, 0.30, 3,
	},
	AggressivenessVeryAggressive: {
		2 * time.Minute, 5 * time.Minute, 2, 5 * time.Minute, 0.45, 2, 0.50, 4,
	},
	AggressivenessExtreme: {
		1 * time.Minute, 2 * time.Minute, 1, 2 * time.Minute, 0.55, 1, 0.70, 5,
	},
}

var defaultGrowthParameters = &growthParameters{
	exponentialThresholdFactor: defaultExponentialThresholdFactor,
	growthPercent:              defaultGrowthPercent,
	fixedGrowthFactor:          defaultFixedGrowthFactor,
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

var defaultRingBufferConfig = &config.RingBufferConfig{
	Block:    false,
	RTimeout: 0,
	WTimeout: 0,
}
