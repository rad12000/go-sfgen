package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/fatih/structtag"
	"go/types"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

func main() {
	err := os.Setenv("GODEBUG", "gotypesalias=1")
	if err != nil {
		log.Fatalf("failed to set GODEBUG variable")
	}
	defer func() {
		_ = os.Unsetenv("GODEBUG")
	}()

	var (
		flagOptions      = parseOptions()
		outputFileGroups = make(map[string][]FlagOptions)
		packageDirs      = make([]string, 0, len(flagOptions))
	)

	for _, fOpt := range flagOptions {
		absSrcDir, err := filepath.Abs(fOpt.SourceStructDir)
		if err != nil {
			log.Fatalf("failed to parse source dir: %s", fOpt.SourceStructDir)
		}
		packageDirs = append(packageDirs, absSrcDir)
		fOpt.SourceStructDir = absSrcDir

		if fOpt.OutputFile == "" {
			fOpt.OutputFile = fmt.Sprintf("%s_%s_generated.go", strings.ToLower(fOpt.SourceStruct), strings.ToLower(calculateBaseName(fOpt)))
		}

		absOutDir, err := filepath.Abs(fOpt.OutputDir)
		if err != nil {
			log.Fatalf("failed to get absolute path to out file %q: %v", fOpt.OutputFile, err)
		}

		absOut := filepath.Join(absOutDir, fOpt.OutputFile)
		fOpt.OutputDir = absOutDir
		fOpt.OutputFile = absOut
		currentOpts := outputFileGroups[absOut]
		if len(currentOpts) > 0 && currentOpts[0].OutputPackage != fOpt.OutputPackage {
			log.Fatalf("invalid package values provided. Cannot use both %q and %q package values within output file %q",
				currentOpts[0].OutputFile, fOpt.OutputPackage, fOpt.OutputFile)
		}
		outputFileGroups[absOut] = append(outputFileGroups[absOut], fOpt)
	}

	loadPackageScopes(packageDirs)

	var wg sync.WaitGroup
	for _, group := range outputFileGroups {
		wg.Add(1)
		go func(group []FlagOptions) {
			defer wg.Done()
			generateCodeForFileGroup(group)
		}(group)
	}

	wg.Wait()
}

func generateCodeForFileGroup(flagOptions []FlagOptions) {
	if len(flagOptions) == 0 {
		return
	}

	var (
		err      error
		outPkg   = flagOptions[0].OutputPackage
		outFile  = flagOptions[0].OutputFile
		outDir   = flagOptions[0].OutputDir
		imports  = make([][]string, len(flagOptions))
		contents = make([][]byte, len(flagOptions))
	)

	for i, fOpt := range flagOptions {
		contents[i], imports[i], err = parsePackage(fOpt)
		if err != nil {
			log.Fatalf("failed to parse struct: %v", err)
		}
	}

	buf := new(bytes.Buffer)
	buf.WriteString("// Code generated by github.com/rad12000/go-sfgen; DO NOT EDIT.\n\n")
	buf.WriteString(fmt.Sprintf("// Source %s.%s:%s\n\n",
		os.Getenv("GOPACKAGE"), os.Getenv("GOFILE"), os.Getenv("GOLINE")))
	buf.WriteString(fmt.Sprintf("package %s\n", outPkg))
	seenImport := make(map[string]struct{})
	hasWrittenImportHeader := false
	for _, imports := range imports {
	InnerLoop:
		for _, imp := range imports {
			if _, ok := seenImport[imp]; ok {
				continue InnerLoop
			}

			seenImport[imp] = struct{}{}
			if !hasWrittenImportHeader {
				buf.WriteString("\nimport (\n")
				hasWrittenImportHeader = true
			}

			buf.WriteByte('"')
			buf.WriteString(imp)
			buf.WriteByte('"')
			buf.WriteByte('\n')
		}

	}
	if hasWrittenImportHeader {
		buf.WriteString(")\n")
	}

	for _, c := range contents {
		buf.Write(c)
		buf.WriteByte('\n')
	}

	if _, err = os.Stat(outFile); err != nil {
		err = os.MkdirAll(outDir, 0755)
	}

	if err != nil {
		log.Fatalf("%v", err)
	}

	file, err := os.OpenFile(outFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to open file at %s: %v", outFile, err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	_ = file.Truncate(0)

	if _, err = file.Write(buf.Bytes()); err != nil {
		log.Fatalf("failed to write to out file %s: %v", outFile, err)
	}

	cmd := exec.Command("go", "fmt", outFile)
	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to run 'go fmt %s'", outFile)
	}
}

