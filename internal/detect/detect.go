// Package detect identifies the web framework and frontend setup of a project directory.
package detect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Framework holds the result of framework detection for a project.
type Framework struct {
	Language    string   `json:"language"`
	Framework   string   `json:"framework"`
	Version     string   `json:"version,omitempty"`
	EntryPoints []string `json:"entryPoints,omitempty"`
	HasFrontend bool     `json:"hasFrontend"`
	FrontendDir string   `json:"frontendDir,omitempty"`
	HasSwagger  bool     `json:"hasSwagger"`
	Confidence  float64  `json:"confidence"`
}

// Detect scans dir and returns the best-matching Framework.
// It returns an error only for unexpected I/O failures; an unrecognized project
// returns a Framework with Confidence == 0.
func Detect(dir string) (*Framework, error) {
	dir = filepath.Clean(dir)

	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("detect: directory not accessible: %w", err)
	}

	best, bestScore := matchSignatures(dir)

	fw := &Framework{
		Language:   best.Language,
		Framework:  best.Framework,
		Confidence: scoreToConfidence(bestScore, best.Priority),
	}

	fw.EntryPoints = findEntryPoints(dir, best.Framework)
	fw.HasFrontend, fw.FrontendDir = detectFrontend(dir)
	fw.HasSwagger = detectSwagger(dir)
	fw.Version = detectVersion(dir, best)

	return fw, nil
}

// matchSignatures scores each signature against the directory and returns the winner.
func matchSignatures(dir string) (Signature, int) {
	type scored struct {
		sig   Signature
		score int
	}

	var results []scored

	for _, sig := range signatures {
		score := scoreSignature(dir, sig)
		if score > 0 {
			results = append(results, scored{sig, score})
		}
	}

	if len(results) == 0 {
		return Signature{Language: "Unknown", Framework: "unknown"}, 0
	}

	best := results[0]
	for _, r := range results[1:] {
		if r.score > best.score || (r.score == best.score && r.sig.Priority > best.sig.Priority) {
			best = r
		}
	}

	return best.sig, best.score
}

// scoreSignature returns a score > 0 when the signature matches.
func scoreSignature(dir string, sig Signature) int {
	patternHits := 0

	scoreFile := func(fpath string) {
		data, err := os.ReadFile(fpath)
		if err != nil {
			return
		}
		content := string(data)
		for _, pattern := range sig.Patterns {
			if strings.Contains(content, pattern) {
				patternHits++
			}
		}
	}

	for _, filename := range sig.Files {
		scoreFile(filepath.Join(dir, filename))
	}

	// Also search files matched by glob patterns relative to the project root.
	// This handles projects that split dependency files across subdirectories
	// (e.g. requirements/*.txt in Flask/FastAPI projects).
	for _, glob := range sig.FileGlobs {
		matches, err := filepath.Glob(filepath.Join(dir, glob))
		if err != nil {
			continue
		}
		for _, m := range matches {
			scoreFile(m)
		}
	}

	if patternHits == 0 {
		return 0
	}

	return sig.Priority * patternHits
}

// scoreToConfidence converts a raw score and priority into a 0.0–1.0 confidence value.
func scoreToConfidence(score, priority int) float64 {
	if priority == 0 {
		return 0
	}
	c := float64(score) / float64(priority*3)
	if c > 1.0 {
		c = 1.0
	}
	return c
}

// findEntryPoints returns likely route-registration files for the given framework.
func findEntryPoints(dir, framework string) []string {
	var candidates []string

	switch framework {
	case "gin", "echo", "chi", "fiber", "stdlib":
		// Check for cmd/**/main.go pattern (common in Go projects).
		cmdDir := filepath.Join(dir, "cmd")
		if entries, err := os.ReadDir(cmdDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					mg := filepath.Join("cmd", e.Name(), "main.go")
					if _, err := os.Stat(filepath.Join(dir, mg)); err == nil {
						candidates = append(candidates, mg)
					}
				}
			}
		}
		// Also look for route registration files recursively.
		for _, pattern := range []string{"**/route*.go", "**/router*.go", "**/routes*.go", "**/server.go"} {
			if matches, err := filepath.Glob(filepath.Join(dir, pattern)); err == nil {
				for _, m := range matches {
					rel, _ := filepath.Rel(dir, m)
					candidates = append(candidates, rel)
				}
			}
		}
		candidates = append(candidates,
			"main.go",
			"server.go",
			"router.go",
			"routes.go",
			"api/routes.go",
			"internal/server/server.go",
		)
	case "express":
		candidates = []string{
			"index.js", "index.ts",
			"app.js", "app.ts",
			"server.js", "server.ts",
			"src/index.ts", "src/app.ts",
			"routes/index.js", "routes/index.ts",
		}
	case "nestjs":
		candidates = []string{
			"src/main.ts",
			"src/app.module.ts",
		}
	case "fastapi", "flask":
		candidates = []string{
			"main.py", "app.py", "server.py",
			"api/main.py", "api/app.py",
		}
	case "django-rest-framework":
		candidates = []string{
			"urls.py",
			"api/urls.py",
		}
	}

	var found []string
	for _, rel := range candidates {
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			found = append(found, rel)
		}
	}
	return found
}

