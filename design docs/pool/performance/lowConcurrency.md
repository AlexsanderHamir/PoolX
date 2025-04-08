---
# 🧪 Low Concurrency Pool Load Test

This benchmark simulates a **low-concurrency workload** to measure object pool behavior and efficiency under minimal pressure.
---

## 📊 Stats

### 🧠 **Core Pool Metrics**

| **Metric**          | **Value** |
| ------------------- | --------- |
| Objects In Use      | 0         |
| Available Objects   | 20        |
| Current Capacity    | 20        |
| Peak In Use         | 10        |
| Total Gets          | 50        |
| Total Puts          | 50        |
| Growth Events       | 0         |
| Shrink Events       | 0         |
| Consecutive Shrinks | 0         |

---

### ⚙️ **Allocation Path Stats**

| **Path**              | **Hits** |
| --------------------- | -------- |
| Fast Path (L1)        | 50       |
| Slow Path (L2)        | 0        |
| Allocator Misses (L3) | 0        |

---

### 🔁 **Fast Return Stats**

| **Metric**       | **Value** |
| ---------------- | --------- |
| Fast Return Hit  | 50        |
| Fast Return Miss | 0         |
| L2 Spill Rate    | 0.00%     |

---

### 📈 **Efficiency Metrics**

| **Metric**         | **Value** |
| ------------------ | --------- |
| Utilization %      | 50.00%    |
| Request Per Object | 2.5       |

---

## 🧠 Personal Observations

1. In **low-concurrency** environments, a pool only needs ~**2× peak usage** to avoid allocation.
2. The **delay between tasks** simulates actual work. In real applications, this delay might be much shorter, increasing system pressure (higher **TRTAOR**).

---

## 🧪 Test Configuration

```go
var wg sync.WaitGroup

numWorkers := 10               // Low number of concurrent goroutines
objectsPerWorker := 5          // Each goroutine performs 5 get/put operations
delayBetweenTasks := 500 * time.Millisecond // Slower task rate

log.Println("[WORKLOAD] Starting low concurrency load test")

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

			// Simulate slow processing
			time.Sleep(100 * time.Millisecond)

			poolObj.Put(obj)

			// Simulate delay between each task
			time.Sleep(delayBetweenTasks)
		}
	}(i)
}

wg.Wait()
log.Println("[DONE] Low concurrency load test completed")
```
