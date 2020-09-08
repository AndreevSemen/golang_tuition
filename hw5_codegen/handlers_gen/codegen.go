package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"text/template"
	"io"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type GenMethodParam struct {
	MethodName   string
	OwnerType    string
	URL          string
	NeedAuth     bool
	IsGetMethod  bool
	IsPostMethod bool
	ParamType    string
	ReturnType   string
}

type GenValidateParam struct {
	StructName string
	Fields []GenValidateFieldParam
}

type GenValidateFieldParam struct {
	FieldName  string
	IsString   bool
	IsRequired bool
	ParamName  string
	IsEnum     bool
	Enum       []string
	IsDefault  bool
	Default    string
	IsMin      bool
	Min        int
	IsMax      bool
	Max        int
}

var (
serveHTTPTmpl = template.Must(template.New("serveHTTP").Parse(
`// HTTP обертка для структуры {{.OwnerType}}
func (owner {{.OwnerType}}) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	{{range .Params}}case "{{.URL}}":
		owner.Wrapped{{.MethodName}}(w, r)
	{{end}}default:
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(
			"{\"error\":\"unknown method\"}",
		))
	}
}
`))

wrappedMethodTmpl = template.Must(template.New("wrappedMethod").Parse(
`func (owner {{.OwnerType}}) Wrapped{{.MethodName}}(w http.ResponseWriter, r *http.Request) { {{if .NeedAuth}}
	if r.Header.Get("X-Auth") != "100500" {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(
			"{\"error\": \"unauthorized\"}",
		))
		return
	}
	{{end}}{{if or .IsGetMethod .IsPostMethod}}
	if r.Method != {{if .IsGetMethod}}http.MethodGet{{else}}http.MethodPost{{end}} {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(
			"{\"error\":\"bad method\"}",
		))
		return
	}
	{{end}}

	var in {{.ParamType}}
	var err error
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		in, err = Validate{{.ParamType}}(r.Form)
	} else {
		in, err = Validate{{.ParamType}}(r.URL.Query())
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(
			"{\"error\":\""+err.Error()+"\"}",
		))
		return
	}

	if resp, err := owner.{{.MethodName}}(r.Context(), in); err != nil {
		if err, ok := err.(ApiError); ok {
			w.WriteHeader(err.HTTPStatus)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write([]byte(
			"{\"error\":\""+err.Error()+"\"}",
		))
	} else {
		w.WriteHeader(http.StatusOK)
		
		data, err := json.Marshal(map[string]interface{}{
			"error":    "",
			"response": resp,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}
}
`))

validateTypeTmpl = template.Must(template.New("ValidateType").Parse(
`func Validate{{.StructName}}(query url.Values) ({{.StructName}}, error) {
	var res {{.StructName}}

	// Fields declaration
{{range .Fields}}{{if .IsString}}	{{.ParamName}} := query.Get("{{.ParamName}}")
	{{else}}	{{.ParamName}}, err := strconv.Atoi(query.Get("{{.ParamName}}"))
	if err != nil {
		return res, errors.New("{{.ParamName}} must be int")
	}{{end}}
{{end}}
	// Process Required{{range .Fields}}{{if or .IsRequired .IsDefault}}
	if {{.ParamName}} == {{if .IsString}}""{{else}}0{{end}} {
		{{if .IsRequired}}return res, errors.New("{{.ParamName}} must me not {{if .IsString}}empty{{else}}0{{end}}"){{else}}	{{.ParamName}} = "{{.Default}}"{{end}}
	}{{end}}
{{end}}
	// Process Enums{{range .Fields}}{{if .IsEnum}}
	{{.ParamName}}enum := []string{
		{{range .Enum}}"{{.}}", {{end}}
	}
	switch {{.ParamName}} {
	{{range .Enum}}case "{{.}}":
	{{end}}default:
		return res, fmt.Errorf("{{.ParamName}} must be one of ["+strings.Join({{.ParamName}}enum, ", ")+"]")
	}{{end}}
	{{end}}
	// Process Max{{range .Fields}}{{if .IsMax}}
	{{if .IsString}}if len({{.ParamName}}) > {{.Max}} {
		return res, fmt.Errorf("{{.ParamName}} len must be <= %d", {{.Max}})
	}{{else}}if {{.ParamName}} > {{.Max}} {
		return res, fmt.Errorf("{{.ParamName}} must be <= %d", {{.Max}})
	}{{end}}
{{end}}{{end}}
	// Process Min{{range .Fields}}{{if .IsMin}}
	{{if .IsString}}if len({{.ParamName}}) < {{.Min}} {
		return res, fmt.Errorf("{{.ParamName}} len must be >= %d", {{.Min}})
	}{{else}}if {{.ParamName}} < {{.Min}} {
		return res, fmt.Errorf("{{.ParamName}} must be >= %d", {{.Min}})
	}{{end}}{{end}}
{{end}}
	// Release results{{range .Fields}}
	res.{{.FieldName}} = {{.ParamName}}
{{end}}

	return res, nil
}
`,
))
)

