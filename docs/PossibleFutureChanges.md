# Possible Future Changes

## Performance Optimizations

1. Segmented Ring Buffer Implementation
   - Current behavior: Memory grows by allocating a new bigger slice each time
   - Proposed change: Implement a segmented ring buffer structure
   - Benefits:
     - Amortized cost of growing
     - More efficient memory utilization
     - Reduced memory allocation overhead
   - Implementation approach:
     - Create smaller buffer segments as needed
     - Link segments together in a ring structure
     - Maintain pointers for read/write operations

## Flaws

### Memory Shrinking Mechanism

The current shrinking implementation has a limitation: while there is a limit on consecutive shrink operations, the mechanism doesn't properly account for verification checks. This leads to an inefficient cycle where:

1. The system blocks after reaching the consecutive shrinks limit
2. Get requests continue to trigger verification checks
3. These checks,don't count toward the limit
4. This results in repeated unnecessary checks

TODO: Redesign the shrinking mechanism to:

- Better track both shrink operations and verification checks
- Implement a more efficient blocking strategy
- Reduce unnecessary wake-up cycles
