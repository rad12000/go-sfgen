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

func (f *FieldType) TypeName() string {
	return f.typeName()
}

func (f *FieldType) Imports() []string {
	return f.imports()
}

type NewFieldTypeArgs struct {
	TypeName func() string
	Imports  func() []string
	FieldType
}

func NewFieldType(args NewFieldTypeArgs) *FieldType {
	args.typeName = args.TypeName
	args.imports = args.Imports
	return &args.FieldType
}

type FieldType struct {
	// The representation of the type as it should appear in the generated code. For example, "[]*MyStruct" or "map[string]int".
	typeName func() string
	imports  func() []string

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

type ParsedStruct struct {
	Name     string
	BaseName string
	Fields   []ParsedField
}

type ParsedField struct {
	StructField

	// ConstName is the name of the constant to be generated for this field.
	// For example, if the source struct is "Config", the field is "Timeout", and the prefix is "Field", this will be "ConfigFieldTimeout" or "FieldTimeout" depending on whether the --include-struct-name flag is used.
	ConstName string

	// ConstValue is the value of the constant to be generated for this field.
	// By default, this is the field's name, but it can be overridden by struct tags.
	ConstValue string
}
