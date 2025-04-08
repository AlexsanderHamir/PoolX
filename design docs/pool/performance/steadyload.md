---
# 🧪 Steady Load – Pool Benchmark Report

This benchmark simulates a **steady workload** in which 100 workers each process 10 objects at a consistent pace. The goal is to evaluate how the object pool handles uniform load without sudden spikes, focusing on reuse efficiency, growth behavior, and return path performance.
---

## 🎯 Core Pool Metrics

| **Metric**              | **Value** |
| ----------------------- | --------- |
| **Objects In Use**      | 0         |
| **Available Objects**   | 216       |
| **Current Capacity**    | 216       |
| **Peak In Use**         | 100       |
| **Total Gets**          | 1000      |
| **Total Puts**          | 1000      |
| **Growth Events**       | 3         |
| **Shrink Events**       | 0         |
| **Consecutive Shrinks** | 0         |

---

## ⚙️ Allocation Path Performance

| **Path**                  | **Hits** |
| ------------------------- | -------- |
| **Fast Path (L1)**        | 997      |
| **Slow Path (L2)**        | 0        |
| **Allocator Misses (L3)** | 3        |

---

## 🔁 Return Path Behavior

| **Metric**           | **Value** |
| -------------------- | --------- |
| **Fast Return Hit**  | 122       |
| **Fast Return Miss** | 878       |
| **L2 Spill Rate**    | 87.80%    |

---

## 📈 Efficiency & Usage Metrics

| **Metric**             | **Value** |
| ---------------------- | --------- |
| **Request per Object** | 4.62      |
| **Utilization %**      | 46.30%    |

---

## 🧠 Personal Observations

1. A total of **216 objects** were created to handle **1000 requests**.
2. The pool **grew 3 times** beyond its initial capacity of 64.
   - If initialized with 216 objects, it would have achieved **100% hit rate** with **no growth**.
3. Peak concurrent usage (**100 objects**) was equal to the number of goroutines, but more objects were needed over time due to lifecycle timing.
4. **Refilling before blocking** ensures consistent fast-path hits and reduces allocator pressure.
5. Since the pool’s **L1 channel size doesn’t scale** with capacity, the majority of returns were forced to spill into the slower internal queue — leading to a **90%+ spill rate**.
6. Despite the low utilization (~46%), this idle capacity appears necessary for maintaining smooth operation and absorbing reuse cycles.
7. **Each object was reused ~4.62 times** on average — a decent reuse ratio, though not maximally efficient.

---

## 🧪 Test Configuration

```go
var wg sync.WaitGroup

numWorkers := 100
objectsPerWorker := 10
delayBetweenTasks := 100 * time.Millisecond

log.Println("[WORKLOAD] Starting steady load goroutines")

for i := 0; i < numWorkers; i++ {
	wg.Add(1)

	go func(id int) {
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


			time.Sleep(delayBetweenTasks)
		}
	}(i)
}

wg.Wait()
log.Println("[DONE] Steady load goroutines completed")
```

---
