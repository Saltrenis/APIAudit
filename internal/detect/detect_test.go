package detect

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a test helper that writes content to path, creating
// intermediate directories as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func TestDetect_GoGin(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), `module example.com/myapp

go 1.21

require github.com/gin-gonic/gin v1.9.1
`)

	fw, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if fw.Framework != "gin" {
		t.Errorf("want framework gin, got %q", fw.Framework)
	}
	if fw.Language != "Go" {
		t.Errorf("want language Go, got %q", fw.Language)
	}
	if fw.Confidence == 0 {
		t.Errorf("want Confidence > 0, got 0")
	}
}

func TestDetect_Express(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "name": "my-api",
  "dependencies": {
    "express": "^4.18.0"
  }
}
`)

	fw, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if fw.Framework != "express" {
		t.Errorf("want framework express, got %q", fw.Framework)
	}
	if fw.Language != "Node" {
		t.Errorf("want language Node, got %q", fw.Language)
	}
	if fw.Confidence == 0 {
		t.Errorf("want Confidence > 0, got 0")
	}
}

func TestDetect_FastAPI(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), `fastapi==0.110.0
uvicorn>=0.29.0
`)

	fw, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if fw.Framework != "fastapi" {
		t.Errorf("want framework fastapi, got %q", fw.Framework)
	}
	if fw.Language != "Python" {
		t.Errorf("want language Python, got %q", fw.Language)
	}
	if fw.Confidence == 0 {
		t.Errorf("want Confidence > 0, got 0")
	}
}

func TestDetect_FrontendFromSubDir(t *testing.T) {
	dir := t.TempDir()

	// Backend: express API.
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "name": "backend",
  "dependencies": {
    "express": "^4.18.0"
  }
}
`)

	// Frontend: vue app in ./frontend/
	writeFile(t, filepath.Join(dir, "frontend", "package.json"), `{
  "name": "frontend",
  "dependencies": {
    "vue": "^3.0.0"
  }
}
`)

	fw, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if !fw.HasFrontend {
		t.Errorf("want HasFrontend true, got false")
	}
	if fw.FrontendDir != "frontend" {
		t.Errorf("want FrontendDir frontend, got %q", fw.FrontendDir)
	}
}

func TestDetect_NoFramework(t *testing.T) {
	dir := t.TempDir()
	// Write a README only — no framework indicator.
	writeFile(t, filepath.Join(dir, "README.md"), "# My project\n")

	fw, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if fw.Confidence != 0 {
		t.Errorf("want Confidence 0, got %f", fw.Confidence)
	}
}
