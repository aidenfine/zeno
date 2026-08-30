package utils_test

import (
	"testing"
	"zeno/src/utils"
)

// BenchmarkQueueEnqueue measures append-backed growth. The queue grows to
// b.Loop()'s iteration count, so this reflects amortized append plus the
// occasional slice reallocation.
func BenchmarkQueueEnqueue(b *testing.B) {
	q := utils.NewQueue[int]()
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		q.Enqueue(i)
		i++
	}
}

// BenchmarkQueueSteadyState alternates enqueue/dequeue to keep the queue at a
// bounded size. Note: Dequeue reslices with q.items = q.items[1:], so the
// backing array is never reclaimed — this benchmark surfaces that the
// underlying storage keeps growing even though the logical length is steady.
func BenchmarkQueueSteadyState(b *testing.B) {
	q := utils.NewQueue[int]()
	// Prime the queue so every iteration has something to dequeue.
	for i := range 1000 {
		q.Enqueue(i)
	}

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		q.Enqueue(i)
		_, _ = q.Dequeue()
		i++
	}
}
