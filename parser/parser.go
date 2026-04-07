package parser

import (
	"fmt"
	"go/types"
	"log"
	"strings"
	"sync"

	"github.com/rad12000/go-sfgen/template"
)

var fieldTypeMemo = make(map[string]*template.ParsedType)

func parseTypeName(structPackage string, t types.Type) *template.ParsedType {
	existing, ok := fieldTypeMemo[t.String()]
	if ok {
		return existing
	}

	result := new(template.ParsedType)
	fieldTypeMemo[t.String()] = result
	*result = *parseTypeNameInternal(structPackage, t)
	return result
}

func parseTypeNameInternal(structPackage string, t types.Type) *template.ParsedType {
	switch u := t.(type) {
	case *types.Basic:
		return template.NewParsedType(
			template.ParsedTypeArgs{
				TypeName:   u.Name,
				Imports:    func() []string { return nil },
				ParsedType: template.ParsedType{IsBasic: true},
			},
		)
	case *types.Slice:
		parsedFieldType := parseTypeName(structPackage, u.Elem())
		return template.NewParsedType(
			template.ParsedTypeArgs{
				TypeName: sync.OnceValue(func() string {
					return fmt.Sprintf("[]%s", parsedFieldType.TypeName())
				}),
				Imports: func() []string { return parsedFieldType.Imports() },
				ParsedType: template.ParsedType{
					IsSlice: true,
					Elem:    parsedFieldType,
				},
			},
		)
	case *types.Array:
		parsedFieldType := parseTypeName(structPackage, u.Elem())
		return template.NewParsedType(
			template.ParsedTypeArgs{
				TypeName: sync.OnceValue(func() string {
					return fmt.Sprintf("[%d]%s", u.Len(), parsedFieldType.TypeName())
				}),
				Imports: func() []string { return parsedFieldType.Imports() },
				ParsedType: template.ParsedType{
					IsArray: true,
					Elem:    parsedFieldType,
				},
			},
		)
	case *types.Chan:
		parsedFieldType := parseTypeName(structPackage, u.Elem())

		var (
			typeNameFn    func() string
			chanDirection int
		)

		switch u.Dir() {
		case types.SendOnly:
			typeNameFn = sync.OnceValue(func() string {
				return fmt.Sprintf("chan<- %s", parsedFieldType.TypeName())
			})
			chanDirection = 0
		case types.RecvOnly:
			typeNameFn = sync.OnceValue(func() string {
				return fmt.Sprintf("<-chan %s", parsedFieldType.TypeName())
			})
			chanDirection = 1
		case types.SendRecv:
			typeNameFn = sync.OnceValue(func() string {
				return fmt.Sprintf("chan %s", parsedFieldType.TypeName())
			})
			chanDirection = 2
		}

		return template.NewParsedType(
			template.ParsedTypeArgs{
				TypeName: typeNameFn,
				Imports:  func() []string { return parsedFieldType.Imports() },
				ParsedType: template.ParsedType{
					IsChannel:     true,
					Elem:          parsedFieldType,
					ChanDirection: chanDirection,
				},
			},
		)
	case *types.Pointer:
		parsedFieldType := parseTypeName(structPackage, u.Elem())
		return template.NewParsedType(
			template.ParsedTypeArgs{
				TypeName: sync.OnceValue(func() string {
					return fmt.Sprintf("*%s", parsedFieldType.TypeName())
				}),
				Imports: func() []string { return parsedFieldType.Imports() },
				ParsedType: template.ParsedType{
					IsPointer: true,
				},
			},
		)
	case *types.Struct:
		return parseStructType(structPackage, u)
	case *types.Map:
		keyFieldType := parseTypeName(structPackage, u.Key())
		valueFieldType := parseTypeName(structPackage, u.Elem())
		return template.NewParsedType(
			template.ParsedTypeArgs{
				TypeName: sync.OnceValue(func() string {
					return fmt.Sprintf("map[%s]%s", keyFieldType.TypeName(), valueFieldType.TypeName())
				}),
				Imports: sync.OnceValue(func() []string {
					return append(keyFieldType.Imports(), valueFieldType.Imports()...)
				}),
				ParsedType: template.ParsedType{
					KeyElem: keyFieldType,
					Elem:    valueFieldType,
					IsMap:   true,
				},
			},
		)
	case *types.Signature:
		return parseTypeNameSignature(structPackage, u)
	case *types.TypeParam:
		return parseTypeName(structPackage, u.Underlying())
	case *types.Alias:
		parsedFieldType := parseTypeName(structPackage, u.Underlying())
		namedType, imports := parseNamedType(structPackage, u)
		return template.NewParsedType(
			template.ParsedTypeArgs{
				TypeName: func() string { return namedType },
				Imports:  func() []string { return imports },
				ParsedType: template.ParsedType{
					Exported: u.Obj().Exported(),
					IsNamed:  true,
					Elem:     parsedFieldType,
				},
			},
		)
	case *types.Named:
		parsedFieldType := parseTypeName(structPackage, u.Underlying())
		namedType, imports := parseNamedType(structPackage, u)
		return template.NewParsedType(
			template.ParsedTypeArgs{
				TypeName: func() string { return namedType },
				Imports:  func() []string { return imports },
				ParsedType: template.ParsedType{
					Exported: u.Obj().Exported(),
					IsNamed:  true,
					Elem:     parsedFieldType,
				},
			},
		)
	default:
		log.Fatalf("unhandled type %T: %s", t, t)
	}

	return nil
}

func parseStructType(structPackage string, u *types.Struct) *template.ParsedType {
	fields := make([]template.ParsedStructField, 0, u.NumFields())

	for i := range u.NumFields() {
		field := u.Field(i)
		structField := parseStructField(structPackage, field)
		fields = append(fields, structField)
	}

	loadImportsAndTypeNames := sync.OnceValues(func() (string, []string) {
		importSet := make(map[string]struct{})
		fieldDefinitions := make([]string, 0, u.NumFields())
		for _, structField := range fields {
			for _, imp := range structField.ParsedType.Imports() {
				importSet[imp] = struct{}{}
			}
			fieldDefinitions = append(fieldDefinitions, fmt.Sprintf("%s %s", structField.FieldName, structField.TypeName()))
		}
		imps := make([]string, 0, len(importSet))
		for imp := range importSet {
			imps = append(imps, imp)
		}

		return fmt.Sprintf("struct {\n%s\n}", strings.Join(fieldDefinitions, "\n")), imps
	})

	return template.NewParsedType(
		template.ParsedTypeArgs{
			TypeName: sync.OnceValue(func() string {
				typeName, _ := loadImportsAndTypeNames()
				return typeName
			}),
			Imports: sync.OnceValue(func() []string {
				_, imports := loadImportsAndTypeNames()
				return imports
			}),
			ParsedType: template.ParsedType{
				Fields:   fields,
				IsStruct: true,
			},
		},
	)
}
