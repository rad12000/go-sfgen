//go:generate go run ../../... -out-file ../golden/custom_prefix_export.go --struct CustomPrefixExportStruct --prefix MyCustom --export
package gen

type CustomPrefixExportStruct struct {
	Alpha string
	Beta  int
	Gamma bool
}
