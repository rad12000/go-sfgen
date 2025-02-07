package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/google/shlex"
	"os"
	"strings"
)

const (
	StyleTyped   = "typed"
	StyleGeneric = "generic"
	StyleAlias   = "alias"
)

type FlagOptions struct {
	DryRun                  bool
	OutputFile              string
	OutputDir               string
	OutputPackage           string
	SourceStruct            string
	SourceStructDir         string
	PackageName             string
	IncludeTests            bool
	Style                   string
	Tag                     string
	TagNameRegex            string
	Prefix                  *string
	Export                  bool
	UseStructName           bool
	IncludeUnexportedFields bool
	Iter                    bool
	packagesToLoad          packageToLoad
}

func (f *FlagOptions) ParseString(args string) error {
	argSlice, err := shlex.Split(strings.TrimSpace(args))
	if err != nil {
		return fmt.Errorf("failed to parse flag string: %w", err)
	}

	return f.Parse(argSlice)
}

func (f *FlagOptions) Parse(args []string) error {
	flagSet := flag.NewFlagSet("sfgen", flag.ContinueOnError)
	f.RegisterFlags(flagSet)
	if err := flagSet.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	return f.Validate()
}

func (f *FlagOptions) RegisterFlags(flagSet *flag.FlagSet) {
	flagSet.StringVar(&f.OutputFile, "out-file", "", `The file to write generated output to. Defaults to [--struct]_[prefix]_generated.go`)
	flagSet.BoolVar(&f.DryRun, "dry-run", false, `If true, no output file will be written to, but instead results will be written to stdout`)
	flagSet.StringVar(&f.OutputDir, "out-dir", ".", `The directory in which to place the generated file. Defaults to the current directory`)
	flagSet.StringVar(&f.OutputPackage, "out-pkg", os.Getenv("GOPACKAGE"),
		`The package the generated code should belong to. Defaults to the package containing the go:generate directive`)
	flagSet.StringVar(&f.SourceStruct, "struct", "", "The struct to use as the source for code generation. REQUIRED")
	flagSet.StringVar(&f.PackageName, "package", "", "The name of the package in which the source struct resides.")
	flagSet.BoolVar(&f.IncludeTests, "tests", false, "If true, source code in tests will be included. This flag will often need to be used along with the --package flag.")
	flagSet.StringVar(&f.SourceStructDir, "src-dir", ".",
		"The directory containing the --struct. Defaults to the current directory")
	flagSet.StringVar(&f.Tag, "tag", "",
		`If provided, the provided tag will be parsed for each field on the --struct. 
If the tag is missing, the struct field's name is used. 
Otherwise, the first attribute in the tag is used as the name'`)
	flagSet.StringVar(&f.TagNameRegex, "tag-regex", "",
		`This flag requires the --tag flag be provided as well. 
The provided regex will be tested on the specified tag contents for each field.
The first capture group will be used as the value for the generated constant. 
If the regex does not match the tag contents, the struct field's' name will be used instead.`)

	flagSet.Func("prefix", "A value to prepend to the generated const names. Defaults to [tag]Field", func(s string) error {
		if f.Prefix != nil {
			return errors.New("invalid --prefix usage, flag may only be specified once")
		}
		f.Prefix = &s
		return nil
	})
	flagSet.StringVar(&f.Style, "style", "", `Specifies the style of constants desired. Valid options are: alias, typed, generic`)
	flagSet.BoolVar(&f.Export, "export", false, "If true, the generated constants will be exported")
	flagSet.BoolVar(&f.UseStructName, "include-struct-name", false, "If true, the generated constants will be prefixed with the source struct name")
	flagSet.BoolVar(&f.IncludeUnexportedFields, "include-unexported-fields", false, "If true, the generated constants will include fields that are not exported on the struct")
	flagSet.BoolVar(&f.Iter, "iter", false, "if true, an All() method will be generated for the type, which returns an array of all the values generated")
}

func (f *FlagOptions) Validate() error {
	if f.Tag == "" && len(f.TagNameRegex) > 0 {
		return fmt.Errorf("cannot use tag regex %q with an empty tag", f.TagNameRegex)
	}

	type flagNameToValue struct {
		Name     string
		Value    string
		Required bool
		NotEmpty bool
		OneOf    map[string]struct{}
	}

	validations := []flagNameToValue{
		{
			Name:  "style",
			Value: f.Style,
			OneOf: map[string]struct{}{"": {}, StyleAlias: {}, StyleTyped: {}, StyleGeneric: {}},
		},
		{
			Name:     "struct",
			Value:    f.SourceStruct,
			Required: true,
		},
		{
			Name:     "src-dir",
			Value:    f.SourceStructDir,
			NotEmpty: true,
		},
		{
			Name:     "out-pkg",
			Value:    f.OutputPackage,
			NotEmpty: true,
		},
	}

	var err error
	for _, v := range validations {
		if v.Required && v.Value == "" {
			err = errors.Join(err, fmt.Errorf("--%s is required", v.Name))
		}

		if v.NotEmpty && v.Value == "" {
			err = errors.Join(err, fmt.Errorf("--%s must not be empty", v.Name))
		}

		if v.OneOf != nil {
			_, ok := v.OneOf[v.Value]
			if !ok {
				err = errors.Join(err, fmt.Errorf("--%s must be one of %+v", v.Name, v.OneOf))
			}
		}
	}

	return err
}
