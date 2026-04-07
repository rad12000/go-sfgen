//go:generate go run ../../... -out-file ../golden/tag_db.go --struct TagDBStruct --tag db
package gen

type TagDBStruct struct {
	UserID    int    `db:"user_id"`
	UserName  string `db:"user_name"`
	Ignored   string `db:"-"`
	NoDBTag   string
	CreatedAt string `db:"created_at"`
}
