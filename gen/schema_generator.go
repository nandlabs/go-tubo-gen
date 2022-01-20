package gen

import (
	"context"
	"encoding/json"
	"fmt"
	"go.nandlabs.io/turbo-gen/spec"
	"io/ioutil"
	"math"
	"net/url"
	"strings"
)

const (
	Fields          = "fields"
	IsArray         = "is-array"
	XmlPrefixes     = "xml-prefixes"
	DocPath         = "doc-path"
	BasePath        = "base-path"
	RequiredFields  = "required-fields"
	JsonContentType = "application/json"
	XmlContentType  = "text/xml"
)

type Field struct {
	Type        string // Type
	Name        string
	VarName     string
	TargetNames map[string]string
	Required    bool
	Path        string
	IsArray     bool
}
type RefField struct {
	Field
	Reference string
}

type XML struct {
	Name      string
	Namespace string
	Prefix    string
	Attribute bool
	Wrapped   bool
}

type StringField struct {
	Field
	Default *string
	Pattern *string
	MinLen  *int
	MaxLen  *int
	Format  *string
}

type NumberField struct {
	Field
	Default      *float64
	Min          *float64
	Max          *float64
	MinExclusive *float64
	MaxExclusive *float64
	MultipleOf   *float64
}

type BooleanField struct {
	Field
	Default *bool
}

type ArrayField struct {
	Field
}

type ObjectField struct {
	Field
	Members              map[string]interface{}
	AdditionalProperties []interface{}
	MinProperties        int
	MaxProperties        int
}

type SchemaGen struct {
	SchemaInfos map[string]*SchemaInfo
	References  map[string]map[string]*SchemaInfo // [docPath]([itemPath]*SchemaInfo)
}

func NewSchemaGen() SchemaGen {
	return SchemaGen{SchemaInfos: make(map[string]*SchemaInfo),
		References: make(map[string]map[string]*SchemaInfo),
	}
}

type SchemaInfo struct {
	Schema   *spec.Schema
	Name     string
	DocPath  *url.URL
	BasePath *url.URL
	Fields   map[string]interface{}
}

func (sg SchemaGen) Print() {

	b, err := json.Marshal(sg)
	if err == nil {
		fmt.Println(string(b))
	}
}

func (sg SchemaGen) Add(name, docPath, basePath string, schema *spec.Schema) {
	docUrl, _ := url.Parse(docPath)
	baseUrl, _ := url.Parse(basePath)
	itemUrl, _ := url.Parse(basePath + "/" + name)
	si := &SchemaInfo{
		Schema:   schema,
		DocPath:  docUrl,
		BasePath: baseUrl,
		Name:     name,
		Fields:   make(map[string]interface{}),
	}

	sg.SchemaInfos[name] = si

	if v, ok := sg.References[docUrl.String()]; ok {
		v[itemUrl.String()] = si
	} else {
		ref := make(map[string]*SchemaInfo)
		ref[itemUrl.String()] = si
		sg.References[docUrl.String()] = ref
	}

}

func (sg SchemaGen) Generate() {

	for _, si := range sg.SchemaInfos {
		ctx := context.Background()
		xmlPrefixes := make(map[string]string)
		ctx = context.WithValue(ctx, XmlPrefixes, xmlPrefixes)
		ctx = context.WithValue(ctx, IsArray, false)
		ctx = context.WithValue(ctx, Fields, si.Fields)
		ctx = context.WithValue(ctx, DocPath, si.DocPath)
		ctx = context.WithValue(ctx, BasePath, si.BasePath)

		sg.handleSchema(si.Name, si.Schema, ctx)
	}

}

func (sg SchemaGen) handleSchema(name string, schema *spec.Schema, ctx context.Context) {
	if schema.Ref != nil {
		f := RefField{}
		f.Field = getFieldData(name, schema, ctx)
		f.Type = "ref"
		f.Reference = *schema.Ref

		//Handle Ref here
		u, err := url.Parse(*schema.Ref)
		if err != nil {
			panic("Invalid URI Reference for Field " + name)
		}
		if strings.HasPrefix(u.Scheme, "http") {
			//Get Schema from external source
			//	Maybe we should not support it as it may be a security issue in SAAS application.
			// A allow-list of urls  to load schemas would be more secure.

		} else if u.Scheme != "" {
			panic("Unsupported protocol for reference of Type for Field " + name + " Only http or https are valid")
		} else {
			//Either Local Document or relative  Document

			//External  Document
			if u.Path != "" {
				//Load External Document relative to current document
				//The document can be in Yaml or json Format.
				//TODO Add yaml parser later
				//TODO add Error Handling
				currentDocPath := ctx.Value(DocPath).(*url.URL)
				refUrl, err := currentDocPath.Parse(u.String())
				if err == nil {
					f, err := ioutil.ReadFile(refUrl.Path)
					if err != nil {
						oas := spec.OAS{}
						err := json.Unmarshal(f, &oas)
						if err == nil {
							for k, v := range oas.Components.Schemas {
								sg.Add(k, refUrl.Path, refUrl.Fragment, v)
							}
						}

					}
				}
			} else if strings.HasPrefix(u.Fragment, "#") {
				//Current Document should be handled by the schemagen as it is expected to have all schema
			}

		}
		currentScope := ctx.Value(Fields).(map[string]interface{})
		currentScope[name] = f

	} else {

		switch schema.Type {
		case "boolean":
			sg.handleBoolean(name, schema, ctx)
		case "integer":
			sg.handleNumeric(name, schema, ctx)
		case "number":
			sg.handleNumeric(name, schema, ctx)
		case "string":
			sg.handleString(name, schema, ctx)
		case "array":
			sg.handleArray(name, schema, ctx)
		case "object":
			sg.handleObject(name, schema, ctx)

		}

	}
}

