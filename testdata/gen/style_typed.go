//go:generate go run ../../... -out-file ../golden/style_typed.go --struct TypedStruct --style typed
package gen

type TypedStruct struct {
	Name    string
	Age     int
	Active  bool
	Balance float64
}
