package main

import (
	"fmt"
)

func NewMultiFlagOptions() MultiValue[FlagOptions] {
	return NewMultiValue(func(s string) (FlagOptions, error) {
		var f FlagOptions
		return f, f.ParseString(s)
	})
}

func NewMultiValue[T any](parse func(string) (T, error)) MultiValue[T] {
	return MultiValue[T]{parse: parse}
}

type MultiValue[T any] struct {
	values []T
	parse  func(string) (T, error)
}

func (m *MultiValue[T]) Slice() []T {
	return m.values
}

func (m *MultiValue[T]) Len() int {
	return len(m.values)
}

func (m *MultiValue[T]) Get(i int) T {
	return m.values[i]
}

func (m *MultiValue[T]) GetOrDefault(i int, t T) T {
	if len(m.values) <= i {
		return t
	}
	return m.values[i]
}

func (m *MultiValue[T]) String() string {
	var t T
	return fmt.Sprintf("[]%T", t)
}

func (m *MultiValue[T]) Set(s string) error {
	v, err := m.parse(s)
	if err != nil {
		return err
	}
	m.values = append(m.values, v)
	return nil
}
