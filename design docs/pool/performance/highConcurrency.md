---
# 🧪 High-Concurrency Object Pool Load Test

This benchmark simulates a high-concurrency environment to stress-test an object pool implementation. The primary goal is to evaluate **reuse efficiency**, **scalability**, and **memory behavior** under load with minimal work per operation.
---

## 📊 Statistics

### 🧠 Core Pool Metrics

| Metric              | Value |
| ------------------- | ----- |
| Objects In Use      | 0     |
| Available Objects   | 4868  |
| Current Capacity    | 4868  |
| Peak In Use         | 968   |
| Total Gets          | 5000  |
| Total Puts          | 5000  |
| Growth Events       | 75    |
| Shrink Events       | 0     |
| Consecutive Shrinks | 0     |

---

### ⚙️ Allocation Path Stats

| Path                  | Hits |
| --------------------- | ---- |
| Fast Path (L1)        | 4878 |
| Slow Path (L2)        | 47   |
| Allocator Misses (L3) | 75   |

---

### 🔁 Fast Return Stats

| Metric           | Value  |
| ---------------- | ------ |
| Fast Return Hit  | 2031   |
| Fast Return Miss | 2969   |
| L2 Spill Rate    | 59.38% |

---

### 📈 Efficiency Metrics

| Metric                | Value |
| --------------------- | ----- |
| Utilization %         | 0.21% |
| Request Per objects % | 1.67  |

---

## 🧠 Personal Observations

1. **Minimal work inside goroutines** → high object churn, low reuse.
2. **4868 objects** for 5k ops → close to worst-case reuse.
3. **If initial capacity = 3× peak (≈ 3000)**:
   - ✅ No L2/L3 hits.
   - ✅ 100% L1 gets.

---

## ⚠️ Problem Summary

- Massive goroutine fan-out.
- Very fast turnaround on objects.
- Overwhelming pressure on the pool with minimal reuse.
- Pool scaled **almost to match total requests** — low efficiency, high pressure.

---

## 🧪 Test Configuration

```go
var wg sync.WaitGroup

numWorkers := 1000
objectsPerWorker := 5
log.Println("[WORKLOAD] Starting high concurrency load test")

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
			time.Sleep(10 * time.Millisecond)
			poolObj.Put(obj)
		}
	}(i)
}

wg.Wait()
log.Println("[DONE] High concurrency load test completed")
```
