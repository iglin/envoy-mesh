// Package schema converts parsed proto message definitions into Kubernetes
// CRD OpenAPI v3 (JSONSchemaProps) schemas.
package schema

import (
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/iglin/envoy-mesh/crd-gen/internal/parser"
)

// MessageToSchema converts a proto message to a JSONSchemaProps suitable for
// embedding in a CRD spec.openAPIV3Schema.properties.spec field.
func MessageToSchema(msg *parser.Message, reg *parser.Registry) *apiextensionsv1.JSONSchemaProps {
	return msgSchema(msg, reg, make(map[string]bool))
}

func msgSchema(msg *parser.Message, reg *parser.Registry, visited map[string]bool) *apiextensionsv1.JSONSchemaProps {
	s := &apiextensionsv1.JSONSchemaProps{
		Type:        "object",
		Description: msg.Comment,
		Properties:  make(map[string]apiextensionsv1.JSONSchemaProps),
	}

	for _, field := range msg.Fields {
		var fs *apiextensionsv1.JSONSchemaProps
		if field.Type == "map" {
			valSchema := resolveType(field.MapValue, false, msg.Package, reg, cloneVisited(visited))
			fs = &apiextensionsv1.JSONSchemaProps{
				Type: "object",
				AdditionalProperties: &apiextensionsv1.JSONSchemaPropsOrBool{
					Schema: valSchema,
				},
			}
		} else {
			fs = resolveType(field.Type, field.Repeated, msg.Package, reg, cloneVisited(visited))
		}
		if field.Comment != "" {
			fs.Description = field.Comment
		}
		s.Properties[field.Name] = *fs
	}

	if len(s.Properties) == 0 {
		s.XPreserveUnknownFields = boolPtr(true)
	}
	return s
}

// resolveType maps a proto field type (scalar or message/enum name) to a schema,
// wrapping in an array schema when repeated is true.
func resolveType(typeName string, repeated bool, pkg string, reg *parser.Registry, visited map[string]bool) *apiextensionsv1.JSONSchemaProps {
	s := scalarSchema(typeName)
	if s == nil {
		s = resolveMessageOrEnum(typeName, pkg, reg, visited)
	}
	if repeated {
		return &apiextensionsv1.JSONSchemaProps{
			Type:  "array",
			Items: &apiextensionsv1.JSONSchemaPropsOrArray{Schema: s},
		}
	}
	return s
}

// scalarSchema returns a schema for a proto scalar type, or nil if typeName is
// not a scalar.
func scalarSchema(t string) *apiextensionsv1.JSONSchemaProps {
	switch t {
	case "string":
		return &apiextensionsv1.JSONSchemaProps{Type: "string"}
	case "bytes":
		return &apiextensionsv1.JSONSchemaProps{Type: "string", Format: "byte"}
	case "bool":
		return &apiextensionsv1.JSONSchemaProps{Type: "boolean"}
	case "int32", "sint32", "sfixed32":
		return &apiextensionsv1.JSONSchemaProps{Type: "integer", Format: "int32"}
	case "int64", "sint64", "sfixed64":
		return &apiextensionsv1.JSONSchemaProps{Type: "integer", Format: "int64"}
	case "uint32", "fixed32":
		return &apiextensionsv1.JSONSchemaProps{Type: "integer", Format: "int32"}
	case "uint64", "fixed64":
		return &apiextensionsv1.JSONSchemaProps{Type: "integer", Format: "int64"}
	case "float":
		return &apiextensionsv1.JSONSchemaProps{Type: "number", Format: "float"}
	case "double":
		return &apiextensionsv1.JSONSchemaProps{Type: "number", Format: "double"}
	}
	return nil
}

// wellKnownSchemas maps google.protobuf well-known types to their JSON Schema
// equivalents.
var wellKnownSchemas = map[string]*apiextensionsv1.JSONSchemaProps{
	"google.protobuf.Any":         {Type: "object", XPreserveUnknownFields: boolPtr(true)},
	"google.protobuf.Struct":      {Type: "object", XPreserveUnknownFields: boolPtr(true)},
	"google.protobuf.Value":       {XPreserveUnknownFields: boolPtr(true)},
	"google.protobuf.Duration":    {Type: "string"},
	"google.protobuf.Timestamp":   {Type: "string", Format: "date-time"},
	"google.protobuf.Empty":       {Type: "object"},
	"google.protobuf.StringValue": {Type: "string"},
	"google.protobuf.BoolValue":   {Type: "boolean"},
	"google.protobuf.Int32Value":  {Type: "integer", Format: "int32"},
	"google.protobuf.UInt32Value": {Type: "integer", Format: "int32"},
	"google.protobuf.Int64Value":  {Type: "integer", Format: "int64"},
	"google.protobuf.UInt64Value": {Type: "integer", Format: "int64"},
	"google.protobuf.FloatValue":  {Type: "number", Format: "float"},
	"google.protobuf.DoubleValue": {Type: "number", Format: "double"},
	"google.protobuf.BytesValue":  {Type: "string", Format: "byte"},
	"google.protobuf.ListValue": {
		Type: "array",
		Items: &apiextensionsv1.JSONSchemaPropsOrArray{
			Schema: &apiextensionsv1.JSONSchemaProps{XPreserveUnknownFields: boolPtr(true)},
		},
	},
}

func resolveMessageOrEnum(typeName, pkg string, reg *parser.Registry, visited map[string]bool) *apiextensionsv1.JSONSchemaProps {
	// Strip leading dot from absolute proto references (e.g. ".envoy.config....")
	typeName = strings.TrimPrefix(typeName, ".")

	if s, ok := wellKnownSchemas[typeName]; ok {
		return s
	}

	fqn := resolveFQN(typeName, pkg, reg)

	// Cycle guard — return free-form object to break the recursion.
	if visited[fqn] {
		return &apiextensionsv1.JSONSchemaProps{Type: "object", XPreserveUnknownFields: boolPtr(true)}
	}

	if msg, ok := reg.Messages[fqn]; ok {
		visited[fqn] = true
		return msgSchema(msg, reg, visited)
	}

	if enum, ok := reg.Enums[fqn]; ok {
		s := &apiextensionsv1.JSONSchemaProps{Type: "string"}
		for _, v := range enum.Values {
			s.Enum = append(s.Enum, apiextensionsv1.JSON{Raw: []byte(`"` + v + `"`)})
		}
		return s
	}

	// Unknown external type — accept any object.
	return &apiextensionsv1.JSONSchemaProps{Type: "object", XPreserveUnknownFields: boolPtr(true)}
}

// resolveFQN resolves a possibly-relative type name to a fully-qualified name
// by trying the current package and its parent prefixes.
func resolveFQN(typeName, pkg string, reg *parser.Registry) string {
	if inRegistry(typeName, reg) {
		return typeName
	}
	fqn := pkg + "." + typeName
	if inRegistry(fqn, reg) {
		return fqn
	}
	// Walk up the package hierarchy.
	parts := strings.Split(pkg, ".")
	for i := len(parts) - 1; i > 0; i-- {
		candidate := strings.Join(parts[:i], ".") + "." + typeName
		if inRegistry(candidate, reg) {
			return candidate
		}
	}
	return typeName
}

func inRegistry(name string, reg *parser.Registry) bool {
	_, inMsg := reg.Messages[name]
	_, inEnum := reg.Enums[name]
	return inMsg || inEnum
}

func cloneVisited(v map[string]bool) map[string]bool {
	c := make(map[string]bool, len(v))
	for k := range v {
		c[k] = true
	}
	return c
}

func boolPtr(b bool) *bool { return &b }
