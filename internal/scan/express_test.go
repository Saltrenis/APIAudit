package scan

import (
	"path/filepath"
	"testing"
)

func TestExpressScanner(t *testing.T) {
	t.Run("basic routes", func(t *testing.T) {
		dir := t.TempDir()
		src := `const express = require('express');
const app = express();

app.get('/users', listUsers);
app.post('/users', createUser);
`
		writeFile(t, filepath.Join(dir, "app.js"), src)

		s := &ExpressScanner{}
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

	t.Run("router prefix via use", func(t *testing.T) {
		dir := t.TempDir()
		src := `const express = require('express');
const app = express();
const router = express.Router();

app.use('/api', router);

router.get('/users', listUsers);
`
		writeFile(t, filepath.Join(dir, "app.js"), src)

		s := &ExpressScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}

		// Find the /api/users route (prefix applied).
		found := false
		for _, r := range routes {
			if r.Path == "/api/users" && r.Method == "GET" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("want route GET /api/users, routes: %+v", routes)
		}
	})

	t.Run("route chaining", func(t *testing.T) {
		dir := t.TempDir()
		src := `const router = express.Router();

router.route('/users').get(listUsers).post(createUser);
`
		writeFile(t, filepath.Join(dir, "routes.js"), src)

		s := &ExpressScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}

		hasGET := false
		hasPOST := false
		for _, r := range routes {
			if r.Path == "/users" {
				if r.Method == "GET" {
					hasGET = true
				}
				if r.Method == "POST" {
					hasPOST = true
				}
			}
		}
		if !hasGET {
			t.Errorf("want GET /users from chain, routes: %+v", routes)
		}
		if !hasPOST {
			t.Errorf("want POST /users from chain, routes: %+v", routes)
		}
	})

	t.Run("middleware in args uses last as handler", func(t *testing.T) {
		dir := t.TempDir()
		src := `const router = express.Router();

router.post('/users', validate(schema), auth('admin'), controller.create);
`
		writeFile(t, filepath.Join(dir, "routes.js"), src)

		s := &ExpressScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d: %+v", len(routes), routes)
		}
		if routes[0].Handler != "controller.create" {
			t.Errorf("want handler controller.create, got %q", routes[0].Handler)
		}
		assertRoute(t, routes[0], "POST", "/users", "controller.create")
	})

	t.Run("swagger JSDoc detected", func(t *testing.T) {
		dir := t.TempDir()
		src := `/**
 * @swagger
 * /users:
 *   get:
 *     summary: List users
 */
app.get('/users', listUsers);
`
		writeFile(t, filepath.Join(dir, "app.js"), src)

		s := &ExpressScanner{}
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
