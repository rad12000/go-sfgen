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
	OutputFile              string
	OutputDir               string
	OutputPackage           string
	SourceStruct            string
	SourceStructDir         string
	Style                   string
	Tag                     string
	TagNameRegex            string
	Prefix                  *string
	Export                  bool
	UseStructName           bool
	IncludeUnexportedFields bool
	Iter                    bool
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
	flagSet.StringVar(&f.OutputFile, "out-file", "", "the file to write generated code to")
	flagSet.StringVar(&f.OutputDir, "out-dir", ".", "the directory to output code gen to")
	flagSet.StringVar(&f.OutputPackage, "out-pkg", os.Getenv("GOPACKAGE"), /* set by go generate */
		"The package the generated code should belong to. Defaults to the package containing the go:generate directive")
	flagSet.StringVar(&f.SourceStruct, "struct", "", "The struct to generate field consts for")
	flagSet.StringVar(&f.SourceStructDir, "src-dir", ".",
		"The directory containing the --struct. Defaults to the current directory")
	flagSet.StringVar(&f.Tag, "tag", "", "if provided, the name in the provided tag will be used")
	flagSet.StringVar(&f.TagNameRegex, "tag-regex", "",
		"if provided, the regex will be tested on the specified tag contents. The first capture group will be used as the name. If it is empty, or does not match, the field name will be used instead.")
	flagSet.Func("prefix", "if provided, this value will be prepended to the field's name", func(s string) error {
		if f.Prefix != nil {
			return errors.New("invalid --prefix usage, flag may only be specified once")
		}
		f.Prefix = &s
		return nil
	})
	flagSet.StringVar(&f.Style, "style", "",
		"determines whether the fields will have their own simple type, a generic type, or no typing. Valid options are: alias, typed, generic")
	flagSet.BoolVar(&f.Export, "export", false, "if true, the generated constants will be exported")
	flagSet.BoolVar(&f.UseStructName, "include-struct-name", false, "if true, the generated constants will be prefixed with the source struct name")
	flagSet.BoolVar(&f.IncludeUnexportedFields, "include-unexported-fields", false, "if true, the generated constants will include fields that are not exported on the struct")
	flagSet.BoolVar(&f.Iter, "iter", false, "if true, an All() method will be generated for the type. Note this only is compatible with Go 1.23+")
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
			err = fmt.Errorf("--%s is required\n%s", v.Name, err)
		}

		if v.NotEmpty && v.Value == "" {
			err = fmt.Errorf("--%s must not be empty\n%s", v.Name, err)
		}

		if v.OneOf != nil {
			_, ok := v.OneOf[v.Value]
			if !ok {
				err = fmt.Errorf("--%s must be one of %+v\n%s", v.Name, v.OneOf, err)
			}
		}
	}

	return err
}
