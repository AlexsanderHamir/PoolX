package test

import (
	"testing"
)

func TestMemoryContextConcurrentOperations(t *testing.T) {
	t.Run("ConcurrentAcquireRelease", testConcurrentAcquireRelease)
	t.Run("ConcurrentChildCreation", testConcurrentChildCreation)
	t.Run("ConcurrentPoolOperationsWithChildren", testConcurrentPoolOperationsWithChildren)
	t.Run("ConcurrentClose", testConcurrentClose)
	t.Run("OperationsAfterClose", testOperationsAfterClose)
	t.Run("StressTest", testStressTest)
}
