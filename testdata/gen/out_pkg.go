//go:generate go run ../../... -out-file ../golden/out_pkg.go --struct OutPkgStruct --out-pkg custompkg
package gen

type OutPkgStruct struct {
	Server string
	Port   int
}
