package test

import (
	"sync"
	"testing"
	"time"

	"github.com/AlexsanderHamir/PoolX/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperationsOnNewPoolAfterClose(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(100).
		SetHardLimit(1000).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)

	err = p.Close()
	require.NoError(t, err)

	obj, _ := p.Get()
	assert.Nil(t, obj)

	newPool := createTestPool(t, config)
	defer func() {
		require.NoError(t, newPool.Close())
	}()

	obj, err = newPool.Get()
	require.NoError(t, err)
	require.NotNil(t, obj)

	err = newPool.Put(obj)
	require.NoError(t, err)
}

func TestResourceExhaustion(t *testing.T) {
	config, err := pool.NewPoolConfigBuilder().
		SetInitialCapacity(10).
		SetHardLimit(20).
		SetMinShrinkCapacity(10).
		Build()
	require.NoError(t, err)

	p := createTestPool(t, config)
	defer func() {
		require.NoError(t, p.Close())
	}()

	objects := make([]*TestObject, 20)
	for i := range objects {
		objects[i], err = p.Get()
		require.NoError(t, err)
		require.NotNil(t, objects[i])
	}

	obj, _ := p.Get()
	assert.Nil(t, obj)

	for i := range 10 {
		err := p.Put(objects[i])
		require.NoError(t, err)
	}

	obj, err = p.Get()
	require.NoError(t, err)
	require.NotNil(t, obj)
	err = p.Put(obj)
	require.NoError(t, err)

	for i := 10; i < 20; i++ {
		err := p.Put(objects[i])
		require.NoError(t, err)
	}
}

func TestErrorHandlingScenarios(t *testing.T) {
	t.Run("closed pool operations", func(t *testing.T) {
		config, err := pool.NewPoolConfigBuilder().
			SetInitialCapacity(10).
			SetHardLimit(20).
			SetMinShrinkCapacity(10).
			Build()
		require.NoError(t, err)

		p := createTestPool(t, config)
		err = p.Close()
		require.NoError(t, err)

		obj, err := p.Get()
		require.Nil(t, obj)
		require.Nil(t, err)

		err = p.Put(&TestObject{Value: 42})
		require.Equal(t, "ring buffer failed core operation: EOF", err.Error())
	})

	t.Run("concurrent error recovery", func(t *testing.T) {
		config, err := pool.NewPoolConfigBuilder().
			SetInitialCapacity(10).
			SetMinShrinkCapacity(10).
			SetHardLimit(20).
			Build()
		require.NoError(t, err)

		p := createTestPool(t, config)
		defer func() {
			require.NoError(t, p.Close())
		}()

		numGoroutines := 10
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for range numGoroutines {
			go func() {
				defer wg.Done()
				for range 5 {
					obj, err := p.Get()
					if err != nil {
						continue
					}
					if obj != nil {
						time.Sleep(time.Millisecond)
						_ = p.Put(obj)
					}
				}
			}()
		}

		wg.Wait()

		obj, err := p.Get()
		require.NoError(t, err)
		require.NotNil(t, obj)
		err = p.Put(obj)
		require.NoError(t, err)
	})
}
