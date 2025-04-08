---
# 🚀 Bursty Load – Pool Benchmark Report

This test simulates a **bursty workload** with periods of activity followed by idle time. It stresses the pool’s ability to handle **short spikes** in demand while ensuring memory efficiency and reuse without unnecessary growth.
---

## 📊 Pool Statistics Overview

| **Metric**              | **Value** |
| ----------------------- | --------- |
| **Objects In Use**      | 0         |
| **Available Objects**   | 64        |
| **Current Capacity**    | 64        |
| **Peak In Use**         | 20        |
| **Total Gets**          | 1000      |
| **Total Puts**          | 1000      |
| **Growth Events**       | 0         |
| **Shrink Events**       | 0         |
| **Consecutive Shrinks** | 0         |

---

## ⚡ Allocation Path Stats

| **Path**                  | **Hits** |
| ------------------------- | -------- |
| **Fast Path (L1)**        | 1000     |
| **Slow Path (L2)**        | 0        |
| **Allocator Misses (L3)** | 0        |

---

## 🔁 Fast Return Stats

| **Metric**           | **Value** |
| -------------------- | --------- |
| **Fast Return Hit**  | 1000      |
| **Fast Return Miss** | 0         |
| **L2 Spill Rate**    | 0.00%     |

---

## 📈 Efficiency Metrics

| **Metric**              | **Value** |
| ----------------------- | --------- |
| **Utilization %**       | 31.25%    |
| **Requests per Object** | 15.63     |

---

## 🧠 Personal Observations

1. If the pool starts **smaller than L1**, it will grow unnecessarily.
2. You generally need **3× the peak usage** in L1 size to absorb bursts without growth.
3. If **goroutines return objects quickly** and L1 is not large enough, **Fast Return Hit Rate** will be very high.
4. **Low utilization (<50%)** is not bad — it often reflects readiness, not waste. Objects are reused rapidly and rotated constantly.

---

## 🧪 Test Configuration

```go
var wg sync.WaitGroup

numBursts := 5
burstSize := 20
objectsPerWorker := 10
burstInterval := 2 * time.Second

log.Println("[WORKLOAD] Starting bursty load test")

for burst := 0; burst < numBursts; burst++ {
	log.Printf("[BURST %d] Launching %d workers", burst+1, burstSize)

	for i := 0; i < burstSize; i++ {
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

				time.Sleep(50 * time.Millisecond)
				poolObj.Put(obj)
			}
		}(i + burst*burstSize)
	}

	time.Sleep(burstInterval)
}

wg.Wait()
log.Println("[DONE] Bursty load goroutines completed")
```
