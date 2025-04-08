---
# 🧪 Long-Running Object Pool Test

This test simulates long-lived tasks using a worker pool that acquires and returns objects from a custom object pool. The goal is to measure the efficiency, reuse, and memory behavior under heavy and sustained load.
---

## 📊 Stats

### 🧠 Core Pool Metrics

| **Metric**          | **Value** |
| ------------------- | --------- |
| Objects In Use      | 0         |
| Available Objects   | 144       |
| Current Capacity    | 144       |
| Peak In Use         | 50        |
| Total Gets          | 100       |
| Total Puts          | 100       |
| Growth Events       | 2         |
| Shrink Events       | 0         |
| Consecutive Shrinks | 0         |

---

### ⚙️ Allocation Path Stats

| **Path**              | **Hits** |
| --------------------- | -------- |
| Fast Path (L1)        | 98       |
| Slow Path (L2)        | 0        |
| Allocator Misses (L3) | 2        |

---

### 🔁 Fast Return Stats

| **Metric**       | **Value** |
| ---------------- | --------- |
| Fast Return Hit  | 19        |
| Fast Return Miss | 81        |
| L2 Spill Rate    | 81.00%    |

---

### 📈 Efficiency Metrics

| **Metric**         | **Value** |
| ------------------ | --------- |
| Utilization %      | 34.72%    |
| Request Per object | 0.69      |

---

## 🧠 Personal Observations

📉 _Interpretation_: The longer an object is held, the more requests it takes to justify its existence. Higher concurrency with shorter tasks improves this ratio.

---

## 🧪 Test Configuration

```go
var wg sync.WaitGroup

numWorkers := 50
objectsPerWorker := 2
longTaskDuration := 3 * time.Second

log.Println("[WORKLOAD] Starting long-running tasks test")

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

			time.Sleep(longTaskDuration)

			poolObj.Put(obj)
		}
	}(i)
}

wg.Wait()
log.Println("[DONE] Long-running tasks test completed")
```
