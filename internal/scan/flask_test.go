package scan

import (
	"path/filepath"
	"testing"
)

func TestFlaskScanner(t *testing.T) {
	t.Run("basic route decorator", func(t *testing.T) {
		dir := t.TempDir()
		src := `from flask import Flask

app = Flask(__name__)

@app.route('/users', methods=['GET'])
def list_users():
    return []

@app.route('/users', methods=['POST'])
def create_user():
    return {}
`
		writeFile(t, filepath.Join(dir, "app.py"), src)

		s := &FlaskScanner{}
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

	t.Run("blueprint with prefix via register_blueprint", func(t *testing.T) {
		dir := t.TempDir()

		// Blueprint definition in a separate module.
		bpSrc := `from flask import Blueprint

auth = Blueprint('auth', __name__)

@auth.route('/login', methods=['POST'])
def login():
    return {}
`
		writeFile(t, filepath.Join(dir, "auth.py"), bpSrc)

		// Main app registers the blueprint with a url_prefix.
		appSrc := `from flask import Flask
from auth import auth

app = Flask(__name__)
app.register_blueprint(auth, url_prefix='/auth')
`
		writeFile(t, filepath.Join(dir, "app.py"), appSrc)

		s := &FlaskScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d: %+v", len(routes), routes)
		}
		assertRoute(t, routes[0], "POST", "/auth/login", "login")
	})
}
