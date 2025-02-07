/*
go-sfgen generates constants from struct fields.

Below is a list of flags that can be used with the //go:generate directive.

Usage:

	go-sfgen --struct [struct_name] [flags]

Flags are:

	-dry-run
		If true, no output file will be written to, but instead results will be written to stdout
	-export
		If true, the generated constants will be exported
	-gen value
		accepts all the top level flags in a string, allowing multiple generate commands to be specified
	-include-struct-name
		If true, the generated constants will be prefixed with the source struct name
	-include-unexported-fields
		If true, the generated constants will include fields that are not exported on the struct
	-iter
		if true, an All() method will be generated for the type, which returns an array of all the values generated
	-out-dir string
		The directory in which to place the generated file. Defaults to the current directory (default ".")
	-out-file string
		The file to write generated output to. Defaults to [--struct]_[prefix]_generated.go
	-out-pkg string
		The package the generated code should belong to. Defaults to the package containing the go:generate directive
	-package string
		The name of the package in which the source struct resides.
	-prefix value
		A value to prepend to the generated const names. Defaults to [tag]Field
	-src-dir string
		The directory containing the --struct. Defaults to the current directory (default ".")
	-struct string
		The struct to use as the source for code generation. REQUIRED
	-style string
		Specifies the style of constants desired. Valid options are: alias, typed, generic
	-tag string
		If provided, the provided tag will be parsed for each field on the --struct.
		If the tag is missing, the struct field's name is used.
		Otherwise, the first attribute in the tag is used as the name'
	-tag-regex string
		This flag requires the --tag flag be provided as well.
		The provided regex will be tested on the specified tag contents for each field.
		The first capture group will be used as the value for the generated constant.
		If the regex does not match the tag contents, the struct field's' name will be used instead.
	-tests
		If true, source code in tests will be included. This flag will often need to be used along with the --package flag.
*/
package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/fatih/structtag"
	"go/format"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

var flagOptions []FlagOptions

