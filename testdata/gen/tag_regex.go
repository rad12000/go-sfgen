//go:generate go run ../../... -out-file ../golden/tag_regex.go --struct TagRegexStruct --tag db --tag-regex "^([a-z_]+)$"
package gen

type TagRegexStruct struct {
	UserID   int    `db:"user_id"`
	UserName string `db:"user_name"`
	Notes    string `db:"NOTES_UPPER"`
	NoTag    string
}
