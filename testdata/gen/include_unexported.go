//go:generate go run ../../... -out-file ../golden/include_unexported.go --struct UnexportedStruct --include-unexported-fields
package gen

type UnexportedStruct struct {
	Public    string
	hidden    int
	secret    bool
	Visible   float64
	internal  string
}