func main() {
	flagOptions = parseOptions()
	err := os.Setenv("GODEBUG", "gotypesalias=1")
	if err != nil {
		log.Fatalf("failed to set GODEBUG variable")
	}
	defer func() {
		_ = os.Unsetenv("GODEBUG")
	}()

	var (
		outputFileGroups = make(map[string][]FlagOptions)
		packagesToLoad   = make([]packageToLoad, 0, len(flagOptions))
	)

	for _, fOpt := range flagOptions {
		absSrcDir, err := filepath.Abs(fOpt.SourceStructDir)
		if err != nil {
			log.Fatalf("failed to parse source dir: %s", fOpt.SourceStructDir)
		}
		pkgToLoad := packageToLoad{Dir: absSrcDir, IncludeTests: fOpt.IncludeTests, PackageName: fOpt.PackageName}
		packagesToLoad = append(packagesToLoad, pkgToLoad)
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

		fOpt.packagesToLoad = pkgToLoad
		outputFileGroups[absOut] = append(outputFileGroups[absOut], fOpt)
	}

	loadPackageScopes(packagesToLoad)

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
		dryRun   = flagOptions[0].DryRun
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

	if dryRun {
		printDryRun(buf.Bytes())
		return
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

	out, err := format.Source(buf.Bytes())
	if err != nil {
		panic(fmt.Sprintf("failed to format output '%v'", err))
	}

	if _, err = file.Write(out); err != nil {
		log.Fatalf("failed to write to out file %s: %v", outFile, err)
	}
}

func printDryRun(b []byte) {
	out, err := format.Source(b)
	if err != nil {
		panic(fmt.Sprintf("failed to format output '%v': %s", err, b))
	}

	if _, err = os.Stdout.Write(out); err != nil {
		panic(fmt.Sprintf("failed to write to stdout: %v", err))
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

	structType, s, err := loadStruct(f.packagesToLoad, f.SourceStruct)
	if err != nil {
		return nil, nil, err
	}
	structPackage := structType.String()[:strings.LastIndexByte(structType.String(), '.')]

	var (
		outBuf         bytes.Buffer
		constBuf       bytes.Buffer
		closeConstants = func() {
			constBuf.WriteByte(')')
		}
	)

	baseName := calculateBaseName(f)
	firstChar := strings.ToLower(baseName[:1])

	if f.Style != "" {
		outBuf.WriteString(fmt.Sprintf("// %s is a strong type generated from %s. Its type is used for all of its related generated constants.\n", baseName, f.SourceStruct))
	}

	switch f.Style {
	case StyleAlias:
		outBuf.WriteString(fmt.Sprintf("type %s = string\n", baseName))
	case StyleTyped:
		outBuf.WriteString(fmt.Sprintf("type %s string\n", baseName))
		outBuf.WriteString("// String implements the [fmt.Stringer] interface\n")
		outBuf.WriteString(fmt.Sprintf("func (%s %s) String() string { return (string)(%s) }\n", firstChar, baseName, firstChar))
	case StyleGeneric:
		outBuf.WriteString(fmt.Sprintf("type %s[T any] string\n", baseName))
		outBuf.WriteString("// String implements the [fmt.Stringer] interface\n")
		outBuf.WriteString(fmt.Sprintf("func (%s %s[T]) String() string { return (string)(%s) }\n", firstChar, baseName, firstChar))
	}

	fields, err := parseStructFields(f, structPackage, baseName, s)
	if err != nil {
		return nil, nil, err
	}

	if len(fields) == 0 {
		closeConstants()
	}

	var fieldNames []string
	for i, field := range fields {
		if f.Style == StyleGeneric {
			imports = append(imports, field.requiredImports...)
		}

		if constBuf.Len() == 0 {
			constBuf.WriteByte('\n')
			constBuf.WriteString(fmt.Sprintf("// Constants generated from [%s] struct field\n", f.SourceStruct))
			constBuf.WriteString("const (")
		} else {
			constBuf.WriteByte('\n')
		}

		switch f.Style {
		case StyleAlias, StyleTyped:
			constBuf.WriteString(fmt.Sprintf("%s %s = %q", field.constName, field.baseName, field.constValue))
		case StyleGeneric:
			constBuf.WriteString(fmt.Sprintf("%s %s[%s] = %q", field.constName, field.baseName, field.fieldType, field.constValue))
		default:
			constBuf.WriteString(fmt.Sprintf("%s = %q", field.constName, field.constValue))
		}
		fieldNames = append(fieldNames, field.constValue)
		if i == len(fields)-1 {
			closeConstants()
		}
	}

	if f.Iter {
		outBuf.WriteString(fmt.Sprintf("// All was generated from the [%s] struct. It returns an array of all [%s]'s associated constant values.\n", f.SourceStruct, baseName))

		var sb strings.Builder
		for _, n := range fieldNames {
			sb.WriteByte('\n')
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

type parsedField struct {
	parseFieldResult
	baseName string
}

func fieldIsEmbeddedStruct(f *types.Var) (*types.Struct, bool) {
	if !f.Embedded() {
		return nil, false
	}

	t := f.Type()
	for {
		switch v := t.(type) {
		case *types.Pointer:
			t = t.Underlying()
		case *types.Named:
			t = t.Underlying()
		case *types.Struct:
			return v, true
		default:
			return nil, false
		}
	}
}

func parseStructFields(f FlagOptions, structPackage, baseName string, s *types.Struct) ([]parsedField, error) {
	var (
		topLevelFields = make(map[string]struct{})
		fields         []parsedField
		embeddedFields []parsedField
	)
	for i := 0; i < s.NumFields(); i++ {
		field := s.Field(i)
		if !f.IncludeUnexportedFields && !field.Exported() {
			continue
		}

		tag := s.Tag(i)
		parseFieldResult, err := parseField(structPackage, field, tag, baseName, f)
		if err != nil {
			return nil, fmt.Errorf("failed to parse field with name %s: %w", field.Name(), err)
		}

		if parseFieldResult.constValue == "-" { // Handle the case that the field is ignored
			continue
		}

		if structType, ok := fieldIsEmbeddedStruct(field); ok {
			embFields, err := parseStructFields(f, structPackage, baseName, structType)
			if err != nil {
				return nil, err
			}

			embeddedFields = append(embeddedFields, embFields...)
			continue
		}

		bName := []rune(baseName)
		if f.Export {
			bName[0] = unicode.ToUpper(bName[0])
		} else {
			bName[0] = unicode.ToLower(bName[0])
		}
		baseName = string(bName)
		fields = append(fields, parsedField{
			parseFieldResult: parseFieldResult,
			baseName:         baseName,
		})
		topLevelFields[parseFieldResult.constName] = struct{}{}
	}

	for _, field := range embeddedFields {
		_, ok := topLevelFields[field.constName]
		if ok {
			continue
		}
		fields = append(fields, field)
	}

	return fields, nil
}

type parseFieldResult struct {
	fieldType, constName, constValue string
	requiredImports                  []string
}

func parseField(structPackage string, field *types.Var, tag, baseName string, f FlagOptions) (parseFieldResult, error) {
	tags, err := structtag.Parse(tag)
	if err != nil {
		return parseFieldResult{}, fmt.Errorf("failed to parse struct tags for field %s: %w", field.Name(), err)
	}

	fieldType, imps := parseTypeName(structPackage, field.Type())
	if sfgenTag, ok := sfgenTagName(f.Tag, tags); ok {
		return parseFieldResult{
			fieldType:       fieldType,
			constName:       baseName + field.Name(),
			constValue:      sfgenTag,
			requiredImports: imps,
		}, nil
	}

	tagNameValue := field.Name()
	if f.Tag != "" {
		nameFromTag, err := tags.Get(f.Tag)
		if err == nil && len(nameFromTag.Value()) > 0 && f.TagNameRegex != "" {
			re, err := regexp.Compile(f.TagNameRegex)
			if err != nil {
				return parseFieldResult{}, fmt.Errorf("failed to compile regex expression %q: %w", f.TagNameRegex, err)
			}

			if matches := re.FindStringSubmatch(nameFromTag.Value()); len(matches) >= 2 {
				tagNameValue = matches[1]
			}
		}

		if err == nil && len(nameFromTag.Name) > 0 && f.TagNameRegex == "" {
			tagNameValue = nameFromTag.Name
		}
	}

	return parseFieldResult{
		fieldType:       fieldType,
		constName:       baseName + field.Name(),
		constValue:      tagNameValue,
		requiredImports: imps,
	}, nil
}

func sfgenTagName(targetTagName string, tags *structtag.Tags) (string, bool) {
	sfgenTag, err := tags.Get("sfgen")
	if err != nil {
		return "", false
	}

	tagValue := sfgenTag.Value()
	if tagValue == "" {
		return "", false
	}

	tagParts := strings.SplitN(strings.TrimSpace(tagValue), ",", 2)
	tagName := tagParts[0] // We are guaranteed at least a slice with len(1)
	if len(tagParts) == 1 {
		return tagName, tagName != ""
	}

	// From here on we know that tagParts length is 2
	tagSpecificValues := strings.Split(tagParts[1], " ")
	for _, tagSpecificVal := range tagSpecificValues {
		tagSpecificVal = strings.TrimSpace(tagSpecificVal)
		if tagSpecificVal == "" {
			continue
		}

		tagValParts := strings.SplitN(tagSpecificVal, ":", 2)
		if len(tagValParts) != 2 || tagValParts[0] != targetTagName {
			continue
		}

		if tagValParts[1] != "" {
			tagName = tagValParts[1]
			break
		}
	}

	return tagName, tagName != ""
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

func loadStruct(source packageToLoad, structName string) (*types.Named, *types.Struct, error) {
	pkg, scope, ok := scopeForPackage(source)
	if !ok {
		var a []string
		for k := range packageNameToScopes {
			a = append(a, k)
		}
		return nil, nil, fmt.Errorf("failed to find package scope: %s, %+v", source, a)
	}

	// *types.TypeName is returned here
	foundObj := scope.Lookup(structName)
	if foundObj == nil {
		foundObj = scope.Lookup(strings.SplitN(structName, ".", 2)[1])
	}
	if foundObj == nil {
		return nil, nil, fmt.Errorf("type %s not found in package %s#%s", structName, pkg.Dir, pkg.Name)
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

func parseNamedType(structPackage string, u types.Type) (string, []string) {
	name := u.String()
	dotIndex := strings.LastIndexByte(name, '.')
	pkgPath := name
	if dotIndex >= 0 {
		pkgPath = name[:dotIndex]
	}

	if pkgPath == structPackage {
		return name[dotIndex+1:], nil
	}

	slashIndex := strings.LastIndexByte(name, '/')
	newName := name
	if slashIndex >= 0 {
		newName = name[slashIndex+1:]
	}

	if dotIndex >= 0 {
		return newName, []string{name[:dotIndex]}
	}

	return newName, nil
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
