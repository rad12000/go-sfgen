//go:generate go run ../../... -out-file ../golden/include_struct_name_export.go --struct StructNameExportStruct --include-struct-name --export
package gen

type StructNameExportStruct struct {
	Width  int
	Height int
	Depth  int
}
