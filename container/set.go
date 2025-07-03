package container

import "iter"

func NewSet[T comparable](capacity int) *Set[T] {
	return &Set[T]{
		items: make(map[T]struct{}, capacity),
	}
}

type Set[T comparable] struct {
	items map[T]struct{}
}

func (s *Set[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range s.items {
			if !yield(v) {
				return
			}
		}
	}
}

func (s *Set[T]) Delete(val T) {
	delete(s.items, val)
}

func (s *Set[T]) Add(val T) {
	s.items[val] = struct{}{}
}

func (s *Set[T]) Contains(val T) bool {
	_, ok := s.items[val]

	return ok
}

func (s *Set[T]) Slice() []T {
	ret := make([]T, 0, len(s.items))

	for val := range s.items {
		ret = append(ret, val)
	}

	return ret
}
