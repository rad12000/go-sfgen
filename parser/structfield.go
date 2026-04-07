package parser

import (
	"fmt"
	"go/types"
	"regexp"
	"strings"
	"unicode"

	"github.com/fatih/structtag"
	"github.com/rad12000/go-sfgen/template"
)

func parseTopLevelField(structPackage string, field *types.Var, tag, baseName string, f template.GenOptions) (template.ParsedField, error) {
	tags, err := structtag.Parse(tag)
	if err != nil {
		return template.ParsedField{}, fmt.Errorf("failed to parse struct tags for field %s: %w", field.Name(), err)
	}

	structField := parseStructField(structPackage, field)
	if sfgenTag, ok := sfgenTagName(f.Tag, tags); ok {
		return template.ParsedField{
			ConstName:   baseName + field.Name(),
			ConstValue:  sfgenTag,
			StructField: structField,
		}, nil
	}

	tagNameValue := field.Name()
	if f.Tag != "" {
		nameFromTag, err := tags.Get(f.Tag)
		if err == nil && len(nameFromTag.Value()) > 0 && f.TagNameRegex != "" {
			re, err := regexp.Compile(f.TagNameRegex)
			if err != nil {
				return template.ParsedField{}, fmt.Errorf("failed to compile regex expression %q: %w", f.TagNameRegex, err)
			}

			if matches := re.FindStringSubmatch(nameFromTag.Value()); len(matches) >= 2 {
				tagNameValue = matches[1]
			}
		}

		if err == nil && len(nameFromTag.Name) > 0 && f.TagNameRegex == "" {
			tagNameValue = nameFromTag.Name
		}
	}

	return template.ParsedField{
		ConstName:   baseName + field.Name(),
		ConstValue:  tagNameValue,
		StructField: structField,
	}, nil
}

func parseStructField(structPackage string, field *types.Var) template.StructField {
	fieldType := parseTypeName(structPackage, field.Type())
	return template.StructField{
		Embedded:  field.Embedded(),
		Exported:  field.Exported(),
		FieldName: field.Name(),
		FieldType: fieldType,
	}
}

func ParseStructFields(f template.GenOptions, structPackage, baseName string, s *types.Struct) (template.ParsedStruct, error) {
	bName := []rune(baseName)
	if f.Export {
		bName[0] = unicode.ToUpper(bName[0])
	} else {
		bName[0] = unicode.ToLower(bName[0])
	}
	baseName = string(bName)

	var (
		topLevelFields = make(map[string]struct{})
		fields         []template.ParsedField
		embeddedFields []template.ParsedField
	)
	for i := 0; i < s.NumFields(); i++ {
		field := s.Field(i)
		if !f.IncludeUnexportedFields && !field.Exported() {
			continue
		}

		tag := s.Tag(i)
		parseFieldResult, err := parseTopLevelField(structPackage, field, tag, baseName, f)
		if err != nil {
			return template.ParsedStruct{}, fmt.Errorf("failed to parse field with name %s: %w", field.Name(), err)
		}

		if parseFieldResult.ConstValue == "-" { // Handle the case that the field is ignored
			continue
		}

		if structType, ok := fieldIsEmbeddedStruct(field); ok {
			embFields, err := ParseStructFields(f, structPackage, baseName, structType)
			if err != nil {
				return template.ParsedStruct{}, err
			}

			embeddedFields = append(embeddedFields, embFields.Fields...)
			continue
		}

		fields = append(fields, parseFieldResult)
		topLevelFields[parseFieldResult.ConstName] = struct{}{}
	}

	for _, field := range embeddedFields {
		_, ok := topLevelFields[field.ConstName]
		if ok {
			continue
		}
		fields = append(fields, field)
	}

	return template.ParsedStruct{
		BaseName: baseName,
		Fields:   fields,
	}, nil
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
