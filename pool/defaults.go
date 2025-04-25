package pool

import "time"



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
	initialSize:         defaultPoolCapacity,
	fillAggressiveness:  defaultfillAggressiveness,
	refillPercent:       defaultRefillPercent,
	enableChannelGrowth: defaultEnableChannelGrowth,
	growthEventsTrigger: defaultGrowthEventsTrigger,
	shrinkEventsTrigger: defaultShrinkEventsTrigger,
	growth:              defaultGrowthParameters,
	shrink:              defaultShrinkParameters,
}

var defaultRingBufferConfig = &ringBufferConfig{
	block:    false,
	rTimeout: 0,
	wTimeout: 0,
}
