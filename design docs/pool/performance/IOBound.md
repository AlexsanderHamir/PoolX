---
# 🌐 IO-Bound Tasks – Pool Benchmark Report

This benchmark simulates **IO-bound workloads**, where each goroutine performs operations that simulate waiting on external resources (like disk or network). The test is designed to evaluate the pool’s behavior under conditions where **object hold times are long** and **release is delayed**, stressing reuse, growth, and return logic.
---

## 📊 Pool Statistics Overview

### 🧠 Core Pool Metrics

| **Metric**              | **Value** |
| ----------------------- | --------- |
| **Objects In Use**      | 0         |
| **Available Objects**   | 144       |
| **Current Capacity**    | 144       |
| **Peak In Use**         | 50        |
| **Total Gets**          | 250       |
| **Total Puts**          | 250       |
| **Growth Events**       | 2         |
| **Shrink Events**       | 0         |
| **Consecutive Shrinks** | 0         |

---

### ⚙️ Allocation Path Stats

| **Path**                  | **Hits** |
| ------------------------- | -------- |
| **Fast Path (L1)**        | 248      |
| **Slow Path (L2)**        | 0        |
| **Allocator Misses (L3)** | 2        |

---

### 🔁 Fast Return Stats

| **Metric**           | **Value** |
| -------------------- | --------- |
| **Fast Return Hit**  | 73        |
| **Fast Return Miss** | 177       |
| **L2 Spill Rate**    | 70.80%    |

---

### 📈 Efficiency Metrics

| **Metric**              | **Value** |
| ----------------------- | --------- |
| **Utilization %**       | 34.72%    |
| **Requests per Object** | 1.73      |

---

## 🧠 Personal Observations

1. **Initial capacity** should ideally be **3× peak in-use**, but **peak is hard to predict**.
   - In this case, setting capacity to 150 would avoid growth, but the pool instead grew to 216 before stabilizing.
2. **High goroutine concurrency** worsens reuse efficiency, especially with delays.
3. **Return speed matters** — when goroutines issue gets rapidly but return slowly, reuse collapses, and growth kicks in.
4. **Growth policy is unbounded** — this can lead to **memory waste** in prolonged IO-heavy conditions.

---

## 🧪 Test Configuration

```go
var wg sync.WaitGroup

numWorkers := 50
objectsPerWorker := 5
ioDelay := 500 * time.Millisecond

log.Println("[WORKLOAD] Starting IO-bound tasks test")

for i := 0; i < numWorkers; i++ {
	wg.Add(1)
	go func(workerID int) {
		defer wg.Done()
		for j := 0; j < objectsPerWorker; j++ {
			obj := poolObj.Get()
			obj.name = "user_" + randomString(5)
			obj.age = rand.Intn(100)
			obj.friends = make([]string, rand.Intn(5))
			for k := range obj.friends {
				obj.friends[k] = randomString(4)
			}
			time.Sleep(ioDelay)
			poolObj.Put(obj)
		}
	}(i)
}

wg.Wait()
log.Println("[DONE] IO-bound tasks test completed")
```
