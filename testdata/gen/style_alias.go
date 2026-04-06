//go:generate go run ../../... -out-file ../golden/style_alias.go --struct AliasStruct --style alias
package gen

type AliasStruct struct {
	FirstName string
	LastName  string
	Email     string
}
