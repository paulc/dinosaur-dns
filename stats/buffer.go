package stats

import (
	"sync"
)

// Generic thread safe circular buffer
type CircularBuffer[T any] struct {
	sync.RWMutex
	hooks    map[string]func(T)
	buffer   []T
	capacity int
	position int
	length   int
}

// Constructor
func NewCircularBuffer[T any](capacity int) *CircularBuffer[T] {
	return &CircularBuffer[T]{buffer: make([]T, capacity), hooks: make(map[string]func(T)), capacity: capacity, position: 0}
}

// Insert item
func (b *CircularBuffer[T]) Insert(item T) {
	b.Lock()
	defer b.Unlock()
	b.buffer[b.position] = item
	b.position = (b.position + 1) % b.capacity
	// Track how full buffer is
	if b.length < b.capacity {
		b.length++
	}
	// Call hooks when items inserted
	for _, v := range b.hooks {
		v(item)
	}
}

// Manage hook functions
func (b *CircularBuffer[T]) AddHook(id string, f func(T)) {
	b.hooks[id] = f
}

func (b *CircularBuffer[T]) DeleteHook(id string) {
	delete(b.hooks, id)
}

// Get items from head of buffer (with optional offset)
func (b *CircularBuffer[T]) GetOffset(offset, count int) (res []T) {
	b.Lock()
	defer b.Unlock()
	if offset > b.length || count <= 0 {
		return
	}
	if count+offset > b.length {
		count = b.length - offset
	}
	// Tail of buffer is at t.position (unless not full)
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

// Get from head of buffer
func (b *CircularBuffer[T]) Get(count int) (res []T) {
	return b.GetOffset(0, count)
}

// Get all from head of buffer
func (b *CircularBuffer[T]) GetAll() (res []T) {
	return b.GetOffset(0, b.length)
}

// Get from tail of buffer
func (b *CircularBuffer[T]) Tail(count int) (res []T) {
	b.Lock()
	defer b.Unlock()
	if count > b.length {
		count = b.length
	}
	// b.position is insertion point so need to step backwards from b.position - 1
	for i := 1; i <= count; i++ {
		pos := b.position - i
		if pos < 0 {
			pos = b.length + pos
		}
		res = append(res, b.buffer[pos])
	}
	return
}
