// Package detect provides framework detection logic for API audit.
package detect

// Signature defines how to recognize a specific web framework within a project directory.
type Signature struct {
	// Language is the programming language (e.g., "Go", "Node", "Python").
	Language string
	// Framework is the framework name (e.g., "gin", "express", "fastapi").
	Framework string
	// Files is a list of filenames whose existence signals this framework.
	Files []string
	// Patterns is a list of substrings to search within those files.
	Patterns []string
	// Priority determines which signature wins when multiple match. Higher wins.
	Priority int
}

// signatures is the ordered list of all supported framework signatures.
var signatures = []Signature{
	// -------------------------------------------------------------------------
	// Go frameworks
	// -------------------------------------------------------------------------
	{
		Language:  "Go",
		Framework: "gin",
		Files:     []string{"go.mod", "go.sum"},
		Patterns:  []string{"github.com/gin-gonic/gin"},
		Priority:  90,
	},
	{
		Language:  "Go",
		Framework: "echo",
		Files:     []string{"go.mod"},
		Patterns:  []string{"github.com/labstack/echo"},
		Priority:  90,
	},
	{
		Language:  "Go",
		Framework: "chi",
		Files:     []string{"go.mod"},
		Patterns:  []string{"github.com/go-chi/chi"},
		Priority:  90,
	},
	{
		Language:  "Go",
		Framework: "fiber",
		Files:     []string{"go.mod"},
		Patterns:  []string{"github.com/gofiber/fiber"},
		Priority:  90,
	},
	{
		Language:  "Go",
		Framework: "stdlib",
		Files:     []string{"go.mod"},
		Patterns:  []string{"module "},
		Priority:  10,
	},
	// -------------------------------------------------------------------------
	// Node / TypeScript frameworks
	// -------------------------------------------------------------------------
	{
		Language:  "Node",
		Framework: "nestjs",
		Files:     []string{"package.json"},
		Patterns:  []string{"@nestjs/core", "@nestjs/common"},
		Priority:  95,
	},
	{
		Language:  "Node",
		Framework: "fastify",
		Files:     []string{"package.json"},
		Patterns:  []string{"\"fastify\""},
		Priority:  85,
	},
	{
		Language:  "Node",
		Framework: "koa",
		Files:     []string{"package.json"},
		Patterns:  []string{"\"koa\""},
		Priority:  80,
	},
	{
		Language:  "Node",
		Framework: "express",
		Files:     []string{"package.json"},
		Patterns:  []string{"\"express\""},
		Priority:  70,
	},
	// -------------------------------------------------------------------------
	// Python frameworks
	// -------------------------------------------------------------------------
	{
		Language:  "Python",
		Framework: "fastapi",
		Files:     []string{"requirements.txt", "pyproject.toml", "setup.py"},
		Patterns:  []string{"fastapi"},
		Priority:  90,
	},
	{
		Language:  "Python",
		Framework: "django-rest-framework",
		Files:     []string{"requirements.txt", "pyproject.toml", "setup.py"},
		Patterns:  []string{"djangorestframework", "rest_framework"},
		Priority:  85,
	},
	{
		Language:  "Python",
		Framework: "flask",
		Files:     []string{"requirements.txt", "pyproject.toml", "setup.py"},
		Patterns:  []string{"flask", "Flask"},
		Priority:  70,
	},
}
