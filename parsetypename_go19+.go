//go:build !go1.22

package main

import (
	"fmt"
	"go/types"
	"log"
)

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
	case *types.Named:
		return parseNamedType(structPackage, u)
	default:
		log.Fatalf("unhandled type %T: %s", t, t)
	}
	return "", nil
}
