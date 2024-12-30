package snail_mem

type AllocatorFunc[T any] func() *T
type DeAllocatorFunc[T any] func(*T)

type MemoryManager[T any] struct {
	Allocator   AllocatorFunc[T]
	DeAllocator DeAllocatorFunc[T]
}

func NewSingleThreadedCircularBufferTestMemMgr[T any](poolSize int) MemoryManager[T] {
	pool := make([]T, poolSize)
	poolIdx := 0
	return MemoryManager[T]{
		Allocator: func() *T {
			res := &pool[poolIdx]
			poolIdx++
			if poolIdx >= poolSize {
				poolIdx = 0
			}
			return res
		},
		DeAllocator: func(r *T) {}, // no op,
	}
}

func NewGoDefaultMemMgr[T any](poolSize int) MemoryManager[T] {
	return MemoryManager[T]{
		Allocator:   func() *T { return new(T) },
		DeAllocator: func(r *T) {}, // no op,
	}
}
