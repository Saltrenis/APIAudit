package scan

import (
	"path/filepath"
	"testing"
)

func TestEchoScanner(t *testing.T) {
	t.Run("basic routes", func(t *testing.T) {
		dir := t.TempDir()
		src := `package main

import "github.com/labstack/echo/v4"

func main() {
	e := echo.New()
	e.GET("/users", listUsers)
	e.POST("/users", createUser)
}
`
		writeFile(t, filepath.Join(dir, "main.go"), src)

		s := &EchoScanner{}
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

	t.Run("group variable prefix", func(t *testing.T) {
		dir := t.TempDir()
		src := `package main

func setupRoutes(e *echo.Echo) {
	v1 := e.Group("/v1")
	v1.GET("/users", listUsers)
}
`
		writeFile(t, filepath.Join(dir, "routes.go"), src)

		s := &EchoScanner{}
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

func setupRoutes(e *echo.Echo) {
	v1 := e.Group("/v1")
	users := v1.Group("/users")
	users.DELETE("/:id", deleteUser)
}
`
		writeFile(t, filepath.Join(dir, "routes.go"), src)

		s := &EchoScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d", len(routes))
		}
		assertRoute(t, routes[0], "DELETE", "/v1/users/:id", "deleteUser")
	})
}
