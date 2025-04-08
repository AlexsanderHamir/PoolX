---
# ⚙️ Mixed Workload Object Pool Benchmark

This benchmark evaluates how an object pool behaves under a **diverse workload** composed of CPU-bound, I/O-bound, and lightweight tasks. It focuses on assessing reuse efficiency, pool scaling, and allocation path behavior.
---

## 📊 Statistics

### 🧠 Core Pool Metrics

| Metric              | Value |
| ------------------- | ----- |
| Objects In Use      | 0     |
| Available Objects   | 216   |
| Current Capacity    | 216   |
| Peak In Use         | 98    |
| Total Gets          | 500   |
| Total Puts          | 500   |
| Growth Events       | 3     |
| Shrink Events       | 0     |
| Consecutive Shrinks | 0     |

---

### ⚙️ Allocation Path Stats

| Path                  | Hits |
| --------------------- | ---- |
| Fast Path (L1)        | 497  |
| Slow Path (L2)        | 0    |
| Allocator Misses (L3) | 3    |

---

### 🔁 Fast Return Stats

| Metric           | Value  |
| ---------------- | ------ |
| Fast Return Hit  | 404    |
| Fast Return Miss | 96     |
| L2 Spill Rate    | 19.20% |

---

### 📈 Efficiency Metrics

| Metric             | Value |
| ------------------ | ----- |
| Utilization %      | 1.85% |
| Request Per Object | 2.3   |

---

## 🧠 Personal Observations

1. **2 out of 3 tasks** involve delay (IO or CPU), reducing object reuse.

---

## 🧪 Test Configuration

```go
var wg sync.WaitGroup

numWorkers := 100
objectsPerWorker := 5

log.Println("[WORKLOAD] Starting mixed workload test")

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

			// Mixed workload
			taskType := rand.Intn(3)
			switch taskType {
			case 0: // CPU-bound
				result := 0
				for n := 0; n < 25_000_000; n++ {
					x := n % 7
					y := n % 13
					z := n % 17
					result += x*x + y*y - z
				}
				_ = result
			case 1: // IO-bound
				time.Sleep(300 * time.Millisecond)
			case 2: // Lightweight
				time.Sleep(10 * time.Millisecond)
			}

			poolObj.Put(obj)
		}
	}(i)
}

wg.Wait()
log.Println("[DONE] Mixed workload test completed")
```
