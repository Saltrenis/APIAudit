// Package openapi generates OpenAPI 3.0 specifications from scanned routes.
package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Saltrenis/APIAudit/internal/scan"
)

// Info holds the metadata written into the OpenAPI info block.
type Info struct {
	// Title is the API title.
	Title string
	// Version is the API version string (e.g., "1.0.0").
	Version string
	// Description is a human-readable description of the API.
	Description string
}

// OpenAPISpec is the top-level OpenAPI 3.0 document.
type OpenAPISpec struct {
	OpenAPI    string              `json:"openapi" yaml:"openapi"`
	Info       OpenAPIInfo         `json:"info" yaml:"info"`
	Paths      map[string]PathItem `json:"paths" yaml:"paths"`
	Components Components          `json:"components,omitempty" yaml:"components,omitempty"`
}

// OpenAPIInfo maps to the OpenAPI info object.
type OpenAPIInfo struct {
	Title       string `json:"title" yaml:"title"`
	Version     string `json:"version" yaml:"version"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// PathItem holds all operations for a single URL path.
type PathItem struct {
	Get     *Operation `json:"get,omitempty" yaml:"get,omitempty"`
	Post    *Operation `json:"post,omitempty" yaml:"post,omitempty"`
	Put     *Operation `json:"put,omitempty" yaml:"put,omitempty"`
	Delete  *Operation `json:"delete,omitempty" yaml:"delete,omitempty"`
	Patch   *Operation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Head    *Operation `json:"head,omitempty" yaml:"head,omitempty"`
	Options *Operation `json:"options,omitempty" yaml:"options,omitempty"`
}

// Operation represents a single HTTP operation.
type Operation struct {
	OperationID string              `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Summary     string              `json:"summary,omitempty" yaml:"summary,omitempty"`
	Tags        []string            `json:"tags,omitempty" yaml:"tags,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses" yaml:"responses"`
	Parameters  []Parameter         `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// RequestBody describes the HTTP request body.
type RequestBody struct {
	Required bool                 `json:"required,omitempty" yaml:"required,omitempty"`
	Content  map[string]MediaType `json:"content" yaml:"content"`
}

// MediaType wraps a schema for a content type.
type MediaType struct {
	Schema SchemaObject `json:"schema" yaml:"schema"`
}

// Response describes a single HTTP response.
type Response struct {
	Description string               `json:"description" yaml:"description"`
	Content     map[string]MediaType `json:"content,omitempty" yaml:"content,omitempty"`
}

// Parameter describes a query, path, or header parameter.
type Parameter struct {
	Name     string       `json:"name" yaml:"name"`
	In       string       `json:"in" yaml:"in"`
	Required bool         `json:"required,omitempty" yaml:"required,omitempty"`
	Schema   SchemaObject `json:"schema" yaml:"schema"`
}

// SchemaObject is a simplified OpenAPI schema node.
type SchemaObject struct {
	Type       string                  `json:"type,omitempty" yaml:"type,omitempty"`
	Properties map[string]SchemaObject `json:"properties,omitempty" yaml:"properties,omitempty"`
	Items      *SchemaObject           `json:"items,omitempty" yaml:"items,omitempty"`
	Ref        string                  `json:"$ref,omitempty" yaml:"$ref,omitempty"`
}

// Components holds reusable spec objects.
type Components struct {
	Schemas map[string]SchemaObject `json:"schemas,omitempty" yaml:"schemas,omitempty"`
}

// Generate builds an OpenAPI 3.0 spec from the provided routes and info.
func Generate(routes []scan.Route, info Info) (*OpenAPISpec, error) {
	if info.Title == "" {
		info.Title = "API"
	}
	if info.Version == "" {
		info.Version = "1.0.0"
	}

	spec := &OpenAPISpec{
		OpenAPI: "3.0.0",
		Info: OpenAPIInfo{
			Title:       info.Title,
			Version:     info.Version,
			Description: info.Description,
		},
		Paths: make(map[string]PathItem),
	}

	// Group routes by normalized path.
	for _, route := range routes {
		oaPath := normalizePathParams(route.Path)
		item := spec.Paths[oaPath]

		op := buildOperation(route)

		switch strings.ToUpper(route.Method) {
		case "GET":
			item.Get = op
		case "POST":
			item.Post = op
		case "PUT":
			item.Put = op
		case "DELETE":
			item.Delete = op
		case "PATCH":
			item.Patch = op
		case "HEAD":
			item.Head = op
		case "OPTIONS":
			item.Options = op
		}

		spec.Paths[oaPath] = item
	}

	return spec, nil
}

// buildOperation constructs an Operation from a single Route.
func buildOperation(route scan.Route) *Operation {
	op := &Operation{
		OperationID: buildOperationID(route.Method, route.Path, route.Handler),
		Summary:     fmt.Sprintf("%s %s", strings.ToUpper(route.Method), route.Path),
		Tags:        []string{pathTag(route.Path)},
		Responses: map[string]Response{
			"200": {Description: "Successful response"},
			"400": {Description: "Bad request"},
			"500": {Description: "Internal server error"},
		},
	}

	// Extract path parameters.
	op.Parameters = extractPathParams(route.Path)

	// Add request body for mutating methods.
	method := strings.ToUpper(route.Method)
	if method == "POST" || method == "PUT" || method == "PATCH" {
		body := buildRequestBody(route.RequestBody)
		op.RequestBody = &body
	}

	// Add response schema if available.
	if route.Response != nil {
		op.Responses["200"] = Response{
			Description: "Successful response",
			Content: map[string]MediaType{
				"application/json": {Schema: scanSchemaToOpenAPI(route.Response)},
			},
		}
	}

	return op
}

// buildRequestBody creates a RequestBody from a scan.Schema, defaulting to a generic object.
func buildRequestBody(s *scan.Schema) RequestBody {
	var schema SchemaObject
	if s != nil {
		schema = scanSchemaToOpenAPI(s)
	} else {
		schema = SchemaObject{Type: "object"}
	}
	return RequestBody{
		Required: true,
		Content: map[string]MediaType{
			"application/json": {Schema: schema},
		},
	}
}

// scanSchemaToOpenAPI converts a scan.Schema to a SchemaObject.
func scanSchemaToOpenAPI(s *scan.Schema) SchemaObject {
	if s == nil {
		return SchemaObject{Type: "object"}
	}
	if s.Ref != "" {
		return SchemaObject{Ref: s.Ref}
	}

	obj := SchemaObject{Type: s.Type}

	if len(s.Properties) > 0 {
		obj.Properties = make(map[string]SchemaObject, len(s.Properties))
		for k, v := range s.Properties {
			v2 := v
			obj.Properties[k] = scanSchemaToOpenAPI(&v2)
		}
	}

	if s.Items != nil {
		child := scanSchemaToOpenAPI(s.Items)
		obj.Items = &child
	}

	return obj
}

// normalizePathParams converts framework-specific path param syntax to OpenAPI {param} syntax.
// e.g., ":id" → "{id}", "<id>" → "{id}", "<int:id>" → "{id}"
func normalizePathParams(path string) string {
	// Gin/Echo/Chi/Express: :param
	colonRe := strings.NewReplacer()
	_ = colonRe

	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + part[1:] + "}"
		} else if strings.HasPrefix(part, "<") && strings.HasSuffix(part, ">") {
			// Flask/FastAPI: <type:name> or <name>
			inner := part[1 : len(part)-1]
			if idx := strings.Index(inner, ":"); idx != -1 {
				inner = inner[idx+1:]
			}
			parts[i] = "{" + inner + "}"
		}
	}
	return strings.Join(parts, "/")
}

// extractPathParams discovers path parameters and returns Parameter descriptors.
func extractPathParams(path string) []Parameter {
	var params []Parameter
	for _, segment := range strings.Split(path, "/") {
		if strings.HasPrefix(segment, ":") {
			params = append(params, Parameter{
				Name:     segment[1:],
				In:       "path",
				Required: true,
				Schema:   SchemaObject{Type: "string"},
			})
		} else if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			params = append(params, Parameter{
				Name:     segment[1 : len(segment)-1],
				In:       "path",
				Required: true,
				Schema:   SchemaObject{Type: "string"},
			})
		} else if strings.HasPrefix(segment, "<") && strings.HasSuffix(segment, ">") {
			inner := segment[1 : len(segment)-1]
			if idx := strings.Index(inner, ":"); idx != -1 {
				inner = inner[idx+1:]
			}
			params = append(params, Parameter{
				Name:     inner,
				In:       "path",
				Required: true,
				Schema:   SchemaObject{Type: "string"},
			})
		}
	}
	return params
}

// buildOperationID creates a camelCase operationId from method, path, and handler.
func buildOperationID(method, path, handler string) string {
	if handler != "" {
		return strings.ToLower(method) + capitalize(sanitizeID(handler))
	}
	// Build from path segments.
	parts := []string{strings.ToLower(method)}
	for _, seg := range strings.Split(path, "/") {
		seg = strings.TrimPrefix(seg, ":")
		seg = strings.Trim(seg, "{}")
		seg = strings.Trim(seg, "<>")
		if seg != "" {
			parts = append(parts, capitalize(seg))
		}
	}
	return strings.Join(parts, "")
}

// pathTag derives a tag from the first path segment.
func pathTag(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return "default"
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// WriteYAML serializes the spec to a YAML file at path.
// It uses a manual serializer to avoid importing gopkg.in/yaml.v3.
func WriteYAML(spec *OpenAPISpec, path string) error {
	var sb strings.Builder
	writeSpecYAML(&sb, spec)
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// WriteJSON serializes the spec to a JSON file at path.
func WriteJSON(spec *OpenAPISpec, path string) error {
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("openapi: marshal json: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// writeSpecYAML writes a minimal YAML representation of the spec.
// This avoids an external YAML dependency by hand-writing the relevant subset.
func writeSpecYAML(sb *strings.Builder, spec *OpenAPISpec) {
	sb.WriteString("openapi: \"3.0.0\"\n")
	sb.WriteString("info:\n")
	sb.WriteString(fmt.Sprintf("  title: %q\n", spec.Info.Title))
	sb.WriteString(fmt.Sprintf("  version: %q\n", spec.Info.Version))
	if spec.Info.Description != "" {
		sb.WriteString(fmt.Sprintf("  description: %q\n", spec.Info.Description))
	}
	sb.WriteString("paths:\n")

	// Sort paths for deterministic output.
	paths := make([]string, 0, len(spec.Paths))
	for p := range spec.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		item := spec.Paths[p]
		sb.WriteString(fmt.Sprintf("  %q:\n", p))
		writeOperationYAML(sb, "get", item.Get)
		writeOperationYAML(sb, "post", item.Post)
		writeOperationYAML(sb, "put", item.Put)
		writeOperationYAML(sb, "delete", item.Delete)
		writeOperationYAML(sb, "patch", item.Patch)
		writeOperationYAML(sb, "head", item.Head)
		writeOperationYAML(sb, "options", item.Options)
	}
}

func writeOperationYAML(sb *strings.Builder, method string, op *Operation) {
	if op == nil {
		return
	}
	sb.WriteString(fmt.Sprintf("    %s:\n", method))
	if op.OperationID != "" {
		sb.WriteString(fmt.Sprintf("      operationId: %q\n", op.OperationID))
	}
	if op.Summary != "" {
		sb.WriteString(fmt.Sprintf("      summary: %q\n", op.Summary))
	}
	if len(op.Tags) > 0 {
		sb.WriteString("      tags:\n")
		for _, t := range op.Tags {
			sb.WriteString(fmt.Sprintf("        - %q\n", t))
		}
	}
	sb.WriteString("      responses:\n")
	// Sort response codes.
	codes := make([]string, 0, len(op.Responses))
	for c := range op.Responses {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, code := range codes {
		resp := op.Responses[code]
		sb.WriteString(fmt.Sprintf("        %q:\n", code))
		sb.WriteString(fmt.Sprintf("          description: %q\n", resp.Description))
	}
}
