---
# 🧭 Speed of Arrival – Pool Benchmark Report

This test simulates a **real-world concurrency pattern** where goroutines arrive at **different times** and perform **different types of tasks** (CPU-heavy, I/O-bound, or lightweight). The goal is to evaluate how the object pool handles unpredictable timing, task variance, and uneven load distribution.
---

## 📊 Pool Statistics

### 🧠 Core Metrics

| **Metric**              | **Value** |
| ----------------------- | --------- |
| **Objects In Use**      | 0         |
| **Available Objects**   | 144       |
| **Current Capacity**    | 144       |
| **Peak In Use**         | 70        |
| **Total Gets**          | 500       |
| **Total Puts**          | 500       |
| **Growth Events**       | 2         |
| **Shrink Events**       | 0         |
| **Consecutive Shrinks** | 0         |

---

### ⚡ Allocation Path Stats

| **Path**                  | **Hits** |
| ------------------------- | -------- |
| **Fast Path (L1)**        | 498      |
| **Slow Path (L2)**        | 0        |
| **Allocator Misses (L3)** | 2        |

---

### 🔁 Fast Return Stats

| **Metric**           | **Value** |
| -------------------- | --------- |
| **Fast Return Hit**  | 435       |
| **Fast Return Miss** | 65        |
| **L2 Spill Rate**    | 13.00%    |

---

### 📈 Efficiency Metrics

| **Metric**              | **Value** |
| ----------------------- | --------- |
| **Utilization %**       | 1.39%     |
| **Requests per Object** | 3.47      |

---

## 🧠 Personal Observations

1. **Goroutines arriving at different times** with **varied workloads** (CPU, IO, light) balance demand well.
2. **Requests per Object**:
   - Requests: `500`
   - Available: `144`
   - → **Req per Obj** = `3.47`
   - 📌 Moderate reuse — not too shallow, not highly efficient either.
3. **Get speed vs. Put speed** matters a lot — in async workloads, pool growth often comes from **early gets + delayed puts**.
4. Without **growth bounds**, the pool may over-allocate when tasks get just a little too slow.

---

## 🧪 Test Configuration

```go
var wg sync.WaitGroup

numWorkers := 100
objectsPerWorker := 5

log.Println("[WORKLOAD] Starting speed-of-arrival test")

for i := 0; i < numWorkers; i++ {
	wg.Add(1)

	go func(workerID int) {
		defer wg.Done()

		// Random delay before this goroutine starts (simulates arrival time)
		time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

		for j := 0; j < objectsPerWorker; j++ {
			obj := poolObj.Get()
			obj.name = "user_" + randomString(5)
			obj.age = rand.Intn(100)
			obj.friends = make([]string, rand.Intn(3))
			for k := range obj.friends {
				obj.friends[k] = randomString(4)
			}

			// Randomly select task intensity
			switch rand.Intn(3) {
			case 0: // CPU-heavy
				result := 0
				for n := 0; n < 25_000_000; n++ {
					x := n % 7
					y := n % 13
					z := n % 17
					result += x*x + y*y - z
				}
				_ = result

			case 1: // I/O-bound
				time.Sleep(300 * time.Millisecond)

			case 2: // Light task
				time.Sleep(20 * time.Millisecond)
			}

			poolObj.Put(obj)
		}
	}(i)
}

wg.Wait()
log.Println("[DONE] Speed-of-arrival test completed")

```
