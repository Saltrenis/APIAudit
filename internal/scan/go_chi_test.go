package scan

import (
	"path/filepath"
	"testing"
)

func TestChiScanner(t *testing.T) {
	t.Run("basic routes", func(t *testing.T) {
		dir := t.TempDir()
		src := `package main

import (
	"net/http"
	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()
	r.Get("/users", listUsers)
	r.Post("/users", createUser)
}
`
		writeFile(t, filepath.Join(dir, "main.go"), src)

		s := &ChiScanner{}
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

	t.Run("r.Route nesting resolves prefix", func(t *testing.T) {
		dir := t.TempDir()
		src := `package main

import "github.com/go-chi/chi/v5"

func setupRoutes(r chi.Router) {
	r.Route("/users", func(r chi.Router) {
		r.Get("/", listUsers)
		r.Post("/", createUser)
	})
}
`
		writeFile(t, filepath.Join(dir, "routes.go"), src)

		s := &ChiScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 2 {
			t.Fatalf("want 2 routes, got %d: %+v", len(routes), routes)
		}
		// r.Get("/") inside r.Route("/users") should resolve to /users (not /users/).
		for _, r := range routes {
			if r.Path != "/users" {
				t.Errorf("want path /users, got %q", r.Path)
			}
		}
	})

	t.Run("r.Mount emits MOUNT route", func(t *testing.T) {
		dir := t.TempDir()
		src := `package main

import "github.com/go-chi/chi/v5"

func setupRoutes(r chi.Router) {
	r.Mount("/admin", adminRouter())
}
`
		writeFile(t, filepath.Join(dir, "routes.go"), src)

		s := &ChiScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route (MOUNT), got %d: %+v", len(routes), routes)
		}
		if routes[0].Method != "MOUNT" {
			t.Errorf("want method MOUNT, got %q", routes[0].Method)
		}
		if routes[0].Path != "/admin/*" {
			t.Errorf("want path /admin/*, got %q", routes[0].Path)
		}
	})
}
