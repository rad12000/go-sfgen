//go:generate go run ../../... -out-file ../golden/include_struct_name.go --struct StructNameStruct --include-struct-name
package gen

type StructNameStruct struct {
	ID     int
	Label  string
	Status string
}
