// Package scan extracts API routes from source files of supported frameworks.
package scan

import "fmt"

// Route represents a single API endpoint discovered in source code.
type Route struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Handler     string   `json:"handler"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Middleware  []string `json:"middleware,omitempty"`
	RequestBody *Schema  `json:"requestBody,omitempty"`
	Response    *Schema  `json:"response,omitempty"`
	HasSwagger  bool     `json:"hasSwagger"`
}

// Schema is a simplified OpenAPI-compatible schema descriptor.
type Schema struct {
	Type       string            `json:"type,omitempty"`
	Properties map[string]Schema `json:"properties,omitempty"`
	Items      *Schema           `json:"items,omitempty"`
	Required   []string          `json:"required,omitempty"`
	Ref        string            `json:"$ref,omitempty"`
}

// Scanner is the interface implemented by each framework-specific route extractor.
type Scanner interface {
	// Name returns the framework identifier this scanner handles.
	Name() string
	// Scan walks dir and returns all discovered routes.
	Scan(dir string) ([]Route, error)
}

// GetScanner returns the appropriate Scanner for the given framework name.
// It returns an error if the framework is not supported.
func GetScanner(framework string) (Scanner, error) {
	switch framework {
	case "gin":
		return &GinScanner{}, nil
	case "echo":
		return &EchoScanner{}, nil
	case "chi":
		return &ChiScanner{}, nil
	case "fiber":
		// Fiber uses very similar patterns to Gin.
		return &GinScanner{nameOverride: "fiber"}, nil
	case "stdlib":
		return &StdlibScanner{}, nil
	case "express":
		return &ExpressScanner{}, nil
	case "nestjs":
		return &NestJSScanner{}, nil
	case "fastapi":
		return &FastAPIScanner{}, nil
	case "flask":
		return &FlaskScanner{}, nil
	case "django-rest-framework", "django":
		return &DjangoScanner{}, nil
	default:
		return nil, fmt.Errorf("scan: unsupported framework %q", framework)
	}
}