func parseOptions() []FlagOptions {
	var (
		commands     = NewMultiFlagOptions()
		topLevelOpts FlagOptions
	)

	flag.Var(&commands, "gen", "accepts all the top level flags in a string, allowing multiple generate commands to be specified")
	topLevelOpts.RegisterFlags(flag.CommandLine)
	flag.Parse()

	var (
		visitedGen    bool
		visitedNonGen bool
	)

	flag.Visit(func(f *flag.Flag) {
		if f.Name == "gen" {
			visitedGen = true
		} else {
			visitedNonGen = true
		}
	})

	if visitedGen && visitedNonGen {
		log.Fatalf("if --gen flags are used, only --gen flags may be provided")
	}

	if visitedGen {
		return commands.Slice()
	}

	if err := topLevelOpts.Validate(); err != nil {
		log.Fatal(err.Error())
	}

	return []FlagOptions{topLevelOpts}
}

func parsePackage(f FlagOptions) (code []byte, imports []string, err error) {
	if f.Iter && f.Style == StyleAlias {
		log.Fatalf("Invalid style %s: only %s and %s styles may be used with the --iter flag", f.Style, StyleGeneric, StyleTyped)
	}

	structType, s, err := loadStruct(f.SourceStructDir, f.SourceStruct)
	if err != nil {
		return nil, nil, err
	}
	structPackage := structType.String()[:strings.LastIndexByte(structType.String(), '.')]

	var (
		outBuf              bytes.Buffer
		constBuf            bytes.Buffer
		maybeCloseConstants = func(i int) {
			if i == s.NumFields()-1 {
				constBuf.WriteByte(')')
			}
		}
	)

	baseName := calculateBaseName(f)
	firstChar := strings.ToLower(baseName[:1])
	switch f.Style {
	case StyleAlias:
		outBuf.WriteString(fmt.Sprintf("type %s = string\n", baseName))
	case StyleTyped:
		outBuf.WriteString(fmt.Sprintf("type %s string\n", baseName))
		outBuf.WriteString(fmt.Sprintf("func (%s %s) String() string { return (string)(%s) }\n", firstChar, baseName, firstChar))
	case StyleGeneric:
		outBuf.WriteString(fmt.Sprintf("type %s[T any] string\n", baseName))
		outBuf.WriteString(fmt.Sprintf("func (%s %s[T]) String() string { return (string)(%s) }\n", firstChar, baseName, firstChar))
	}

	var fieldNames []string
	for i := 0; i < s.NumFields(); i++ {
		field := s.Field(i)
		if !f.IncludeUnexportedFields && !field.Exported() {
			maybeCloseConstants(i)
			continue
		}

		tag := s.Tag(i)
		fieldType, constName, value, imps, err := parseField(structPackage, field, tag, baseName, f)
		if err != nil {
			return nil, nil, err
		}

		if value == "-" { // Handle the case that the field is ignored
			maybeCloseConstants(i)
			continue
		}

		if f.Style == StyleGeneric {
			imports = append(imports, imps...)
		}

		bName := []rune(baseName)
		if f.Export {
			bName[0] = unicode.ToUpper(bName[0])
		} else {
			bName[0] = unicode.ToLower(bName[0])
		}
		baseName = string(bName)

		if constBuf.Len() == 0 {
			constBuf.WriteByte('\n')
			constBuf.WriteString("const (")
		} else {
			constBuf.WriteByte('\n')
		}

		switch f.Style {
		case StyleAlias, StyleTyped:
			constBuf.WriteString(fmt.Sprintf("%s %s = %q", constName, baseName, value))
		case StyleGeneric:
			constBuf.WriteString(fmt.Sprintf("%s %s[%s] = %q", constName, baseName, fieldType, value))
		default:
			constBuf.WriteString(fmt.Sprintf("%s = %q", constName, value))
		}
		fieldNames = append(fieldNames, value)
		maybeCloseConstants(i)
	}

	if f.Iter {
		var sb strings.Builder
		for _, n := range fieldNames {
			sb.WriteByte('"')
			sb.WriteString(n)
			sb.WriteByte('"')
			sb.WriteByte(',')
		}
		fieldNamesStr := sb.String()
		if f.Style == StyleGeneric {
			outBuf.WriteString(fmt.Sprintf("func (%s %s[T]) All() [%d]string { return [%d]string{%s} }\n", firstChar, baseName, len(fieldNames), len(fieldNames), fieldNamesStr))
		} else {
			outBuf.WriteString(fmt.Sprintf("func (%s %s) All() [%d]string { return [%d]string{%s} }\n", firstChar, baseName, len(fieldNames), len(fieldNames), fieldNamesStr))
		}
	}

	if _, err = constBuf.WriteTo(&outBuf); err != nil {
		log.Fatalf("failed to write full contents in memory: %v", err)
	}

	return outBuf.Bytes(), imports, nil
}

