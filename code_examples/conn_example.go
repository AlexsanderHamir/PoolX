package code_examples

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/AlexsanderHamir/PoolX/v2/pool"
)

type DBConnection struct {
	ID        int64
	Connected bool
	LastUsed  time.Time
}

func DbConnectionExample() {
	poolConfig, err := pool.NewPoolConfigBuilder[*DBConnection]().
		SetPoolBasicConfigs(256, 3000, true).
		SetRingBufferBasicConfigs(true, 0, 0, time.Second*3).
		SetRingBufferGrowthConfigs(5.0, 1, 0.2). // 5.0 (500% => 1,280) threshold, 1 (100%) growth under threshold, 0.2 (20%) growth factor above threshold
		SetRingBufferShrinkConfigs(time.Second*1, time.Second*3, 3, 4, 10, 2, 8).
		SetFastPathBasicConfigs(64, 8, 8, 1.0, 30).
		SetFastPathGrowthConfigs(80.0, 1.2, 0.90).
		SetFastPathShrinkConfigs(60, 3).
		Build()

	if err != nil {
		log.Fatalf("Failed to create pool config: %v", err)
	}

	allocator := func() *DBConnection {
		return &DBConnection{
			ID:        time.Now().UnixNano(),
			Connected: true,
			LastUsed:  time.Now(),
		}
	}

	cleaner := func(conn *DBConnection) {
		conn.Connected = false
		conn.LastUsed = time.Time{}
	}

	connPool, err := pool.NewPool(poolConfig, allocator, cleaner, nil)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}

	http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		conn, err := connPool.Get()
		if err != nil {
			http.Error(w, "Failed to get database connection", http.StatusInternalServerError)
			return
		}

		conn.LastUsed = time.Now()

		time.Sleep(100 * time.Millisecond)

		if err := connPool.Put(conn); err != nil {
			log.Printf("Failed to return connection to pool: %v", err)
		}

		fmt.Fprintf(w, "Query executed using connection ID: %d\n", conn.ID)
	})

	fmt.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
