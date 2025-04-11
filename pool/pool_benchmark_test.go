package pool

import (
	"runtime/debug"
	"testing"
)

var allocator = func() *Example {
	return &Example{}
}

var cleaner = func(e *Example) {
	e.Name = ""
	e.Age = 0
}

func setupPool(b *testing.B) *pool[*Example] {
	config, err := NewPoolConfigBuilder().
		SetShrinkAggressiveness(AggressivenessExtreme).
		Build()
	if err != nil {
		b.Fatalf("Failed to build pool config: %v", err)
	}

	p, err := NewPool(config, allocator, cleaner)
	if err != nil {
		b.Fatalf("Failed to create pool: %v", err)
	}
	return p
}

func BenchmarkPool_Setup(b *testing.B) {
	debug.SetGCPercent(-1)
	b.ReportAllocs()
	InitDefaultFields()

	for i := 0; i < b.N; i++ {
		poolObj := setupPool(b)
		_ = poolObj
	}

}

// b.RunParallel(func(pb *testing.PB) {
// 	for pb.Next() {
// 		pprof.Do(context.Background(), pprof.Labels("op", "put_get"), func(ctx context.Context) {
// 			obj := poolObj.Get()
// 			obj.Name = "user"
// 			obj.Age = 140
// 			poolObj.Put(obj)
// 		})
// 	}
// })
