package template

import (
	"text/template"

	_ "embed"
)

//go:embed templates/default.gotmpl
var defaultTemplateStr string

func defaultTemplate(t *template.Template) (*template.Template, error) {
	return t.Parse(defaultTemplateStr)
}

func Default() func(*template.Template) (*template.Template, error) {
	return defaultTemplate
}
