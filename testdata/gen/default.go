//go:generate go run ../../... -out-file ../golden/default.go --struct Simple --style generic
package gen

import "time"

type (
	StringAlias                          = string
	Number[T interface{ int | float64 }] struct {
		V T
	}
)

func (n Number[T]) Value() int {
	return int(n.V)
}

type Simple struct {
	// String has a basic description here
	String  string
	Map     map[string]Simple
	Slice   []Simple
	Channel chan<- Simple
	Array   [3]int
	Generic Number[int]
	Alias   StringAlias
	Imports time.Time
}
