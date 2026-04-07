package parser

import (
	"go/types"
	"strings"
)

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
