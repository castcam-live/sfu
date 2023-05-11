package main

type IterableMap[K comparable, V any] map[K]V

func (m IterableMap[K, V]) Iterate() <-chan V {
	c := make(chan V)
	go func() {
		for _, v := range m {
			c <- v
		}
		close(c)
	}()

	return c
}

type Iterable[K any] interface {
	Iterate() <-chan K
}

func ToSlice[K any](iterable Iterable[K]) []K {
	var slice []K
	for value := range iterable.Iterate() {
		slice = append(slice, value)
	}
	return slice
}
