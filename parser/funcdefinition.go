package parser

import (
	"go/types"
	"strings"
	"sync"

	"github.com/rad12000/go-sfgen/template"
)

func parseTypeNameSignature(structPackage string, u *types.Signature) *template.ParsedType {
	result := template.FuncSignature{
		Parameters:       make([]template.FuncParam, 0, u.Params().Len()),
		ReturnParameters: make([]template.FuncParam, 0, u.Results().Len()),
	}

	for i := 0; i < u.Params().Len(); i++ {
		param := u.Params().At(i)
		parsedType := parseTypeName(structPackage, param.Type())
		result.Parameters = append(result.Parameters, template.FuncParam{
			Name: param.Name(),
			Type: parsedType,
		})
	}

	for i := 0; i < u.Results().Len(); i++ {
		param := u.Results().At(i)
		parsedType := parseTypeName(structPackage, param.Type())
		result.ReturnParameters = append(result.ReturnParameters, template.FuncParam{
			Name: param.Name(),
			Type: parsedType,
		})
	}

	typeNameAndImports := sync.OnceValues(func() (string, []string) {
		var (
			sb      strings.Builder
			imports []string
		)
		sb.WriteString("func (")

		for i, param := range result.Parameters {
			imports = append(imports, param.Type.Imports()...)
			if i > 0 && i < len(result.Parameters) {
				sb.WriteByte(',')
			}
			sb.WriteString(param.Name)
			sb.WriteByte(' ')
			sb.WriteString(param.Type.TypeName())
		}

		sb.WriteByte(')')

		if len(result.ReturnParameters) > 1 {
			sb.WriteByte('(')
		}

		for i, param := range result.ReturnParameters {
			imports = append(imports, param.Type.Imports()...)
			if i > 0 && i < len(result.ReturnParameters) {
				sb.WriteByte(',')
			}
			sb.WriteString(param.Type.TypeName())
		}

		if len(result.ReturnParameters) > 1 {
			sb.WriteByte(')')
		}

		return sb.String(), imports
	})

	return template.NewParsedType(
		template.ParsedTypeArgs{
			TypeName: func() string {
				typeName, _ := typeNameAndImports()
				return typeName
			},
			Imports: func() []string {
				_, imports := typeNameAndImports()
				return imports
			},
			ParsedType: template.ParsedType{
				IsFuncSignature: true,
				FuncSignature:   result,
			},
		},
	)
}
