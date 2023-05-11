package main

type Set[K comparable] map[K]bool

func (s Set[K]) Add(value K) {
	s[value] = true
}

func (s Set[K]) Remove(value K) {
	delete(s, value)
}

func (s Set[K]) Iterate() <-chan K {
	c := make(chan K)
	go func() {
		defer close(c)
		for k := range s {
			c <- k
		}
	}()

	return c
}
