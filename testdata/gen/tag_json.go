//go:generate go run ../../... -out-file ../golden/tag_json.go --struct TagJSONStruct --tag json
package gen

type TagJSONStruct struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Age       int    `json:"age"`
	NoTag     string
}
