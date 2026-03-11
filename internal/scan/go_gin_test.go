package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGinScanner(t *testing.T) {
	t.Run("basic routes", func(t *testing.T) {
		dir := t.TempDir()
		src := `package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.GET("/users", listUsers)
	r.POST("/users", createUser)
}
`
		writeFile(t, filepath.Join(dir, "main.go"), src)

		s := &GinScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 2 {
			t.Fatalf("want 2 routes, got %d", len(routes))
		}

		assertRoute(t, routes[0], "GET", "/users", "listUsers")
		assertRoute(t, routes[1], "POST", "/users", "createUser")
	})

	t.Run("group prefix", func(t *testing.T) {
		dir := t.TempDir()
		src := `package main

func setupRoutes(r *gin.Engine) {
	v1 := r.Group("/v1")
	v1.GET("/users", listUsers)
}
`
		writeFile(t, filepath.Join(dir, "routes.go"), src)

		s := &GinScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d", len(routes))
		}
		assertRoute(t, routes[0], "GET", "/v1/users", "listUsers")
	})

	t.Run("nested groups", func(t *testing.T) {
		dir := t.TempDir()
		src := `package main

func setupRoutes(r *gin.Engine) {
	v1 := r.Group("/v1")
	admin := v1.Group("/admin")
	admin.DELETE("/users/:id", deleteUser)
}
`
		writeFile(t, filepath.Join(dir, "routes.go"), src)

		s := &GinScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d", len(routes))
		}
		assertRoute(t, routes[0], "DELETE", "/v1/admin/users/:id", "deleteUser")
	})

	t.Run("empty path route normalises to slash", func(t *testing.T) {
		dir := t.TempDir()
		src := `package main

func setupRoutes(r *gin.Engine) {
	r.POST("", createRoot)
}
`
		writeFile(t, filepath.Join(dir, "routes.go"), src)

		s := &GinScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d", len(routes))
		}
		assertRoute(t, routes[0], "POST", "/", "createRoot")
	})

	t.Run("inline handler reported as inline", func(t *testing.T) {
		dir := t.TempDir()
		src := `package main

func setupRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
`
		writeFile(t, filepath.Join(dir, "routes.go"), src)

		s := &GinScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d: %+v", len(routes), routes)
		}
		if routes[0].Handler != "<inline>" {
			t.Errorf("want handler <inline>, got %q", routes[0].Handler)
		}
		if routes[0].Path != "/health" {
			t.Errorf("want path /health, got %q", routes[0].Path)
		}
	})

	t.Run("swagger annotation detected", func(t *testing.T) {
		dir := t.TempDir()
		src := `package main

// @Summary List all users
// @Tags users
// @Produce json
// @Success 200 {array} User
// @Router /users [get]
func listUsersHandler() {}

func setupRoutes(r *gin.Engine) {
	r.GET("/users", listUsersHandler)
}
`
		writeFile(t, filepath.Join(dir, "routes.go"), src)

		s := &GinScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d", len(routes))
		}
		if !routes[0].HasSwagger {
			t.Errorf("want HasSwagger true, got false")
		}
	})
}

// writeFile is a test helper that writes content to path, failing the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// assertRoute checks method, path, and handler on a Route.
func assertRoute(t *testing.T, r Route, method, path, handler string) {
	t.Helper()
	if r.Method != method {
		t.Errorf("method: want %q, got %q", method, r.Method)
	}
	if r.Path != path {
		t.Errorf("path: want %q, got %q", path, r.Path)
	}
	if r.Handler != handler {
		t.Errorf("handler: want %q, got %q", handler, r.Handler)
	}
}
