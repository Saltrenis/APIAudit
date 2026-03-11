package scan

import (
	"path/filepath"
	"testing"
)

func TestNestJSScanner(t *testing.T) {
	t.Run("basic controller with method decorators", func(t *testing.T) {
		dir := t.TempDir()
		src := `import { Controller, Get, Post } from '@nestjs/common';

@Controller('users')
export class UsersController {
  @Get()
  findAll() {
    return [];
  }

  @Post()
  create() {
    return {};
  }
}
`
		writeFile(t, filepath.Join(dir, "users.controller.ts"), src)

		s := &NestJSScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 2 {
			t.Fatalf("want 2 routes, got %d: %+v", len(routes), routes)
		}

		hasGET := false
		hasPOST := false
		for _, r := range routes {
			if r.Path == "/users" {
				switch r.Method {
				case "GET":
					hasGET = true
				case "POST":
					hasPOST = true
				}
			}
		}
		if !hasGET {
			t.Errorf("want GET /users, routes: %+v", routes)
		}
		if !hasPOST {
			t.Errorf("want POST /users, routes: %+v", routes)
		}
	})

	t.Run("object-form controller with method sub-path", func(t *testing.T) {
		dir := t.TempDir()
		src := `import { Controller, Post } from '@nestjs/common';

@Controller({ path: 'auth', version: '1' })
export class AuthController {
  @Post('login')
  login() {
    return {};
  }
}
`
		writeFile(t, filepath.Join(dir, "auth.controller.ts"), src)

		s := &NestJSScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d: %+v", len(routes), routes)
		}
		assertRoute(t, routes[0], "POST", "/auth/login", "login")
	})

	t.Run("swagger ApiTags detected", func(t *testing.T) {
		dir := t.TempDir()
		src := `import { Controller, Get } from '@nestjs/common';
import { ApiTags } from '@nestjs/swagger';

@ApiTags('users')
@Controller('users')
export class UsersController {
  @Get()
  findAll() {
    return [];
  }
}
`
		writeFile(t, filepath.Join(dir, "users.controller.ts"), src)

		s := &NestJSScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d: %+v", len(routes), routes)
		}
		if !routes[0].HasSwagger {
			t.Errorf("want HasSwagger true, got false")
		}
	})
}