func parseField(structPackage string, field *types.Var, tag, baseName string, f FlagOptions) (fieldType, constName, value string, imps []string, err error) {
	tags, err := structtag.Parse(tag)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed to parse struct tags for field %s: %w", field.Name(), err)
	}

	fieldType, imps = parseTypeName(structPackage, field.Type())
	tagNameValue := field.Name()

	if f.Tag != "" {
		nameFromTag, err := tags.Get(f.Tag)
		if err == nil && len(nameFromTag.Name) > 0 {
			tagNameValue = nameFromTag.Name
		}
	}

	return fieldType, baseName + field.Name(), tagNameValue, imps, nil
}

func calculateBaseName(f FlagOptions) string {
	var (
		tagName string
		prefix  string
	)

	if f.UseStructName || f.Export {
		tagName = strings.ToUpper(f.Tag)
	} else {
		tagName = strings.ToLower(f.Tag)
	}

	if f.Prefix == nil {
		prefix = f.SourceStruct + tagName
		if !f.UseStructName {
			prefix = tagName
		}

		prefix += "Field"
	} else {
		prefix = *f.Prefix
	}

	properlyCasedName := []rune(prefix)
	if f.Export {
		properlyCasedName[0] = unicode.ToUpper(properlyCasedName[0])
	} else {
		properlyCasedName[0] = unicode.ToLower(properlyCasedName[0])
	}

	return string(properlyCasedName)
}

func loadStruct(source, structName string) (*types.Named, *types.Struct, error) {
	scope, ok := scopeForPackage(source)
	if !ok {
		var a []string
		for k := range packageNameToScopes {
			a = append(a, k)
		}
		return nil, nil, fmt.Errorf("failed to find package scope: %s, %+v", source, a)
	}

	foundObj := scope.Lookup(structName) // *types.TypeName is returned here
	if foundObj == nil {
		return nil, nil, fmt.Errorf("type %s not found in package %s", structName, source)
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

func parseTypeName(structPackage string, t types.Type) (fieldType string, importPath []string) {
	switch u := t.(type) {
	case *types.Basic:
		return u.Name(), nil
	case *types.Slice:
		sliceElemType, imports := parseTypeName(structPackage, u.Elem())
		return fmt.Sprintf("[]%s", sliceElemType), imports
	case *types.Array:
		arrElemType, imports := parseTypeName(structPackage, u.Elem())
		return fmt.Sprintf("[%d]%s", u.Len(), arrElemType), imports
	case *types.Chan:
		chanElemType, imports := parseTypeName(structPackage, u.Elem())
		switch u.Dir() {
		case types.SendOnly:
			return fmt.Sprintf("chan <- %s", chanElemType), imports
		case types.RecvOnly:
			return fmt.Sprintf("<-chan %s", chanElemType), imports
		case types.SendRecv:
			return fmt.Sprintf("chan %s", chanElemType), imports
		}
	case *types.Pointer:
		elemType, imports := parseTypeName(structPackage, u.Elem())
		return fmt.Sprintf("*%s", elemType), imports
	case *types.Map:
		key, keyImps := parseTypeName(structPackage, u.Key())
		val, valImps := parseTypeName(structPackage, u.Elem())
		return fmt.Sprintf("map[%s]%s", key, val), append(keyImps, valImps...)
	case *types.Signature:
		return parseTypeNameSignature(structPackage, u)
	case *types.TypeParam:
		return "any", nil
	case *types.Alias, *types.Named:
		return parseNamedType(structPackage, u)
	default:
		log.Fatalf("unhandled type %T: %s", t, t)
	}
	return "", nil
}

func parseNamedType(structPackage string, u types.Type) (string, []string) {
	name := u.String()
	dotIndex := strings.LastIndexByte(name, '.')
	pkgPath := name[:dotIndex]
	if pkgPath == structPackage {
		return name[dotIndex+1:], nil
	}

	slashIndex := strings.LastIndexByte(name, '/')
	return name[slashIndex+1:], []string{name[:dotIndex]}
}

func parseTypeNameSignature(structPackage string, u *types.Signature) (string, []string) {
	var (
		sb      strings.Builder
		imports []string
	)

	sb.WriteString("func (")
	for i := 0; i < u.Params().Len(); i++ {
		param := u.Params().At(i)
		paramType, imps := parseTypeName(structPackage, param.Type())
		imports = append(imports, imps...)
		if i > 0 && i < u.Params().Len() {
			sb.WriteByte(',')

		}
		sb.WriteString(paramType)
	}
	sb.WriteByte(')')

	if u.Results().Len() > 1 {
		sb.WriteByte('(')
	}
	for i := 0; i < u.Results().Len(); i++ {
		param := u.Results().At(i)
		paramType, imps := parseTypeName(structPackage, param.Type())
		imports = append(imports, imps...)
		if i > 0 && i < u.Results().Len() {
			sb.WriteByte(',')

		}
		sb.WriteString(paramType)
	}
	if u.Results().Len() > 1 {
		sb.WriteByte(')')
	}

	return sb.String(), imports
}
