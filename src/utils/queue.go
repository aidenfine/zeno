package utils

import "zeno/src/resp"

type Message struct {
	command string
	arg     []resp.Value
}

type Queue[T any] struct {
	items []T
}

func NewMessage(command string, arg []resp.Value) *Message {
	return &Message{
		command: command,
		arg:     arg,
	}
}

func NewQueue[T any]() *Queue[T] {
	return &Queue[T]{
		items: []T{},
	}

}

func (q *Queue[T]) Enqueue(item T) {
	q.items = append(q.items, item)
}
func (q *Queue[T]) Dequeue() (T, bool) {
	if len(q.items) == 0 {
		var zero T
		return zero, false
	}
	item := q.items[0]
	var zero T
	q.items[0] = zero

	q.items = q.items[1:]
	return item, true
}

func (q *Queue[T]) IsEmpty() bool {
	if len(q.items) == 0 {
		return true
	}
	return false
}

func (q *Queue[T]) QueueLength() int {
	return len(q.items)
}
func (q *Queue[T]) ClearQueue() {
	q.items = nil
	q.items = []T{}
}
