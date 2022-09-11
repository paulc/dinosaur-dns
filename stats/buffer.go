package stats

import (
	"sync"
)

type CircularBuffer[T any] struct {
	sync.RWMutex
	buffer   []T
	capacity int
	position int
	length   int
}

func NewCircularBuffer[T any](capacity int) *CircularBuffer[T] {
	return &CircularBuffer[T]{buffer: make([]T, capacity), capacity: capacity, position: 0}
}

func (b *CircularBuffer[T]) Insert(item T) {
	b.Lock()
	defer b.Unlock()
	b.buffer[b.position] = item
	b.position = (b.position + 1) % b.capacity
	if b.length < b.capacity {
		b.length++
	}
}

func (b *CircularBuffer[T]) GetOffset(offset, count int) (res []T) {
	b.Lock()
	defer b.Unlock()
	if offset > b.length {
		return
	}
	if count+offset > b.length {
		count = b.length - offset
	}
	start := b.position
	if b.length < b.capacity {
		// We havent filled full buffer yet so index starts at 0
		start = 0
	}
	for i := offset; i < offset+count; i++ {
		res = append(res, b.buffer[(start+i)%b.capacity])
	}
	return
}

func (b *CircularBuffer[T]) Get(n int) (res []T) {
	return b.GetOffset(0, n)
}

func (b *CircularBuffer[T]) GetAll() (res []T) {
	return b.GetOffset(0, b.length)
}
