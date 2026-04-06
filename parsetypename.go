package main

import (
	"fmt"
	"go/types"
	"log"
)

type FieldType struct {
	// The representation of the type as it should appear in the generated code. For example, "[]*MyStruct" or "map[string]int".
	TypeName string
	Imports  []string

	// Only relevant for map types
	KeyElem *FieldType

	// Only relevant for slice, array, chan, pointer, map and named types.
	Elem            *FieldType
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

func parseTypeName(structPackage string, t types.Type) FieldType {
	switch u := t.(type) {
	case *types.Basic:
		return FieldType{
			TypeName: u.Name(),
			IsBasic:  true,
		}
	case *types.Slice:
		parsedFieldType := parseTypeName(structPackage, u.Elem())
		return FieldType{
			TypeName: fmt.Sprintf("[]%s", parsedFieldType.TypeName),
			IsSlice:  true,
			Elem:     &parsedFieldType,
			Imports:  parsedFieldType.Imports,
		}
	case *types.Array:
		parsedFieldType := parseTypeName(structPackage, u.Elem())
		return FieldType{
			TypeName: fmt.Sprintf("[%d]%s", u.Len(), parsedFieldType.TypeName),
			IsArray:  true,
			Elem:     &parsedFieldType,
			Imports:  parsedFieldType.Imports,
		}
	case *types.Chan:
		parsedFieldType := parseTypeName(structPackage, u.Elem())
		result := FieldType{
			IsChannel: true,
			Elem:      &parsedFieldType,
			Imports:   parsedFieldType.Imports,
		}

		switch u.Dir() {
		case types.SendOnly:
			result.TypeName = fmt.Sprintf("chan <- %s", parsedFieldType.TypeName)
			result.ChanDirection = 0
		case types.RecvOnly:
			result.TypeName = fmt.Sprintf("<-chan %s", parsedFieldType.TypeName)
			result.ChanDirection = 1
		case types.SendRecv:
			result.TypeName = fmt.Sprintf("chan %s", parsedFieldType.TypeName)
			result.ChanDirection = 2
		}

		return result
	case *types.Pointer:
		parsedFieldType := parseTypeName(structPackage, u.Elem())
		return FieldType{
			TypeName:  fmt.Sprintf("*%s", parsedFieldType.TypeName),
			IsPointer: true,
			Imports:   parsedFieldType.Imports,
		}
	case *types.Map:
		keyFieldType := parseTypeName(structPackage, u.Key())
		valueFieldType := parseTypeName(structPackage, u.Elem())
		return FieldType{
			KeyElem: &keyFieldType,
			Elem:    &valueFieldType,
			IsMap:   true,
			Imports: append(keyFieldType.Imports, valueFieldType.Imports...),
		}
	case *types.Signature:
		typeName, imports := parseTypeNameSignature(structPackage, u)
		return FieldType{
			TypeName:        typeName,
			IsFuncSignature: true,
			Imports:         imports,
		}
	case *types.TypeParam:
		return parseTypeName(structPackage, u.Underlying())
	case *types.Alias:
		parsedFieldType := parseTypeName(structPackage, u.Underlying())
		namedType, imports := parseNamedType(structPackage, u)
		return FieldType{
			TypeName: namedType,
			IsNamed:  true,
			Elem:     &parsedFieldType,
			Imports:  imports,
		}
	case *types.Named:
		parsedFieldType := parseTypeName(structPackage, u.Underlying())
		namedType, imports := parseNamedType(structPackage, u)
		return FieldType{
			TypeName: namedType,
			IsNamed:  true,
			Elem:     &parsedFieldType,
			Imports:  imports,
		}
	default:
		log.Fatalf("unhandled type %T: %s", t, t)
	}

	return FieldType{}
}
