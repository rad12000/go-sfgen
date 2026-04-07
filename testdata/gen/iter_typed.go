//go:generate go run ../../... -out-file ../golden/iter_typed.go --struct IterTypedStruct --style typed --iter
package gen

type IterTypedStruct struct {
	One   string
	Two   string
	Three string
}
