//go:generate go run ../../... -out-file ../golden/iter_generic.go --struct IterGenericStruct --style generic --iter
package gen

type IterGenericStruct struct {
	Name  string
	Count int
	Flag  bool
}