func (sg SchemaGen) handleBoolean(name string, schema *spec.Schema, ctx context.Context) {

	currentScope := ctx.Value(Fields).(map[string]interface{})
	f := BooleanField{}
	f.Field = getFieldData(name, schema, ctx)
	f.Type = "bool"
	if schema.Default != nil {
		v := schema.Default.(bool)
		f.Default = &v
	}
	currentScope[name] = f
}

func (sg SchemaGen) handleString(name string, schema *spec.Schema, ctx context.Context) {
	currentScope := ctx.Value(Fields).(map[string]interface{})
	f := StringField{}
	f.Field = getFieldData(name, schema, ctx)
	f.Type = "string"
	if schema.Pattern != nil {
		f.Pattern = schema.Pattern
	}
	if schema.MinLength != nil {
		f.MinLen = schema.MinLength
	}

	if schema.MaxLength != nil {
		f.MaxLen = schema.MaxLength
	}

	if schema.Format != nil {
		f.Format = schema.Format
	}

	if schema.Default != nil {
		v := schema.Default.(string)
		f.Default = &v
	}
	currentScope[name] = f

}

func (sg SchemaGen) handleNumeric(name string, schema *spec.Schema, ctx context.Context) {
	currentScope := ctx.Value(Fields).(map[string]interface{})
	f := NumberField{}
	f.Field = getFieldData(name, schema, ctx)

	if schema.Minimum != nil {
		f.Min = schema.Minimum
	}

	if schema.Maximum != nil {
		f.Max = schema.Maximum
	}

	if schema.ExclusiveMinimum != nil {
		f.MinExclusive = schema.ExclusiveMinimum
	}

	if schema.ExclusiveMaximum != nil {
		f.MaxExclusive = schema.ExclusiveMaximum
	}

	if schema.MultipleOf != nil {
		f.MultipleOf = schema.MultipleOf
	}

	if schema.Default != nil {
		v := schema.Default.(float64)
		f.Default = &v
	}

	if schema.Type == "integer" {
		if schema.Format != nil {
			f.Type = *schema.Format
		} else {
			f.Type = "int64"
		}

	} else if schema.Type == "number" {
		if schema.Maximum != nil && (*schema.Maximum <= math.MaxFloat32) {
			f.Type = "float32"
		} else {
			f.Type = "float64"
		}
	}
	currentScope[name] = f
}

func (sg SchemaGen) handleObject(name string, schema *spec.Schema, ctx context.Context) {
	//TODO Handle the possible infinite loop
	members := make(map[string]interface{})
	objCtx := context.WithValue(ctx, Fields, members)
	requiredFields := make(map[string]bool)
	if schema.Required != nil {
		for _, f := range schema.Required {
			requiredFields[f] = true
		}
	}
	objCtx = context.WithValue(objCtx, RequiredFields, requiredFields)
	if schema.OneOf != nil {
		for _, v := range schema.OneOf {
			sg.handleSchema(name, v, objCtx)
		}
	}

	if schema.AllOf != nil {
		for _, v := range schema.AllOf {
			sg.handleSchema(name, v, objCtx)
		}
	}
	for k, v := range schema.Properties {
		sg.handleSchema(k, v, objCtx)
	}

	currentScope := ctx.Value(Fields).(map[string]interface{})
	f := ObjectField{}
	f.Field = getFieldData(name, schema, ctx)
	f.Type = "struct"
	f.Members = members
	currentScope[name] = f
}

func (sg SchemaGen) handleArray(name string, schema *spec.Schema, ctx context.Context) {

	arrayContext := context.WithValue(ctx, IsArray, true)
	sg.handleSchema(name, schema.Items, arrayContext)

}

func getFieldData(name string, schema *spec.Schema, ctx context.Context) Field {

	targetNames := make(map[string]string)

	targetNames[JsonContentType] = name
	required := false
	if ctx.Value(RequiredFields) != nil {

		requiredFields := ctx.Value(RequiredFields).(map[string]bool)
		_, required = requiredFields[name]

	}
	if schema.Xml != nil {
		if schema.Xml.Name != nil {
			xmlName := ""
			if schema.Xml.Prefix != nil {
				xmlName += *schema.Xml.Prefix + ":"
			}
			if schema.Xml.Namespace != nil {
				xmlPrefixes := ctx.Value(XmlPrefixes).(map[string]string)
				xmlPrefixes[*schema.Xml.Prefix] = *schema.Xml.Namespace
			}

			xmlName += *schema.Xml.Name
			targetNames[XmlContentType] = xmlName
		} else {
			targetNames[XmlContentType] = name
		}

	}

	return Field{
		Type:        "",
		Name:        getFieldName(name),
		VarName:     getVarName(name),
		TargetNames: targetNames,
		Required:    required,
		Path:        "",
		IsArray:     ctx.Value(IsArray).(bool),
	}
}

func getFieldName(name string) string {

	// Add naming formatted according to the final spec.
	return strings.Title(name)
}

func getVarName(name string) string {
	return strings.Title(name)
}
