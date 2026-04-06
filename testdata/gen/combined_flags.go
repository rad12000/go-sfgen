//go:generate go run ../../... -out-file ../golden/combined_flags.go --struct CombinedStruct --export --include-struct-name --tag json --style typed --iter
package gen

type CombinedStruct struct {
	ID        int    `json:"id"`
	FullName  string `json:"full_name"`
	IsActive  bool   `json:"is_active"`
	Score     float64 `json:"score"`
}
