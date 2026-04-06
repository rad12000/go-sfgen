//go:generate go run ../../... -out-file ../golden/sfgen_tag.go --struct SfgenTagStruct --tag json --style typed
package gen

type SfgenTagStruct struct {
	Normal      string `json:"normal_field"`
	Overridden  string `json:"original" sfgen:"custom_override"`
	TagSpecific string `json:"json_val" sfgen:",json:sfgen_json_val"`
	Plain       string
}
