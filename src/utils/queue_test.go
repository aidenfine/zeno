package utils_test

import (
	"testing"
	"zeno/src/utils"
)

func TestQueue(t *testing.T) {
	// make queue of ints
	q := utils.NewQueue[int]()

	for i := range 100 {
		q.Enqueue(i)
	}
	if q.QueueLength() != 100 {
		t.Errorf("Expected queue length to be 100 got %d", q.QueueLength())
	}

	if q.IsEmpty() {
		t.Errorf("Expected queue to not be empty got %t", q.IsEmpty())
	}
	for i := range 100 {
		item, _ := q.Dequeue()
		if item != i {
			t.Errorf("Expected %d got %d", i, item)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Expected queue to be empty got %t", q.IsEmpty())
	}

}
