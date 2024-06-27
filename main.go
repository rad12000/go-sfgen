package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/fatih/structtag"
	"go/types"
	"golang.org/x/tools/go/packages"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

var (
	structName        string
	source            string
	out               string
	outPkg            string
	targetTag         string
	export            bool
	includeStructName bool
	prefix            string
	prefixSet         bool
	includeUnexported bool
)

func main() {
	parseFlags()
	out, err := filepath.Abs(out)
	if err != nil {
		log.Fatalf("cannot parse out filepath %v", err)
	}

	_, err = os.Stat(out)
	if err != nil {
		err = os.MkdirAll(filepath.Dir(out), os.ModeDir)
	}

	if err != nil {
		log.Fatalf("%v", err)
	}

	source, err := filepath.Abs(filepath.Dir(source))
	if err != nil {
		log.Fatalf("failed: %v", err)
	}

	contents, err := parsePackage(source, structName)
	if err != nil {
		log.Fatalf("failed to parse struct: %v", err)
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("package %s\n", outPkg))
	buf.Write(contents)
	if err := os.WriteFile(out, buf.Bytes(), 0666); err != nil {
		log.Fatalf("failed to write to out file: %v", err)
	}

	cmd := exec.Command("go", "fmt", out)
	if err := cmd.Run(); err != nil {
		slog.Warn("failed to format output file", "err", err)
	}
}

func parseFlags() {
	flag.StringVar(&structName, "struct", "", "the struct to generate")
	flag.StringVar(&source, "src", "", "the source")
	flag.StringVar(&out, "out", "", "the output filepath")
	flag.StringVar(&outPkg, "out-pkg", "", "the output package")
	flag.StringVar(&targetTag, "tag", "", "if provided, the name portion of the provided tag will be used")
	flag.StringVar(&prefix, "prefix", "", "if provided, this value will be prepended to the field's name")
	flag.BoolVar(&export, "export", false, "if true, the generated constants will be exported")
	flag.BoolVar(&includeStructName, "include-struct", false, "if true, the generated constants will be prefixed with the source struct's name")
	flag.BoolVar(&includeUnexported, "unexported", false, "if true, the generated constants will include fields that are not exported on the struct")
	flag.Parse()
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "prefix" {
			prefixSet = true
		}
	})

	if structName == "" || source == "" || out == "" || outPkg == "" {
		log.Fatalf("--struct, --src, --out, and --out-pkg must not be empty")
	}
}

func parsePackage(source, structName string) ([]byte, error) {
	_, s, err := loadStruct(source, structName)
	if err != nil {
		return nil, err
	}

	var (
		imports             []string
		sb                  strings.Builder
		maybeCloseConstants = func(i int) {
			if i == s.NumFields()-1 {
				sb.WriteByte(')')
			}
		}
	)

	for i := range s.NumFields() {
		field := s.Field(i)
		if !includeUnexported && !field.Exported() {
			maybeCloseConstants(i)
			continue
		}

		if sb.Len() == 0 {
			sb.WriteString("const (")
		} else {
			sb.WriteByte('\n')
		}

		tag := s.Tag(i)

		constName, value, imp, err := parseField(field, tag)
		if err != nil {
			return nil, err
		}

		if value == "-" { // Handle the case that the field is ignored
			maybeCloseConstants(i)
			continue
		}

		if imp != "" {
			imports = append(imports, imp)
		}

		sb.WriteString(fmt.Sprintf("%s = %q", constName, value))
		maybeCloseConstants(i)
	}

	var buf bytes.Buffer
	//for i, imp := range imports {
	//	if i == 0 {
	//		buf.WriteString("import (")
	//	}
	//
	//	buf.WriteString(fmt.Sprintf("%q", imp))
	//
	//	if i == len(imports)-1 {
	//		buf.WriteString(")\n")
	//	}
	//}

	buf.WriteString(sb.String())
	return buf.Bytes(), nil
}

func parseField(field *types.Var, tag string) (constName, value, imp string, err error) {
	tags, err := structtag.Parse(tag)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse struct tags for field %s: %w", field.Name(), err)
	}

	_, imp = parseTypeName(field.Type().String())
	tagNameValue := field.Name()

	if targetTag != "" {
		nameFromTag, err := tags.Get(targetTag)
		if err == nil && len(nameFromTag.Name) > 0 {
			tagNameValue = nameFromTag.Name
		}
	}

	tagName := strings.ToUpper(targetTag)
	if !includeStructName && !export {
		tagName = strings.ToLower(tagName)
	}

	typeNameR := []rune(structName + tagName)
	if !includeStructName {
		typeNameR = []rune(tagName)
	}

	cName := []rune(string(typeNameR) + "Field" + field.Name())
	if prefixSet {
		cName = []rune(prefix + field.Name())
	}

	if export {
		cName[0] = unicode.ToUpper(cName[0])
	} else {
		cName[0] = unicode.ToLower(cName[0])
	}

	return string(cName), tagNameValue, imp, nil
}

func loadStruct(source, structName string) (*types.Named, *types.Struct, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
	}

	p, err := packages.Load(cfg, source)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load struct package: %w", err)
	}

	if len(p) != 1 {
		return nil, nil, fmt.Errorf("expected 1 package for %s, got %d", source, len(p))
	}

	if len(p[0].Errors) > 0 {
		return nil, nil, fmt.Errorf("%v", p[0].Errors)
	}

	scope := p[0].Types.Scope()
	if scope == nil {
		return nil, nil, fmt.Errorf("couldn't find scope: %w", err)

	}

	foundObj := scope.Lookup(structName) // *types.TypeName is returned here
	if foundObj == nil {
		return nil, nil, fmt.Errorf("type %s not found %s: %w", structName, err)
	}

	n, ok := foundObj.Type().(*types.Named)
	if !ok {
		return nil, nil, fmt.Errorf("cannot use type %s, only named struct types are supported", structName)
	}

	s, ok := n.Underlying().(*types.Struct)
	if !ok {
		return nil, nil, fmt.Errorf("cannot use type %s, only named struct types are supported", structName)
	}

	return n, s, nil
}

func parseTypeName(name string) (fieldType, importPath string) {
	lastForwardSlash := strings.LastIndexByte(name, '/')
	if lastForwardSlash >= 0 {
		fieldType = name[lastForwardSlash+1:]
		dotIndex := strings.LastIndexByte(name, '.')
		return fieldType, name[:dotIndex]
	}

	dotIndex := strings.LastIndexByte(name, '.')
	if dotIndex >= 0 {
		return name, name[:dotIndex]
	}

	return name, ""
}