type GenMethodComment struct {
	URl    string `json:"url"`
	Auth   bool   `json:"auth"`
	Method string `json:"method"`
}

type CodeGenerator struct {
	File io.Writer
}

func NewCodeGenerator(out io.Writer) CodeGenerator {
	return CodeGenerator{out}
}

func (cg CodeGenerator) Write(data []byte) (int, error) {
	return cg.File.Write(data)
}

func (cg *CodeGenerator) Writef(format string, v ...interface{}) {
	_, _ = fmt.Fprintf(cg.File, format+"\n", v...)
}

func (cg *CodeGenerator) Space() {
	_, _ = fmt.Fprintln(cg.File)
}

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, _ := os.Create(os.Args[2])

	cg := NewCodeGenerator(out)
	skipper := func(format string, v ...interface{}) {
		fmt.Fprintf(cg.File, "// SKIP: "+format+"\n", v...)
	}

	cg.Writef("package %s", node.Name.Name)
	cg.Space()
	cg.Writef(`import "fmt"`)
	cg.Writef(`import "strings"`)
	cg.Writef(`import "encoding/json"`)
	cg.Writef(`import "errors"`)
	cg.Writef(`import "net/http"`)
	cg.Writef(`import "net/url"`)
	cg.Writef(`import "strconv"`)
	cg.Space()


	var methodOwners = make(map[string][]*GenMethodParam)
	var validationStructs = make([]*GenValidateParam, 0)
	for _, decl := range node.Decls {
		switch obj := decl.(type) {
		case *ast.FuncDecl:
			if param, err := ParseCodegenMethodParams(obj); err != nil {
				skipper("parsing method error occurred: %s", err)
				continue
			} else {
				methodOwners[param.OwnerType] = append(methodOwners[param.OwnerType], param)
			}
		case *ast.GenDecl:
			for _, spec := range obj.Specs {
				currType, ok := spec.(*ast.TypeSpec)
				if !ok {
					skipper("%T is not ast.TypeSpec\n", spec)
					continue
				}

				if param, err := ParseCodegenValidateParams(currType); err != nil {
					skipper("parsing validation error occurred: %s", err)
				} else {
					validationStructs = append(validationStructs, param)
				}
			}
		default:
			fmt.Printf("SKIP %T is not *ast.GenDecl or *ast.FuncDecl\n", obj)
		}
	}

	for _, params := range methodOwners {
		code := GenerateHTTPWrappers(params)
		cg.Write([]byte(code))
	}

	for _, param := range validationStructs {
		code := GenerateValidator(param)
		cg.Write([]byte(code))
	}
}

func AdjustTypeString(raw string) string {
	if strings.HasPrefix(raw, "&") {
		lowerBound := strings.Index(raw, " ") + 1
		higherBound := len(raw) - 1
		raw = "*" + raw[lowerBound:higherBound]
	}
	return raw
}

func ParseCodegenMethodParams(decl *ast.FuncDecl) (*GenMethodParam, error) {
	// Parsing MethodName
	var param GenMethodParam
	param.MethodName = decl.Name.Name

	// Parsing OwnerType
	if decl.Recv == nil {
		return nil, fmt.Errorf("function %s is not a method", param.MethodName)
	}

	param.OwnerType = fmt.Sprintf("%v", decl.Recv.List[0].Type)
	param.OwnerType = AdjustTypeString(param.OwnerType)

	// Parsing URL, NeedAuth, IsGetMethod, IsPostMethod
	if decl.Doc == nil {
		return nil, fmt.Errorf("method %s has no docs", param.MethodName)
	}

	var needCodegen bool
	var foundJson string
	for _, doc := range decl.Doc.List {
		var prefix = "// apigen:api "
		if strings.HasPrefix(doc.Text, prefix) {
			needCodegen = true
			foundJson = doc.Text[len(prefix):]
		}
	}
	if !needCodegen {
		return nil, fmt.Errorf("method %s don't need codegen", param.MethodName)
	}

	var comment GenMethodComment
	err := json.Unmarshal([]byte(foundJson), &comment)
	if err != nil {
		return nil, fmt.Errorf("method %s codegen params are invalid: %s", param.MethodName, foundJson)
	}

	param.URL = comment.URl
	param.NeedAuth = comment.Auth
	switch comment.Method {
	case "":
		break
	case "GET":
		param.IsGetMethod = true
	case "POST":
		param.IsPostMethod = true
	default:
		return nil, fmt.Errorf("function "+param.MethodName+" has invalid method: "+comment.Method)
	}

	// Parsing ParamType, ReturnType
	if len(decl.Type.Params.List) != 2 {
		return nil, fmt.Errorf("in method %s not enoght input arguments", param.MethodName)
	}

	param.ParamType = fmt.Sprintf("%v", decl.Type.Params.List[1].Type)
	param.ParamType = AdjustTypeString(param.ParamType)

	param.ReturnType = fmt.Sprintf("%v", decl.Type.Results.List[0].Type)
	param.ReturnType = AdjustTypeString(param.ReturnType)

	return &param, nil
}

