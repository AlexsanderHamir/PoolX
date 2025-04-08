Here's your second experiment packaged as a polished **README** that highlights the code, metrics, analysis, and your observations:

---

# 🔄 Resource Contention Test — Object Pool Behavior Under Pressure

This test simulates **resource contention** by introducing a shared bottleneck (e.g., a DB connection pool) to observe how object reuse, pool growth, and performance behave under constrained conditions.

---

## 📊 Stats

### 🧠 Core Pool Metrics

| **Metric**          | **Value** |
| ------------------- | --------- |
| Objects In Use      | 0         |
| Available Objects   | 216       |
| Current Capacity    | 216       |
| Peak In Use         | 100       |
| Total Gets          | 500       |
| Total Puts          | 490       |
| Growth Events       | 3         |
| Shrink Events       | 0         |
| Consecutive Shrinks | 0         |

---

### ⚙️ Allocation Path Stats

| **Path**              | **Hits** |
| --------------------- | -------- |
| Fast Path (L1)        | 497      |
| Slow Path (L2)        | 0        |
| Allocator Misses (L3) | 3        |

---

### 🔁 Fast Return Stats

| **Metric**       | **Value** |
| ---------------- | --------- |
| Fast Return Hit  | 401       |
| Fast Return Miss | 89        |
| L2 Spill Rate    | 18.16%    |

---

### 📈 Efficiency Metrics

| **Metric**         | **Value** |
| ------------------ | --------- |
| Utilization %      | 46.30%    |
| Request Per Object | 2.31      |

---

## 🧠 Personal Observations

1. The number of available objects is constantly 2×/4× bigger than the number of objects at peak, resulting in quite low utilization.

---

## 🧪 Test Configuration

```go
var wg sync.WaitGroup

numWorkers := 100
objectsPerWorker := 5

sharedResource := make(chan struct{}, 5) // Simulates limited external resource

log.Println("[WORKLOAD] Starting resource contention test")

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

			sharedResource <- struct{}{} // acquire resource
			log.Printf("[RESOURCE] Worker %d acquired shared resource", workerID)

			time.Sleep(100 * time.Millisecond) // simulate exclusive work

			<-sharedResource // release resource
			log.Printf("[RESOURCE] Worker %d released shared resource", workerID)

			poolObj.Put(obj)
		}
	}(i)
}

wg.Wait()
log.Println("[DONE] Resource contention test completed")
```
