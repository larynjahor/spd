package container

func NewStack[T any](vals ...T) *Stack[T] {
	return &Stack[T]{
		vals: vals,
	}
}

type Stack[T any] struct {
	vals []T
}

func (s *Stack[T]) Empty() bool {
	return len(s.vals) == 0
}

func (s *Stack[T]) Push(v T) {
	s.vals = append(s.vals, v)
}

func (s *Stack[T]) Top() T {
	top := s.vals[len(s.vals)-1]

	return top
}

func (s *Stack[T]) Values() []T {
	return s.vals
}

func (s *Stack[T]) Pop() T {
	top := s.Top()

	s.vals = s.vals[:len(s.vals)-1]

	return top
}
