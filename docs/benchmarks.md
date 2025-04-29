# Memory Profiling Results

## Overview

This document contains memory profiling results for the memory_context package, broken down by methods.

## Get Method Performance

### Memory Allocation Summary

Showing nodes accounting for 293.80MB (100% of total allocated memory)

### Key Memory Consumers

1. Ring Buffer Allocation

   - Memory Used: 125.38MB (42.67%)
   - Source: `github.com/AlexsanderHamir/memory_context/src/pool.NewRingBuffer[go.shape.*uint8]`

2. Pool Initialization
   - Memory Used: 162.50MB (55.31%)
   - Source: `github.com/AlexsanderHamir/memory_context/src/pool.init.func1[Allocator]`

### Analysis

The Get method's memory usage is primarily dominated by two operations:

1. Creating new objects
2. Resizing the ring buffer

PSA: The memory usage comes from these two operations in almost all other methods, everything else was optimized.
