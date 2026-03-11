package scan

import (
	"path/filepath"
	"testing"
)

func TestDjangoScanner(t *testing.T) {
	t.Run("path() function routes", func(t *testing.T) {
		dir := t.TempDir()
		src := `from django.urls import path
from . import views

urlpatterns = [
    path('users/', views.UserList.as_view(), name='user-list'),
    path('users/<int:pk>/', views.UserDetail.as_view(), name='user-detail'),
]
`
		writeFile(t, filepath.Join(dir, "urls.py"), src)

		s := &DjangoScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 2 {
			t.Fatalf("want 2 routes, got %d: %+v", len(routes), routes)
		}

		assertRoute(t, routes[0], "ANY", "/users", "views.UserList")
		// Path parameter should be normalised to {pk}.
		if routes[1].Path != "/users/{pk}" {
			t.Errorf("want path /users/{pk}, got %q", routes[1].Path)
		}
	})

	t.Run("legacy url() function", func(t *testing.T) {
		dir := t.TempDir()
		src := `from django.conf.urls import url
from . import views

urlpatterns = [
    url(r'^users/$', views.user_list),
]
`
		writeFile(t, filepath.Join(dir, "urls.py"), src)

		s := &DjangoScanner{}
		routes, err := s.Scan(dir)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(routes) != 1 {
			t.Fatalf("want 1 route, got %d: %+v", len(routes), routes)
		}
		if routes[0].Path != "/users" {
			t.Errorf("want path /users, got %q", routes[0].Path)
		}
		if routes[0].Method != "ANY" {
			t.Errorf("want method ANY, got %q", routes[0].Method)
		}
	})
}
