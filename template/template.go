// Package template provides types and functions for executing to templates.
// All templates are provided the [Data] type as its argument.
//
// Each template also has access to the following functions:
//
//   - [Builtin Template Functions]
//
//   - [Sprig Functions]: the `slice` sprig function is renamed to `slice_list` to avoid conflicts with the built-in `slice` function, which has different behavior.
//
//   - func fatalf(format string, args ..any): exits the program with the formatted error message.
//
//   - func to_lower(s string) string: returns the lowercase version of the input string.
//
//   - func require_imports(imps []string): adds the provided imports to the list of required imports for the generated code.
//
// [Sprig Functions]: https://masterminds.github.io/sprig/string_slice.html
// [Builtin Template Functions]: https://pkg.go.dev/text/template#hdr-Functions
package template

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

type Template struct {
	imports      map[string]struct{}
	loadTemplate func() (*template.Template, error)
}

func (t *Template) Execute(out io.Writer, data *Data) error {
	tmpl, err := t.loadTemplate()
	if err != nil {
		return err
	}
	return tmpl.Execute(out, data)
}

func (t *Template) Reset() {
	t.imports = make(map[string]struct{})
}

func (t *Template) Imports() []string {
	result := make([]string, 0, len(t.imports))
	for imp := range t.imports {
		result = append(result, imp)
	}
	return result
}

func New(loadTemplate func(*template.Template) (*template.Template, error)) *Template {
	imports := make(map[string]struct{})
	return &Template{
		imports: imports,
		loadTemplate: sync.OnceValues(func() (*template.Template, error) {
			tmpl := template.New("root")
			sprigFuncs := sprig.HermeticTxtFuncMap()
			sprigFuncs["slice_list"] = sprigFuncs["slice"]
			delete(sprigFuncs, "slice")

			sprigFuncs["fatalf"] = func(format string, args ...any) string {
				log.Fatalf(format, args...)
				return ""
			}
			sprigFuncs["to_lower"] = strings.ToLower
			sprigFuncs["require_imports"] = func(imps []string) string {
				for _, imp := range imps {
					imports[imp] = struct{}{}
				}
				return ""
			}
			tmpl.Funcs(sprigFuncs)

			tmpl, err := loadTemplate(tmpl)
			if err != nil {
				return nil, fmt.Errorf("failed to load template: %w", err)
			}

			return tmpl, nil
		}),
	}
}
