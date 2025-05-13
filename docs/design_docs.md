# Pool Design Documentation

![Flow](../assets/flow.png)

## Overview

This object pool is designed to control object creation based on configurable parameters.

## Architecture

### Core Components

**Fast Path (L1 Cache)**
The ring buffer is optimized for speed but requires a lock on each request. To reduce mutex overhead, we perform fewer, larger requests and fill a channel with objects.

**Main Pool (L2)**
A ring buffer for efficient object storage that can return `nil`, block, or block with a timeout.

**Configuration System**

* Highly configurable with default values for common use cases.
* Uses a builder pattern for easy setup.

**Pool**
The pool manages the fast path and the ring buffer, responsible for growth, shrinking, and other core operations.

### Key Features

* **Adaptive Growth**: Supports both exponential and fixed growth strategies.
* **Intelligent Shrinking**: Configurable shrink behavior based on utilization.
* **Fast Path Optimization**: Channel for high-performance access.
* **Memory Management**: Configurable hard limits and capacity controls.

## Configuration Options

### Pool Configuration

* `initialCapacity`: Starting size of the pool
* `hardLimit`: Maximum number of objects
* `growth`: Growth strategy settings
* `shrink`: Shrink behavior settings
* `fastPath`: L1 cache configuration
* `ringBufferConfig`: Main pool settings

### Growth Parameters

* `exponentialThresholdFactor`: Threshold for switching growth modes
* `growthPercent`: Percentage growth in exponential mode
* `fixedGrowthFactor`: Fixed growth in fixed mode

### Shrink Parameters

* `enforceCustomConfig`: Removes all default settings
* `aggressivenessLevel`: Controls shrink aggressiveness (0-5)
* `checkInterval`: Frequency of shrink checks
* `idleThreshold`: Minimum idle time before shrinking
* `minUtilizationBeforeShrink`: Utilization threshold for shrinking
* `shrinkPercent`: Reduction percentage for pool size
* `maxConsecutiveShrinks`: Limit on consecutive shrink operations
* `minIdleBeforeShrink`: Consecutive idle checks required before shrinking
* `shrinkCooldown`: Minimum time between consecutive shrink operations
* `stableUnderutilizationRounds`: Consecutive underutilization checks before shrinking
* `minCapacity`: Minimum capacity after shrinking

### Fast Path Parameters

* `bufferSize`: L1 cache capacity
* `growthEventsTrigger`: Events required to trigger L1 growth
* `shrinkEventsTrigger`: Events required to trigger L1 shrinking
* `fillAggressiveness`: Refilling aggressiveness
* `refillPercent`: Threshold for L1 refilling
