//go:generate go run ../../... -out-file ../golden/export.go --struct ExportStruct --export
package gen

type ExportStruct struct {
	Username string
	Password string
	Token    string
}
