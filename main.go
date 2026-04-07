/*
“
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
	-template string
	  	The path to a Go template file to use for generating the code from struct fields. The template is provided an instance of [template.Data] as its argument.
	-tests
	  	If true, source code in tests will be included. This flag will often need to be used along with the --package flag.
*/
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	_ "embed"

	"github.com/rad12000/go-sfgen/parser"
	"github.com/rad12000/go-sfgen/template"
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

	// var wg sync.WaitGroup
	for _, group := range outputFileGroups {
		// wg.Add(1)
		// go func(group []FlagOptions) {
		// 	defer wg.Done()
		generateCodeForFileGroup(group)
		// }(group)
	}

	// wg.Wait()
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
	fmt.Fprintf(buf, "// Source %s.%s:%s\n\n",
		os.Getenv("GOPACKAGE"), os.Getenv("GOFILE"), os.Getenv("GOLINE"))
	fmt.Fprintf(buf, "package %s\n", outPkg)
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
		err = os.MkdirAll(outDir, 0o755)
	}

	if err != nil {
		log.Fatalf("%v", err)
	}

	file, err := os.OpenFile(outFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("failed to open file at %s: %v", outFile, err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	_ = file.Truncate(0)

	unformatted := buf.Bytes()
	out, err := format.Source(unformatted)
	if err != nil {
		_, _ = file.Write(unformatted)
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

	baseName := calculateBaseName(f)
	structPackage := structType.String()[:strings.LastIndexByte(structType.String(), '.')]
	parsedStruct, err := parser.ParseStructFields(f.GenOptions, structPackage, baseName, s)
	if err != nil {
		return nil, nil, err
	}
	parsedStruct.Name = structType.Obj().Name()
	parsedStruct.BaseName = baseName

	var templateWrapper *template.Template
	if f.Template != "" {
		templateWrapper = template.New(template.FromFile(f.Template))
	} else {
		templateWrapper = template.New(template.Default())
	}

	var outBuf bytes.Buffer
	templateData := &template.Data{
		Options: &f.GenOptions,
		Struct:  &parsedStruct,
	}
	err = templateWrapper.Execute(&outBuf, templateData)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute template: %w", err)
	}

	return outBuf.Bytes(), templateWrapper.Imports(), nil
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
