## Error: Available objects (497) does not match current capacity (505)

### Context

- The error only occurs when the pool is allowed to grow; it does not happen with a small hard limit.
- No errors are observed when the capacity is below 100.
- The total number of gets and returns match, but the number of objects does not.
- The test always fails when resizing L1.
- Switching from broadcast to signal for waking readers significantly reduces errors.

---

### Key Facts

- There is a 100% spill rate to the ring buffer due to blocked readers. **[IMPORTANT]**

---

### Test Scenario: Growth Allowed + Small Hard Limit (High Contention)

1. 1,000 goroutines attempt to get objects; the remaining 1,000 block.
2. The objects channel closes when the blocked goroutines receive their objects.
3. Blocked goroutines can only proceed if writers return objects.
4. Writers can only return objects after all objects are returned and readers unblock, closing the channel.

---

### Attempts to Fix

1. No shrinking / no L1 resizing — **failed**
2. Checking hard limit on the wrong stat — **failed**
3. Verifying calculation for adding new objects — **failed**
4. Ensuring total returns match total gets — **failed**
5. Checking if `slowPathPut` fails — **failed**

---

### Consistent Errors

- Deadlock occurs without a runtime panic; the system blocks indefinitely.
- Example: Available objects (999) does not match current capacity (1000)
  - Possible causes:
    - Race condition
    - Dropped objects

---

#### Observations After Attempts

- Blocking is more common than the accounting error (ratio: 30:2).

---

### Identified Flaws

- **A. Refill Logic**  
  Returning incorrect failure, causing cascading errors throughout the system. **[Fixed]**
- **B. handleShrinkBlocked**  
  Checking flags outside of a mutex. **[Fixed]**
- **C. Get**  
  Should not return `nil`; must fail instead. **[Fixed]**
- **D. 100% Spill Rate**  
  Need to rethink how goroutines are unblocked. **[Unresolved]**

---

### Tracking & Analysis

- [ ] Take a general look for obvious flaws in the system, related or not to the current problem.

---

### Next Steps

- The problem is on the refill/grow logic.

## Problem
Race conditions & Deadlocks

### Work on the refill logic
1. 

