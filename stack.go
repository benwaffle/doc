package main

type stack[T any] struct {
	items []T
}

func (s *stack[T]) Push(item T) {
	s.items = append(s.items, item)
}

func (s *stack[T]) Pop() T {
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return item
}

func (s *stack[T]) Peek() T {
	return s.items[len(s.items)-1]
}

func (s *stack[T]) Len() int {
	return len(s.items)
}
