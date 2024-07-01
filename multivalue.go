package main

import (
	"fmt"
	"strconv"
)

func NewMultiStringValue() MultiValue[string] {
	return NewMultiValue(func(s string) (string, error) {
		return s, nil
	})
}

func NewMultiBoolValue() MultiValue[bool] {
	return NewMultiValue(func(b string) (bool, error) {
		if b == "" {
			return false, nil
		}
		return strconv.ParseBool(b)
	})
}

func NewMultiValue[T any](parse func(string) (T, error)) MultiValue[T] {
	return MultiValue[T]{parse: parse}
}

type MultiValue[T any] struct {
	values []T
	parse  func(string) (T, error)
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
