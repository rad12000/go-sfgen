//go:generate go run ../../... -out-file ../golden/no_style.go --struct NoStyleStruct
package gen

type NoStyleStruct struct {
	Host string
	Port int
	TLS  bool
}
