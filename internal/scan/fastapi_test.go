package scan

import (
	"path/filepath"
	"testing"
)

func TestFastAPIScanner(t *testing.T) {
	t.Run("basic decorator", func(t *testing.T) {
		dir := t.TempDir()
		src := `from fastapi import FastAPI

app = FastAPI()

@app.get("/users")
async def list_users():
    return []

@app.post("/users")
async def create_user():
    pass
`
		writeFile(t, filepath.Join(dir, "main.py"), src)

		s := &FastAPIScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 2 {
			t.Fatalf("want 2 routes, got %d: %+v", len(routes), routes)
		}
		assertRoute(t, routes[0], "GET", "/users", "list_users")
		assertRoute(t, routes[1], "POST", "/users", "create_user")
	})

	t.Run("router with prefix", func(t *testing.T) {
		dir := t.TempDir()
		src := `from fastapi import APIRouter

router = APIRouter(prefix="/api")

@router.get("/users")
async def list_users():
    return []
`
		writeFile(t, filepath.Join(dir, "users.py"), src)

		s := &FastAPIScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d: %+v", len(routes), routes)
		}
		assertRoute(t, routes[0], "GET", "/api/users", "list_users")
	})

	t.Run("multi-line decorator", func(t *testing.T) {
		dir := t.TempDir()
		src := `from fastapi import APIRouter
from typing import List

router = APIRouter(prefix="/api")

@router.get(
    "/users",
    response_model=List[User]
)
async def list_users():
    return []
`
		writeFile(t, filepath.Join(dir, "users.py"), src)

		s := &FastAPIScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d: %+v", len(routes), routes)
		}
		assertRoute(t, routes[0], "GET", "/api/users", "list_users")
	})
}
