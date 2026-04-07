package template

type Data struct {
	Options *GenOptions
	Struct  *ParsedStruct
}

type GenOptions struct {
	DryRun                  bool
	OutputFile              string
	OutputDir               string
	OutputPackage           string
	SourceStruct            string
	SourceStructDir         string
	PackageName             string
	IncludeTests            bool
	Style                   string
	Template                string
	Tag                     string
	TagNameRegex            string
	Prefix                  *string
	Export                  bool
	UseStructName           bool
	IncludeUnexportedFields bool
	Iter                    bool
}

type ParsedStructField struct {
	Tags      []string
	FieldName string
	Embedded  bool
	Exported  bool
	*ParsedType
}

type FuncParam struct {
	// Name is the name of the parameter. It can be empty for unnamed parameters.
	Name string
	Type *ParsedType
}

type FuncSignature struct {
	Parameters       []FuncParam
	ReturnParameters []FuncParam
}

func (f *ParsedType) TypeName() string {
	return f.typeName()
}

func (f *ParsedType) Imports() []string {
	return f.imports()
}

type ParsedTypeArgs struct {
	TypeName func() string
	Imports  func() []string
	ParsedType
}

func NewParsedType(args ParsedTypeArgs) *ParsedType {
	args.typeName = args.TypeName
	args.imports = args.Imports
	return &args.ParsedType
}

type ParsedType struct {
	// The representation of the type as it should appear in the generated code. For example, "[]*MyStruct" or "map[string]int".
	typeName func() string
	imports  func() []string

	// Only relevant for map types
	KeyElem *ParsedType

	// Only relevant for slice, array, chan, pointer, map and named types. For maps, this is the value type.
	Elem *ParsedType

	// Only relevant for struct types.
	Fields        []ParsedStructField
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

type ParsedStruct struct {
	Name     string
	BaseName string
	Fields   []ParsedConstField
}

type ParsedConstField struct {
	ParsedStructField

	// ConstName is the name of the constant to be generated for this field.
	// For example, if the source struct is "Config", the field is "Timeout", and the prefix is "Field", this will be "ConfigFieldTimeout" or "FieldTimeout" depending on whether the --include-struct-name flag is used.
	ConstName string

	// ConstValue is the value of the constant to be generated for this field.
	// By default, this is the field's name, but it can be overridden by struct tags.
	ConstValue string
}
