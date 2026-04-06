package main

import (
	"fmt"
	"log"
	"text/template"
)

type templateWrapper struct {
	imports  map[string]struct{}
	Template *template.Template
}

func (t *templateWrapper) Imports() []string {
	result := make([]string, 0, len(t.imports))
	for imp := range t.imports {
		result = append(result, imp)
	}
	return result
}

func newTemplateWrapper(parseTemplate func(*template.Template) (*template.Template, error)) (*templateWrapper, error) {
	imports := make(map[string]struct{})
	tmpl := template.New("root")
	tmpl.Funcs(template.FuncMap{
		"fatalf": func(format string, args ...any) string {
			log.Fatalf(format, args...)
			return ""
		},
		"require_imports": func(imps []string) string {
			for _, imp := range imps {
				imports[imp] = struct{}{}
			}
			return ""
		},
	})

	tmpl, err := parseTemplate(tmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse default template: %w", err)
	}

	return &templateWrapper{
		Template: tmpl,
	}, nil
}
