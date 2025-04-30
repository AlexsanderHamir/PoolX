package code_examples

import "testing"

func TestRunBasicExample(t *testing.T) {
	err := RunBasicExample()
	if err != nil {
		t.Fatalf("failed to run basic example: %v", err)
	}
}
