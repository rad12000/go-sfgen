//go:generate go run ../../... -out-file ../golden/out_pkg.go --struct OutPkgStruct --out-pkg custompkg --style generic
package gen

type (
	Foo          struct{}
	OutPkgStruct struct {
		Server            string
		Port              int
		ExternalReference Foo // TODO: this should actually result in the correct package being imported
	}
)
