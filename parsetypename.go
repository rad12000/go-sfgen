package main

import (
	"fmt"
	"go/types"
	"log"
	"strings"
	"sync"
)

type StructField struct {
	FieldName string
	Embedded  bool
	Exported  bool
	*FieldType
}

type FuncParam struct {
	// Name is the name of the parameter. It can be empty for unnamed parameters.
	Name string
	Type *FieldType
}

type FuncSignature struct {
	Parameters       []FuncParam
	ReturnParameters []FuncParam
}

type FieldType struct {
	// The representation of the type as it should appear in the generated code. For example, "[]*MyStruct" or "map[string]int".
	TypeName func() string
	Imports  func() []string

	// Only relevant for map types
	KeyElem *FieldType

	// Only relevant for slice, array, chan, pointer, map and named types. For maps, this is the value type.
	Elem *FieldType

	// Only relevant for struct types.
	Fields        []StructField
	FuncSignature FuncSignature

	// Whether the named type is exported.
	// This is only relevant when [IsNamed] is true.
	Exported        bool
	IsNamed         bool
	IsPointer       bool
	IsBasic         bool
	IsArray         bool
	IsSlice         bool
	ChanDirection   int // 0 = send, 1 = recv, 2 = sendrecv
	IsChannel       bool
	IsMap           bool
	IsStruct        bool
	IsFuncSignature bool
}

var fieldTypeMemo = make(map[string]*FieldType)

func parseTypeName(structPackage string, t types.Type) *FieldType {
	existing, ok := fieldTypeMemo[t.String()]
	if ok {
		return existing
	}

	result := new(FieldType)
	fieldTypeMemo[t.String()] = result
	*result = *parseTypeNameInternal(structPackage, t)
	return result
}

func parseTypeNameInternal(structPackage string, t types.Type) *FieldType {
	switch u := t.(type) {
	case *types.Basic:
		return &FieldType{
			TypeName: u.Name,
			IsBasic:  true,
			Imports:  func() []string { return nil },
		}
	case *types.Slice:
		parsedFieldType := parseTypeName(structPackage, u.Elem())
		return &FieldType{
			TypeName: sync.OnceValue(func() string {
				return fmt.Sprintf("[]%s", parsedFieldType.TypeName())
			}),
			IsSlice: true,
			Elem:    parsedFieldType,
			Imports: func() []string { return parsedFieldType.Imports() },
		}
	case *types.Array:
		parsedFieldType := parseTypeName(structPackage, u.Elem())
		return &FieldType{
			TypeName: sync.OnceValue(func() string {
				return fmt.Sprintf("[%d]%s", u.Len(), parsedFieldType.TypeName())
			}),
			IsArray: true,
			Elem:    parsedFieldType,
			Imports: func() []string { return parsedFieldType.Imports() },
		}
	case *types.Chan:
		parsedFieldType := parseTypeName(structPackage, u.Elem())
		result := FieldType{
			IsChannel: true,
			Elem:      parsedFieldType,
			Imports:   func() []string { return parsedFieldType.Imports() },
		}

		switch u.Dir() {
		case types.SendOnly:
			result.TypeName = sync.OnceValue(func() string {
				return fmt.Sprintf("chan<- %s", parsedFieldType.TypeName())
			})
			result.ChanDirection = 0
		case types.RecvOnly:
			result.TypeName = sync.OnceValue(func() string {
				return fmt.Sprintf("<-chan %s", parsedFieldType.TypeName())
			})
			result.ChanDirection = 1
		case types.SendRecv:
			result.TypeName = sync.OnceValue(func() string {
				return fmt.Sprintf("chan %s", parsedFieldType.TypeName())
			})
			result.ChanDirection = 2
		}

		return &result
	case *types.Pointer:
		parsedFieldType := parseTypeName(structPackage, u.Elem())
		return &FieldType{
			TypeName: sync.OnceValue(func() string {
				return fmt.Sprintf("*%s", parsedFieldType.TypeName())
			}),
			IsPointer: true,
			Imports:   func() []string { return parsedFieldType.Imports() },
		}
	case *types.Struct:
		return parseStructType(structPackage, u)
	case *types.Map:
		keyFieldType := parseTypeName(structPackage, u.Key())
		valueFieldType := parseTypeName(structPackage, u.Elem())
		return &FieldType{
			TypeName: sync.OnceValue(func() string {
				return fmt.Sprintf("map[%s]%s", keyFieldType.TypeName(), valueFieldType.TypeName())
			}),
			KeyElem: keyFieldType,
			Elem:    valueFieldType,
			IsMap:   true,
			Imports: sync.OnceValue(func() []string {
				return append(keyFieldType.Imports(), valueFieldType.Imports()...)
			}),
		}
	case *types.Signature:
		return parseTypeNameSignature(structPackage, u)
	case *types.TypeParam:
		return parseTypeName(structPackage, u.Underlying())
	case *types.Alias:
		parsedFieldType := parseTypeName(structPackage, u.Underlying())
		namedType, imports := parseNamedType(structPackage, u)
		return &FieldType{
			Exported: u.Obj().Exported(),
			IsNamed:  true,
			Elem:     parsedFieldType,
			TypeName: func() string { return namedType },
			Imports:  func() []string { return imports },
		}
	case *types.Named:
		parsedFieldType := parseTypeName(structPackage, u.Underlying())
		namedType, imports := parseNamedType(structPackage, u)
		return &FieldType{
			Exported: u.Obj().Exported(),
			IsNamed:  true,
			Elem:     parsedFieldType,
			TypeName: func() string { return namedType },
			Imports:  func() []string { return imports },
		}
	default:
		log.Fatalf("unhandled type %T: %s", t, t)
	}

	return nil
}

func parseStructType(structPackage string, u *types.Struct) *FieldType {
	result := FieldType{
		Fields:   make([]StructField, 0, u.NumFields()),
		IsStruct: true,
	}

	for i := range u.NumFields() {
		field := u.Field(i)
		structField := parseStructField(structPackage, field)
		result.Fields = append(result.Fields, structField)
	}

	loadImportsAndTypeNames := sync.OnceValues(func() (string, []string) {
		importSet := make(map[string]struct{})
		fieldDefinitions := make([]string, 0, u.NumFields())
		for _, structField := range result.Fields {
			for _, imp := range structField.FieldType.Imports() {
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

	result.TypeName = sync.OnceValue(func() string {
		typeName, _ := loadImportsAndTypeNames()
		return typeName
	})

	result.Imports = sync.OnceValue(func() []string {
		_, imports := loadImportsAndTypeNames()
		return imports
	})

	return &result
}
