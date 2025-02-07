## go-sfgen

See https://pkg.go.dev/github.com/rad12000/go-sfgen for detailed documentation.

go-sfgen is a command line tool, designed to be used in `// go:generate` directives. It aims to remove the boilerplate 
of creating `const` values that match tag name values. For example:
```go
package main

type Person struct {
	FullName string `db:"full_name"`
	Age     int     `db:"age"`
}

const (
	DBColFullName = "full_name"
	DBColAge      = "age"
)
```

becomes

```go
// -- main.go --
//go:generate go-sfgen --struct Person --tag db --prefix DBCol --export
package main

type Person struct {
	FullName string `db:"full_name"`
	Age     int     `db:"age"`
}

// -- person_dbcol_generated.go --
const (
	DBColFullName = "full_name"
	DBColAge      = "age"
)
```

it can also generate new types, type aliases, and generic types for type safety:

#### Alias
```go
// -- main.go --
//go:generate go-sfgen --style alias --struct Person --tag db prefix DBCol --export
package main

type Person struct {
	FullName string `db:"full_name"`
	Age     int     `db:"age"`
}

// -- person_dbcol_generated.go --
type DBCol = string
const (
	DBColFullName DBCol = "full_name"
	DBColAge      DBCol = "age"
)
```

#### Type
```go
// -- main.go --
//go:generate go-sfgen --style typed --struct Person --tag db --prefix DBCol --export
package main

type Person struct {
	FullName string `db:"full_name"`
	Age     int     `db:"age"`
}

// -- person_dbcol_generated.go --
type DBCol string
func (d DBCol) String() string {
	return (string)(d)
}

const (
	DBColFullName DBCol = "full_name"
	DBColAge      DBCol = "age"
)
```

#### Generic
```go
// -- main.go --
//go:generate go-sfgen --style typed --struct Person --tag db --prefix DBCol --export
package main

type Person struct {
	FullName string `db:"full_name"`
	Age     int     `db:"age"`
}

// -- person_dbcol_generated.go --
type DBCol[T any] string
func (d DBCol[T]) String() string {
	return (string)(d)
}

const (
	DBColFullName DBCol[string] = "full_name"
	DBColAge      DBCol[int] = "age"
)
```

One can also generate enum-like values from a struct:
```go
// -- main.go --
//go:generate go-sfgen --style typed --iter --struct Person --export
package main

type Person struct {
	FullName string `db:"full_name"`
	Age     int     `db:"age"`
}

// -- person_dbcol_generated.go --
type Field string
func (f Field) String() string {
	return (string)(f)
}

func (F Field) All() [2]string {
	return [2]string{"FullName", "Age"}
}

const (
	FieldFullName Field = "FullName"
	FieldAge      Field = "Age"
)
```

## Overriding Fields Names
Individual field names can be customized via the `sfgen` struct tag. The tag's value is written as `sfgen:"[field_name][,[tag_name:field_name tag_name:field_name etc...]]`.
The `field_name`, if present, will be used as the fieldName in the generated constant, regardless of what we may be generating constants for.
If what is essentially a global override does not fit your use case, the `field_name` may be left empty and followed by a comma. After
the comma, any number of space-separated `tag_name:field_name` key value pairs may be provided. See the examples for more detail.


## More Examples
Look at the examples on pkg.go.dev for more detailed examples.