// detectFrontend returns true and the relative directory path when a frontend
// project is co-located with the backend.
func detectFrontend(dir string) (bool, string) {
	// Explicit frontend directories.
	frontendDirs := []string{"frontend", "client", "web", "ui", "dashboard"}
	for _, d := range frontendDirs {
		pkg := filepath.Join(dir, d, "package.json")
		if _, err := os.Stat(pkg); err == nil {
			if hasFrontendDep(pkg) {
				return true, d
			}
		}
	}

	// Detect src/ with component/view/page dirs (SPA without a wrapper dir).
	srcDirs := []string{
		filepath.Join(dir, "src", "components"),
		filepath.Join(dir, "src", "views"),
		filepath.Join(dir, "src", "pages"),
	}
	for _, d := range srcDirs {
		if _, err := os.Stat(d); err == nil {
			return true, "src"
		}
	}

	// package.json at root with frontend deps.
	rootPkg := filepath.Join(dir, "package.json")
	if _, err := os.Stat(rootPkg); err == nil {
		if hasFrontendDep(rootPkg) {
			return true, "."
		}
	}

	return false, ""
}

// hasFrontendDep reports whether the package.json at path contains a known
// frontend framework dependency.
func hasFrontendDep(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		// Fall back to string search.
		content := string(data)
		for _, kw := range []string{"\"vue\"", "\"react\"", "\"angular\"", "\"svelte\"", "\"next\"", "\"nuxt\""} {
			if strings.Contains(content, kw) {
				return true
			}
		}
		return false
	}

	frontendPkgs := []string{"vue", "react", "react-dom", "@angular/core", "svelte", "next", "nuxt", "@nuxtjs/composition-api"}
	allDeps := pkg.Dependencies
	if allDeps == nil {
		allDeps = map[string]string{}
	}
	for k, v := range pkg.DevDependencies {
		allDeps[k] = v
	}
	for _, fp := range frontendPkgs {
		if _, ok := allDeps[fp]; ok {
			return true
		}
	}
	return false
}

// detectSwagger returns true when OpenAPI/Swagger artifacts or annotations are found.
func detectSwagger(dir string) bool {
	// Known output files / dirs.
	swaggerFiles := []string{
		"swagger.json", "swagger.yaml", "swagger.yml",
		"openapi.json", "openapi.yaml", "openapi.yml",
		"docs/swagger.json", "docs/swagger.yaml",
		"docs/openapi.yaml", "docs/openapi.json",
		"api/swagger.yaml",
	}
	for _, f := range swaggerFiles {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}

	// Swaggo docs directory.
	swagDirs := []string{"docs", "api/docs"}
	for _, d := range swagDirs {
		if _, err := os.Stat(filepath.Join(dir, d, "swagger.json")); err == nil {
			return true
		}
	}

	// Check package.json for swagger-jsdoc (Node).
	pkgPath := filepath.Join(dir, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		if strings.Contains(string(data), "swagger-jsdoc") || strings.Contains(string(data), "swagger-ui") {
			return true
		}
	}

	return false
}

// detectVersion attempts to parse the framework version from known manifest files.
func detectVersion(dir string, sig Signature) string {
	switch sig.Language {
	case "Go":
		return goModVersion(dir, sig.Framework)
	case "Node":
		return nodeVersion(dir, sig.Framework)
	case "Python":
		return pythonVersion(dir, sig.Framework)
	}
	return ""
}

func goModVersion(dir, framework string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}

	var prefix string
	switch framework {
	case "gin":
		prefix = "github.com/gin-gonic/gin"
	case "echo":
		prefix = "github.com/labstack/echo"
	case "chi":
		prefix = "github.com/go-chi/chi"
	case "fiber":
		prefix = "github.com/gofiber/fiber"
	}

	if prefix == "" {
		return ""
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

func nodeVersion(dir, framework string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}

	all := pkg.Dependencies
	if all == nil {
		all = map[string]string{}
	}
	for k, v := range pkg.DevDependencies {
		all[k] = v
	}

	keys := map[string]string{
		"express": "express",
		"nestjs":  "@nestjs/core",
		"fastify": "fastify",
		"koa":     "koa",
	}

	if dep, ok := keys[framework]; ok {
		return all[dep]
	}
	return ""
}

func pythonVersion(dir, framework string) string {
	reqPath := filepath.Join(dir, "requirements.txt")
	data, err := os.ReadFile(reqPath)
	if err != nil {
		return ""
	}

	var prefix string
	switch framework {
	case "fastapi":
		prefix = "fastapi"
	case "flask":
		prefix = "Flask"
	case "django-rest-framework":
		prefix = "djangorestframework"
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		lowerPrefix := strings.ToLower(prefix)
		if strings.HasPrefix(lower, lowerPrefix) {
			// e.g. "fastapi==0.95.0" or "fastapi>=0.95.0"
			for _, sep := range []string{"==", ">=", "<=", "~="} {
				if idx := strings.Index(line, sep); idx != -1 {
					return strings.TrimSpace(line[idx+len(sep):])
				}
			}
			return ""
		}
	}
	return ""
}
