//go:generate go run ../../... -out-file ../golden/export_with_tag.go --struct ExportTagStruct --export --tag json
package gen

type ExportTagStruct struct {
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
	UserAge   int    `json:"user_age"`
}
