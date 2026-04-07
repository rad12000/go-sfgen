package template

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"text/template"
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
			tmpl.Funcs(template.FuncMap{
				"fatalf": func(format string, args ...any) string {
					log.Fatalf(format, args...)
					return ""
				},
				"to_lower": strings.ToLower,
				"require_imports": func(imps []string) string {
					for _, imp := range imps {
						imports[imp] = struct{}{}
					}
					return ""
				},
			})

			tmpl, err := loadTemplate(tmpl)
			if err != nil {
				return nil, fmt.Errorf("failed to load template: %w", err)
			}

			return tmpl, nil
		}),
	}
}
