Here’s your second test written as a polished and structured **README.md** entry — ready to drop into your repo or docs.

---

# 🔬 CPU-Bound Object Pool Test

This benchmark tests how an object pool performs under high CPU pressure with minimal I/O delays. The goal is to observe object reuse, contention, and pool behavior during compute-heavy tasks.

---

## 📊 Stats

### 🧠 **Core Pool Metrics**

| Metric              | Value |
| ------------------- | ----- |
| Objects In Use      | 0     |
| Available Objects   | 64    |
| Current Capacity    | 64    |
| Peak In Use         | 30    |
| Total Gets          | 500   |
| Total Puts          | 500   |
| Growth Events       | 0     |
| Shrink Events       | 0     |
| Consecutive Shrinks | 0     |

---

### ⚙️ **Allocation Path Stats**

| Path                  | Hits |
| --------------------- | ---- |
| Fast Path (L1)        | 500  |
| Slow Path (L2)        | 0    |
| Allocator Misses (L3) | 0    |

---

### 🔁 **Fast Return Stats**

| Metric           | Value |
| ---------------- | ----- |
| Fast Return Hit  | 500   |
| Fast Return Miss | 0     |
| L2 Spill Rate    | 0.00% |

---

### 📈 **Efficiency Metrics**

| Metric             | Value |
| ------------------ | ----- |
| Utilization %      | 4.69% |
| Request Per Object | 7.81  |

---

## 🧠 Personal Observations

1. ⏱️ **Short Delay = Max Reuse**  
   Objects are held briefly, and CPU load dominates — this leads to **100% reuse** and **no contention**.

2. 📐 **Request-to-Object Ratio**:

   - Total Requests: 500
   - Available Objects: 64
   - → `500 / 64` = **7.81 Requests per Object**

3. 🔻 **If CPU Task Duration Increases**:
   - More delay between `Get()` and `Put()`
   - **New Stats**:
     - Available Objects: 216
     - → `500 / 216` = **2.31 Requests per Object**
   - 🧠 **Insight**: Delay is the key limiter — slower returns = fewer reuses = more allocations

---

## 🧪 Test Configuration

```go
var wg sync.WaitGroup

numWorkers := 100          // Reasonable concurrency to saturate CPU cores
objectsPerWorker := 5      // Each goroutine performs 5 compute-heavy tasks
cpuIntensity := 10_000_000 // Number of iterations to simulate heavy CPU load

log.Println("[WORKLOAD] Starting CPU-bound tasks test")

for i := range numWorkers {
	wg.Add(1)

	go func(workerID int) {
		defer wg.Done()

		for range objectsPerWorker {
			obj := poolObj.Get()
			obj.name = "user_" + randomString(5)
			obj.age = rand.Intn(100)
			obj.friends = make([]string, rand.Intn(5))
			for k := range obj.friends {
				obj.friends[k] = randomString(4)
			}

			// Simulate CPU-bound task
			result := 0
			for n := range cpuIntensity {
				result += n % 7
			}

			_ = result // prevent compiler optimizations

			poolObj.Put(obj)
		}
	}(i)
}

wg.Wait()
log.Println("[DONE] CPU-bound tasks test completed")
```
