package http233

import (
	"net/http/httptest"
	"testing"
)

func TestFindRoute(t *testing.T) {
	r := New()
	r.GET("/api/users", func(c *Context) {})
	r.GET("/api/posts", func(c *Context) {})

	tests := []struct {
		path     string
		wantNil  bool
		wantType nodeType
	}{
		{"/api/users", false, nodeStatic},
		{"/api/posts", false, nodeStatic},
		{"/api/unknown", true, 0},
		{"/", true, 0},
	}

	for _, tt := range tests {
		node, _ := r.findRoute(tt.path, nil)
		if tt.wantNil {
			if node != nil {
				t.Errorf("findRoute(%q) = %v, want nil", tt.path, node)
			}
		} else {
			if node == nil {
				t.Errorf("findRoute(%q) = nil, want node", tt.path)
			}
		}
	}
}

func TestGetHandler(t *testing.T) {
	node := &node{}
	called := false
	handler := func(c *Context) { called = true }
	node.setRoute("GET", nil, handler)

	entry := node.getRouteEntry("GET")
	if entry.handler == nil {
		t.Fatal("getRouteEntry(GET) = nil")
	}
	entry.handler(nil)
	if !called {
		t.Fatal("handler was not called")
	}

	if node.getRouteEntry("POST").handler != nil {
		t.Fatal("getRouteEntry(POST) should be nil")
	}
}

func TestServeHTTP(t *testing.T) {
	r := New()
	r.GET("/hello", func(c *Context) {
		c.String(200, "world")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/hello", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "world" {
		t.Errorf("body = %q, want %q", w.Body.String(), "world")
	}
}

func TestServeHTTPNotFound(t *testing.T) {
	r := New()
	r.GET("/exists", func(c *Context) {})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notexists", nil)
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestServeHTTPMethodNotAllowed(t *testing.T) {
	r := New()
	r.HandleMethodNotAllowed()
	r.GET("/resource", func(c *Context) {})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/resource", nil)
	r.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestContextParam(t *testing.T) {
	c := &Context{}
	c.params = []param{
		{key: "id", value: "42"},
		{key: "name", value: "test"},
	}

	if got := c.Param("id"); got != "42" {
		t.Errorf("Param(id) = %q, want %q", got, "42")
	}
	if got := c.Param("name"); got != "test" {
		t.Errorf("Param(name) = %q, want %q", got, "test")
	}
	if got := c.Param("missing"); got != "" {
		t.Errorf("Param(missing) = %q, want empty", got)
	}
}

func TestContextReset(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	c := &Context{
		params: []param{{key: "old", value: "value"}},
	}
	c.Reset(w, req)

	if c.Response != w {
		t.Error("Response not set after Reset")
	}
	if c.Request != req {
		t.Error("Request not set after Reset")
	}
	if len(c.params) != 0 {
		t.Error("params not cleared after Reset")
	}
}

func TestContextStatus(t *testing.T) {
	w := httptest.NewRecorder()
	c := &Context{Response: w}
	c.Status(201)
	if w.Code != 201 {
		t.Errorf("Status = %d, want 201", w.Code)
	}
}

func TestContextString(t *testing.T) {
	w := httptest.NewRecorder()
	c := &Context{Response: w}
	c.String(200, "hello %s", "world")
	if w.Code != 200 {
		t.Errorf("Status = %d, want 200", w.Code)
	}
	if w.Body.String() != "hello world" {
		t.Errorf("body = %q, want %q", w.Body.String(), "hello world")
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestContextJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c := &Context{Response: w}
	c.JSON(200, map[string]string{"key": "value"})
	if w.Code != 200 {
		t.Errorf("Status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestMultipleRoutes(t *testing.T) {
	r := New()
	r.GET("/a", func(c *Context) { c.String(200, "a") })
	r.GET("/b", func(c *Context) { c.String(200, "b") })
	r.GET("/ab", func(c *Context) { c.String(200, "ab") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/a", nil)
	r.ServeHTTP(w, req)
	if w.Body.String() != "a" {
		t.Errorf("GET /a body = %q, want %q", w.Body.String(), "a")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/b", nil)
	r.ServeHTTP(w, req)
	if w.Body.String() != "b" {
		t.Errorf("GET /b body = %q, want %q", w.Body.String(), "b")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/ab", nil)
	r.ServeHTTP(w, req)
	if w.Body.String() != "ab" {
		t.Errorf("GET /ab body = %q, want %q", w.Body.String(), "ab")
	}
}

func TestMultiMethodParamRoute(t *testing.T) {
	r := New()
	r.HandleMethodNotAllowed()
	r.GET("/users/:id", func(c *Context) { c.String(200, "get") })
	r.PUT("/users/:id", func(c *Context) { c.String(200, "put") })
	r.GET("/users/:id/profile", func(c *Context) { c.String(200, "profile") })

	tests := []struct {
		method string
		path   string
		want   int
		body   string
	}{
		{"GET", "/users/123", 200, "get"},
		{"PUT", "/users/123", 200, "put"},
		{"GET", "/users/123/profile", 200, "profile"},
	}

	for _, tc := range tests {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.want || w.Body.String() != tc.body {
			t.Errorf("%s %s: got %d %q want %d %q", tc.method, tc.path, w.Code, w.Body.String(), tc.want, tc.body)
		}
	}
}

func TestStaticBeforeParam(t *testing.T) {
	r := New()
	r.GET("/users/search", func(c *Context) { c.String(200, "search") })
	r.GET("/users/:id", func(c *Context) { c.String(200, "%s", c.Param("id")) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/users/search", nil))
	if w.Body.String() != "search" {
		t.Errorf("got %q", w.Body.String())
	}
}

func TestHandler(t *testing.T) {
	r := New()
	r.GET("/test", func(c *Context) { c.String(200, "ok") })

	handler := r.Handler()
	if handler == nil {
		t.Fatal("Handler() returned nil")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestStaticRoutes(t *testing.T) {
	router := New()

	router.GET("/", func(c *Context) {
		c.String(200, "home")
	})

	router.GET("/users", func(c *Context) {
		c.String(200, "users")
	})

	router.GET("/api/v1/users", func(c *Context) {
		c.String(200, "api users")
	})

	tests := []struct {
		path     string
		expected string
	}{
		{"/", "home"},
		{"/users", "users"},
		{"/api/v1/users", "api users"},
	}

	for _, test := range tests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", test.path, nil)

		router.ServeHTTP(w, req)

		if w.Body.String() != test.expected {
			t.Errorf("GET %s: expected %s, got %s", test.path, test.expected, w.Body.String())
		}
	}
}

func TestParameterRoutes(t *testing.T) {
	router := New()

	router.GET("/users/:id", func(c *Context) {
		c.String(200, "user %s", c.Param("id"))
	})

	router.GET("/files/*path", func(c *Context) {
		c.String(200, "file %s", c.Param("path"))
	})

	tests := []struct {
		path     string
		expected string
	}{
		{"/users/123", "user 123"},
		{"/files/documents/readme.md", "file documents/readme.md"},
	}

	for _, test := range tests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", test.path, nil)

		router.ServeHTTP(w, req)

		if w.Body.String() != test.expected {
			t.Errorf("GET %s: expected %s, got %s", test.path, test.expected, w.Body.String())
		}
	}
}