func ParseCodegenValidateParams(decl *ast.TypeSpec) (*GenValidateParam, error) {
	// Parsing StructName
	var param = GenValidateParam{
		StructName: decl.Name.Name,
		Fields:     make([]GenValidateFieldParam, 0),
	}

	currStruct, ok := decl.Type.(*ast.StructType)
	if !ok {
		return nil, fmt.Errorf("%T is not ast.StructType\n", currStruct)
	}

	for _, field := range currStruct.Fields.List {
		var fParam GenValidateFieldParam
		fParam = GenValidateFieldParam{
			FieldName:  field.Names[0].Name,
			IsRequired: false,
			ParamName:  strings.ToLower(field.Names[0].Name),
			IsEnum:     false,
			Enum:       nil,
			Default:    "",
			Min:        0,
			Max:        0,
		}

		switch fmt.Sprintf("%v", field.Type) {
		case "string":
			fParam.IsString = true
		case "int"   :
			fParam.IsString = false
		default:
			return nil, fmt.Errorf("invalid type: supports string or int type")
		}

		if field.Tag == nil {
			continue
		}

		tag := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])

		r, ok := tag.Lookup("apivalidator")
		if !ok {
			continue
		}

		type KV struct {
			Key   string
			Value string
		}

		var args = strings.Split(r, ",")
		for _, arg := range args {
			var kv KV
			if idx := strings.Index(arg, "="); idx == -1 {
				kv.Key = arg
				strings.TrimLeft(kv.Key, " ")
				strings.TrimRight(kv.Key, " ")
			} else {
				pair := strings.Split(arg, "=")
				if len(pair) != 2 {
					panic("parse error: more than 1 '=' in kv expression")
				}
				kv.Key   = pair[0]
				kv.Value = pair[1]
				strings.TrimLeft (kv.Key, " ")
				strings.TrimRight(kv.Key, " ")
				strings.TrimLeft (kv.Value, " ")
				strings.TrimRight(kv.Value, " ")
			}

			switch kv.Key {
			case "required" :
				fParam.IsRequired = true
			case "paramname":
				fParam.ParamName = kv.Value
			case "enum"     :
				fParam.IsEnum = true
				fParam.Enum = strings.Split(kv.Value, "|")
				for i := range fParam.Enum {
					strings.TrimLeft (fParam.Enum[i], " ")
					strings.TrimRight(fParam.Enum[i], " ")
				}
			case "default"  :
				fParam.IsDefault = true
				fParam.Default = kv.Value
			case "min"      :
				if res, err := strconv.Atoi(kv.Value); err != nil {
					return nil, err
				} else {
					fParam.IsMin = true
					fParam.Min = res
				}
			case "max"      :
				if res, err := strconv.Atoi(kv.Value); err != nil {
					return nil, err
				} else {
					fParam.IsMax = true
					fParam.Max = res
				}
			default:
				panic("unknown tag key: "+kv.Key)
			}
		}
		param.Fields = append(param.Fields, fParam)
	}

	return &param, nil
}

func GenerateHTTPWrappers(params []*GenMethodParam) string {
	var buf bytes.Buffer

	var s = struct{
		OwnerType  string
		MethodName string
		Params     []*GenMethodParam
	}{
		OwnerType:  params[0].OwnerType,
		Params:     params,
	}
	if err := serveHTTPTmpl.Execute(&buf, s); err != nil {
		panic(err)
	}

	for _, param := range params {
		if err := wrappedMethodTmpl.Execute(&buf, param); err != nil {
			panic(err)
		}
	}

	return buf.String()
}

func GenerateValidator(param *GenValidateParam) string {
	var buf bytes.Buffer

	if err := validateTypeTmpl.Execute(&buf, param); err != nil {
		panic(err)
	}

	return buf.String()
}
