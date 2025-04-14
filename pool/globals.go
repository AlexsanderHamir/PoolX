package pool

import "time"

var defaultShrinkMap = map[AggressivenessLevel]*shrinkDefaults{
	AggressivenessDisabled: {
		0, 0, 0, 0, 0, 0, 0, 0,
	},
	AggressivenessConservative: {
		5 * time.Minute, 10 * time.Minute, 3, 10 * time.Minute, 0.20, 3, 0.10, 1,
	},
	AggressivenessBalanced: {
		2 * time.Minute, 5 * time.Minute, 2, 5 * time.Minute, 0.30, 3, 0.25, 2,
	},
	AggressivenessAggressive: {
		1 * time.Minute, 2 * time.Minute, 2, 2 * time.Minute, 0.40, 2, 0.50, 3,
	},
	AggressivenessVeryAggressive: {
		30 * time.Second, 1 * time.Minute, 1, 1 * time.Minute, 0.50, 1, 0.65, 4,
	},
	AggressivenessExtreme: {
		10 * time.Second, 20 * time.Second, 1, 30 * time.Second, 0.60, 1, 0.80, 5,
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
	bufferSize:          defaultPoolCapacity,
	fillAggressiveness:  defaultfillAggressiveness,
	refillPercent:       defaultRefillPercent,
	enableChannelGrowth: defaultEnableChannelGrowth,
	growthEventsTrigger: defaultGrowthEventsTrigger,
	shrinkEventsTrigger: defaultShrinkEventsTrigger,
	growth:              defaultGrowthParameters,
	shrink:              defaultShrinkParameters,
}

var defaultRingBufferConfig = &RingBufferConfig{
	block:    false,
	rTimeout: 0,
	wTimeout: 0,
	cancel:   nil,
}
