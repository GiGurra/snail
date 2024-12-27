package util_slice

// DiscardFirstN discards the first n elements from a slice, but the same slice
// is used. This should be used if we don't want a lot of 'dangling heads',
// see https://100go.co/#slices-and-memory-leaks-26
func DiscardFirstN[T any](slice []T, n int) []T {
	if n == 0 {
		return slice
	}

	if n < 0 {
		panic("cannot discard a negative number of elements")
	}

	if n > len(slice) {
		panic("cannot discard more elements than the slice has")
	}

	copy(slice, slice[n:])

	return slice[:len(slice)-n]
}

// DiscardAt discards the element at index i from a slice. It does this by
// shifting all elements after i one step to the left. Then, we return the
// same memory block but with a length that is one less than before.
func DiscardAt[T any](slice []T, i int) []T {
	if i < 0 {
		panic("cannot discard an element at negative index")
	}

	if i >= len(slice) {
		panic("cannot discard an element beyond the slice")
	}

	copy(slice[i:], slice[i+1:])

	return slice[:len(slice)-1]
}
