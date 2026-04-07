//go:generate go run ../../... -out-file ../golden/embedded_struct.go --struct ChildStruct --style typed
package gen

type ParentStruct struct {
	ParentField string
	SharedField string
}

type ChildStruct struct {
	ParentStruct
	ChildField  string
	SharedField string
}
