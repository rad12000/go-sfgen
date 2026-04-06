//go:generate go run ../../... -out-file ../golden/custom_prefix.go --struct CustomPrefixStruct --prefix MyCustom
package gen

type CustomPrefixStruct struct {
	Foo string
	Bar int
	Baz bool
}
