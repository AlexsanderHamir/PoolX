# The Problem With Channels in High-Performance Object Pools

## Overview

Go's channels are a powerful and safe abstraction for inter-goroutine communication. However, in performance-critical systems such as object pools, schedulers, or memory allocators, channels can become a source of latency—even in the so-called "fast path."

This document explains the underlying issue, provides concrete examples of where the bottleneck occurs, and justifies moving toward custom data structures (like atomic ring buffers or lock-free linked lists) for latency-sensitive code.

---

## The Problem With Channels

### Misconception: Channels Are Lock-Free

Although Go channels _appear_ lock-free to the developer, internally they use **synchronization primitives** such as:

- `sync.Mutex` and `sync.Mutex.Lock` / `Unlock`
- Atomic operations for coordination
- Goroutine parking / scheduling for blocking cases

Channels are not implemented using purely lock-free algorithms. Even on the **fast path** (where buffer capacity is available or a receiver is waiting), channel operations involve memory fences and runtime-managed coordination.

### Impact in High-Frequency Systems

In low-latency systems (e.g., in-memory pools, schedulers, DBMS buffer managers), the overhead from synchronization primitives within channels becomes significant—particularly at scale. This results in:

- High **tail latency**
- Reduced **throughput** under contention
- Non-deterministic **runtime performance**

---

## Concrete Case: Fast Path Still Incurs Latency

### Example: `tryFastPathPut`

```go
func (p *Pool[T]) tryFastPathPut(obj T) bool {
	defer func() bool {
		if r := recover(); r != nil {
			err := p.slowPathPut(obj)
			return err == nil
		}
		return true
	}()

	p.mu.RLock()
	chPtr := p.cacheL1
	p.mu.RUnlock()

	ch := *chPtr

	select {
	case ch <- obj:
		p.stats.FastReturnHit.Add(1)
		return true
	default:
		return false
	}
}
```

Although this is the fast path (`default` avoids blocking), the underlying `select` and `channel` mechanisms still:

- Perform **synchronization checks** under the hood
- May hit **false sharing** or **contention** when multiple threads touch channel internals

---

### Example: `tryGetFromL1`

```go
func (p *Pool[T]) tryGetFromL1(locked bool) (zero T, found bool) {
	var chPtr *chan T = p.cacheL1

	if !locked {
		p.mu.RLock()
		chPtr = p.cacheL1
		p.mu.RUnlock()
	}

	select {
	case obj, ok := <-*chPtr:
		if !ok {
			return zero, false
		}
		p.stats.totalGets.Add(1)
		return obj, true
	default:
		return zero, false
	}
}
```

Same story: a fast `select` on a channel still involves runtime machinery and atomic checks, which can introduce non-negligible overhead—especially when channels are shared across goroutines.

---

## Why Not Just Use Channels?

| Feature                | `chan`         | Custom Atomic Structure                 |
| ---------------------- | -------------- | --------------------------------------- |
| Safety                 | ✅ Safe        | ❌ Must handle race conditions manually |
| Blocking support       | ✅ Built-in    | ❌ Requires implementation              |
| Latency under pressure | ❌ High        | ✅ Lower (with good design)             |
| Fast-path performance  | ❌ Can degrade | ✅ Optimized with CAS                   |
| Customizability        | ❌ Limited     | ✅ Fully customizable                   |

---

## Summary

Using channels on the fastpath was a bad technical decision, after everything was optimized, what was supposed to be the fastpath was actually the only bottleneck.
