package template

import (
	"fmt"
	"os"
	"text/template"
)

func FromFile(filePath string) func(t *template.Template) (*template.Template, error) {
	return func(t *template.Template) (*template.Template, error) {
		fileContents, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file from path %q: %w", filePath, err)
		}

		return t.Parse(string(fileContents))
	}
}
