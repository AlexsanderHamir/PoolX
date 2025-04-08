Here's your content structured into a clean and readable **README.md** format:

---

# 🧪 High-Concurrency Short-Task Object Pool Test

This test simulates a high-churn, short-duration workload to evaluate the behavior of an object pool under pressure.

---

## 📊 Stats

### 🧠 **Core Pool Metrics**

| **Metric**          | **Value** |
| ------------------- | --------- |
| Objects In Use      | 0         |
| Available Objects   | 836       |
| Current Capacity    | 836       |
| Peak In Use         | 200       |
| Total Gets          | 2000      |
| Total Puts          | 2000      |
| Growth Events       | 12        |
| Shrink Events       | 0         |
| Consecutive Shrinks | 0         |

---

### ⚙️ **Allocation Path Stats**

| **Path**              | **Hits** |
| --------------------- | -------- |
| Fast Path (L1)        | 1988     |
| Slow Path (L2)        | 0        |
| Allocator Misses (L3) | 12       |

---

### 🔁 **Fast Return Stats**

| **Metric**       | **Value** |
| ---------------- | --------- |
| Fast Return Hit  | 1481      |
| Fast Return Miss | 519       |
| L2 Spill Rate    | 25.95%    |

---

### 📈 **Efficiency Metrics**

| **Metric**         | **Value** |
| ------------------ | --------- |
| Utilization %      | 20.93%    |
| Request Per Object | 2.39      |

## 🧠 Personal Observations

1. 💥 High goroutine count, but short return time avoids contention.
2. 🧊 Object availability is 3×–4× the peak usage — leads to low utilization.

---

## 🧪 Test Configuration

```go
var wg sync.WaitGroup

numWorkers := 200      // Higher number to simulate rapid churn
objectsPerWorker := 10 // Each does 10 short tasks
shortTaskDuration := 5 * time.Millisecond

log.Println("[WORKLOAD] Starting short-running tasks test")

for i := range numWorkers {
	wg.Add(1)

	go func(workerID int) {
		defer wg.Done()

		for j := 0; j < objectsPerWorker; j++ {
			obj := poolObj.Get()
			obj.name = "user_" + randomString(5)
			obj.age = rand.Intn(100)
			obj.friends = make([]string, rand.Intn(3))
			for k := range obj.friends {
				obj.friends[k] = randomString(4)
			}

			time.Sleep(shortTaskDuration)

			poolObj.Put(obj)
		}
	}(i)
}

wg.Wait()
log.Println("[DONE] Short-running tasks test completed")
```
