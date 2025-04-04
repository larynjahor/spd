package gopackages

func newStack[T any]() *stack[T] {
	return &stack[T]{
		vals: nil,
	}
}

type stack[T any] struct {
	vals []T
}

func (s *stack[T]) Empty() bool {
	return len(s.vals) == 0
}

func (s *stack[T]) Push(v T) {
	s.vals = append(s.vals, v)
}

func (s *stack[T]) Top() T {
	top := s.vals[len(s.vals)-1]

	return top
}

func (s *stack[T]) Values() []T {
	return s.vals
}

func (s *stack[T]) Pop() T {
	top := s.Top()

	s.vals = s.vals[:len(s.vals)-1]

	return top
}
