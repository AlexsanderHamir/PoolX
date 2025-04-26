## Error: available objects (497) does not match current capacity (505)

## Context

1. The error only happens when the pool can grow, but doesn't happen when the hard limit is small.
2. it appears that below 100 hundred capacity we have no errors.
3. The number of total gets match, the number of total returns match, but the number of objects doesn't ?
4. When resizing L1 the test always fails
5. changing from broadcast to signal when waking it readers reduces errors by a significant amount.

## Attempts

- 1. [failed] no shrinking / no l1 resizing
- 2. [failed] checking hardlimit on the wrong stat
- 3. [failed] checking calculation for adding new objects
- 4. [failed] total returns match the number of gets
- 5. [failed] slowPathPut doesn't fail

## Theories
