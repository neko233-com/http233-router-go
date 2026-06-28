package http233

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func setupBusinessAPI() *Router {
	r := New()
	r.HandleMethodNotAllowed()
	r.Use(Recovery())
	r.Use(func(c *Context) {
		c.Set("request_id", "req-1")
		c.Next()
	})

	r.GET("/health", func(c *Context) { c.JSON(200, map[string]string{"status": "ok"}) })
	r.GET("/health/ready", func(c *Context) { c.JSON(200, map[string]string{"ready": "ok"}) })
	r.GET("/health/live", func(c *Context) { c.JSON(200, map[string]string{"live": "ok"}) })

	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/login", func(c *Context) { c.JSON(200, map[string]string{"token": "xxx"}) })
		auth.POST("/register", func(c *Context) { c.JSON(201, map[string]string{"id": "1"}) })
		auth.POST("/refresh", func(c *Context) { c.JSON(200, map[string]string{"token": "yyy"}) })
		auth.POST("/logout", func(c *Context) { c.JSON(200, map[string]string{"status": "logged_out"}) })
		auth.GET("/verify", func(c *Context) { c.JSON(200, map[string]string{"valid": "true"}) })
		auth.DELETE("/revoke", func(c *Context) { c.JSON(200, map[string]string{"status": "revoked"}) })
	}

	users := r.Group("/api/v1/users")
	{
		users.GET("", func(c *Context) { c.JSON(200, []string{"user1", "user2"}) })
		users.POST("", func(c *Context) { c.JSON(201, map[string]string{"id": "1"}) })
		users.GET("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		users.PUT("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id"), "updated": "true"}) })
		users.DELETE("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id"), "deleted": "true"}) })
		users.GET("/:id/profile", func(c *Context) { c.JSON(200, map[string]string{"user_id": c.Param("id"), "name": "test"}) })
		users.PUT("/:id/profile", func(c *Context) { c.JSON(200, map[string]string{"user_id": c.Param("id"), "updated": "true"}) })
		users.GET("/:id/settings", func(c *Context) { c.JSON(200, map[string]string{"user_id": c.Param("id")}) })
		users.PUT("/:id/settings", func(c *Context) { c.JSON(200, map[string]string{"user_id": c.Param("id"), "settings_updated": "true"}) })
		users.POST("/:id/avatar", func(c *Context) { c.JSON(200, map[string]string{"user_id": c.Param("id")}) })
		users.GET("/:id/followers", func(c *Context) { c.JSON(200, []string{"follower1"}) })
		users.GET("/:id/following", func(c *Context) { c.JSON(200, []string{"following1"}) })
		users.POST("/:id/follow", func(c *Context) { c.JSON(200, map[string]string{"status": "followed"}) })
		users.DELETE("/:id/follow", func(c *Context) { c.JSON(200, map[string]string{"status": "unfollowed"}) })
		users.GET("/search", func(c *Context) { c.JSON(200, []string{"search_result1"}) })
	}

	posts := r.Group("/api/v1/posts")
	{
		posts.GET("", func(c *Context) { c.JSON(200, []string{"post1"}) })
		posts.POST("", func(c *Context) { c.JSON(201, map[string]string{"id": "1"}) })
		posts.GET("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		posts.PUT("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		posts.DELETE("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		posts.GET("/:id/comments", func(c *Context) { c.JSON(200, []string{"comment1"}) })
		posts.POST("/:id/comments", func(c *Context) { c.JSON(201, map[string]string{"post_id": c.Param("id")}) })
		posts.POST("/:id/like", func(c *Context) { c.JSON(200, map[string]string{"status": "liked"}) })
		posts.DELETE("/:id/like", func(c *Context) { c.JSON(200, map[string]string{"status": "unliked"}) })
		posts.GET("/:id/shares", func(c *Context) { c.JSON(200, []string{"share1"}) })
		posts.POST("/:id/share", func(c *Context) { c.JSON(200, map[string]string{"status": "shared"}) })
		posts.GET("/tags/:tag", func(c *Context) { c.JSON(200, map[string]string{"tag": c.Param("tag")}) })
	}

	comments := r.Group("/api/v1/comments")
	{
		comments.GET("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		comments.PUT("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		comments.DELETE("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		comments.POST("/:id/reply", func(c *Context) { c.JSON(201, map[string]string{"parent_id": c.Param("id")}) })
		comments.POST("/:id/like", func(c *Context) { c.JSON(200, map[string]string{"status": "liked"}) })
		comments.DELETE("/:id/like", func(c *Context) { c.JSON(200, map[string]string{"status": "unliked"}) })
		comments.GET("/:id/replies", func(c *Context) { c.JSON(200, []string{"reply1"}) })
		comments.POST("/batch", func(c *Context) { c.JSON(200, map[string]string{"status": "batch_created"}) })
	}

	admin := r.Group("/api/v1/admin")
	admin.Use(func(c *Context) {
		if c.Query("token") != "admin-secret" {
			c.AbortWithStatus(403)
			return
		}
		c.Next()
	})
	{
		admin.GET("/users", func(c *Context) { c.JSON(200, []string{"admin_user1"}) })
		admin.DELETE("/users/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		admin.GET("/stats", func(c *Context) { c.JSON(200, map[string]int{"users": 100}) })
		admin.POST("/broadcast", func(c *Context) { c.JSON(200, map[string]string{"status": "sent"}) })
		admin.GET("/logs", func(c *Context) { c.JSON(200, []string{"log1"}) })
		admin.DELETE("/logs/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		admin.POST("/config", func(c *Context) { c.JSON(200, map[string]string{"status": "updated"}) })
		admin.GET("/config", func(c *Context) { c.JSON(200, map[string]string{"key": "value"}) })
		admin.POST("/cache/clear", func(c *Context) { c.JSON(200, map[string]string{"status": "cleared"}) })
		admin.GET("/cache/stats", func(c *Context) { c.JSON(200, map[string]int{"hits": 100}) })
	}

	files := r.Group("/api/v1/files")
	{
		files.POST("/upload", func(c *Context) { c.JSON(200, map[string]string{"status": "uploaded"}) })
		files.GET("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		files.DELETE("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		files.GET("/:id/download", func(c *Context) { c.Data(200, "application/octet-stream", []byte("file-content")) })
		files.POST("/batch/upload", func(c *Context) { c.JSON(200, map[string]string{"status": "batch_uploaded"}) })
		files.GET("/list", func(c *Context) { c.JSON(200, []string{"file1"}) })
		files.POST("/move", func(c *Context) { c.JSON(200, map[string]string{"status": "moved"}) })
		files.POST("/copy", func(c *Context) { c.JSON(200, map[string]string{"status": "copied"}) })
	}

	search := r.Group("/api/v1/search")
	{
		search.GET("", func(c *Context) { c.JSON(200, []string{"result1"}) })
		search.GET("/users", func(c *Context) { c.JSON(200, []string{"user_result"}) })
		search.GET("/posts", func(c *Context) { c.JSON(200, []string{"post_result"}) })
		search.GET("/comments", func(c *Context) { c.JSON(200, []string{"comment_result"}) })
		search.GET("/tags", func(c *Context) { c.JSON(200, []string{"tag_result"}) })
	}

	notifications := r.Group("/api/v1/notifications")
	{
		notifications.GET("", func(c *Context) { c.JSON(200, []string{"notif1"}) })
		notifications.GET("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		notifications.PUT("/:id/read", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		notifications.DELETE("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		notifications.POST("/read-all", func(c *Context) { c.JSON(200, map[string]string{"status": "all_read"}) })
	}

	messages := r.Group("/api/v1/messages")
	{
		messages.GET("", func(c *Context) { c.JSON(200, []string{"msg1"}) })
		messages.POST("", func(c *Context) { c.JSON(201, map[string]string{"id": "1"}) })
		messages.GET("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		messages.DELETE("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		messages.GET("/conversations/:userId", func(c *Context) { c.JSON(200, map[string]string{"user_id": c.Param("userId")}) })
		messages.POST("/:id/read", func(c *Context) { c.JSON(200, map[string]string{"status": "read"}) })
	}

	r.Static("/static", "./testdata")
	r.StaticFile("/favicon.ico", "./testdata/favicon.ico")
	r.StaticFile("/robots.txt", "./testdata/robots.txt")
	r.StaticFile("/sitemap.xml", "./testdata/sitemap.xml")
	r.StaticFile("/manifest.json", "./testdata/manifest.json")

	r.GET("/files/*path", func(c *Context) { c.JSON(200, map[string]string{"path": c.Param("path")}) })
	r.GET("/docs/*path", func(c *Context) { c.JSON(200, map[string]string{"path": c.Param("path")}) })
	r.GET("/assets/*path", func(c *Context) { c.JSON(200, map[string]string{"path": c.Param("path")}) })

	return r
}

func TestBusinessAPIRoutes(t *testing.T) {
	r := setupBusinessAPI()

	tests := []struct {
		method       string
		path         string
		expectedCode int
		bodyContains string
	}{
		{"GET", "/health", 200, "ok"},
		{"GET", "/health/ready", 200, "ok"},
		{"GET", "/health/live", 200, "ok"},

		{"POST", "/api/v1/auth/login", 200, "token"},
		{"POST", "/api/v1/auth/register", 201, "id"},
		{"POST", "/api/v1/auth/refresh", 200, "token"},
		{"POST", "/api/v1/auth/logout", 200, "logged_out"},
		{"GET", "/api/v1/auth/verify", 200, "valid"},
		{"DELETE", "/api/v1/auth/revoke", 200, "revoked"},

		{"GET", "/api/v1/users", 200, "user1"},
		{"POST", "/api/v1/users", 201, "id"},
		{"GET", "/api/v1/users/123", 200, "123"},
		{"PUT", "/api/v1/users/123", 200, "updated"},
		{"DELETE", "/api/v1/users/123", 200, "deleted"},
		{"GET", "/api/v1/users/123/profile", 200, "test"},
		{"PUT", "/api/v1/users/123/profile", 200, "updated"},
		{"GET", "/api/v1/users/123/settings", 200, "123"},
		{"PUT", "/api/v1/users/123/settings", 200, "settings_updated"},
		{"POST", "/api/v1/users/123/avatar", 200, "123"},
		{"GET", "/api/v1/users/123/followers", 200, "follower1"},
		{"GET", "/api/v1/users/123/following", 200, "following1"},
		{"POST", "/api/v1/users/123/follow", 200, "followed"},
		{"DELETE", "/api/v1/users/123/follow", 200, "unfollowed"},
		{"GET", "/api/v1/users/search", 200, "search_result"},

		{"GET", "/api/v1/posts", 200, "post1"},
		{"POST", "/api/v1/posts", 201, "id"},
		{"GET", "/api/v1/posts/456", 200, "456"},
		{"PUT", "/api/v1/posts/456", 200, "456"},
		{"DELETE", "/api/v1/posts/456", 200, "456"},
		{"GET", "/api/v1/posts/456/comments", 200, "comment1"},
		{"POST", "/api/v1/posts/456/comments", 201, "post_id"},
		{"POST", "/api/v1/posts/456/like", 200, "liked"},
		{"DELETE", "/api/v1/posts/456/like", 200, "unliked"},
		{"GET", "/api/v1/posts/456/shares", 200, "share1"},
		{"POST", "/api/v1/posts/456/share", 200, "shared"},
		{"GET", "/api/v1/posts/tags/go", 200, "go"},

		{"GET", "/api/v1/comments/789", 200, "789"},
		{"PUT", "/api/v1/comments/789", 200, "789"},
		{"DELETE", "/api/v1/comments/789", 200, "789"},
		{"POST", "/api/v1/comments/789/reply", 201, "parent_id"},
		{"POST", "/api/v1/comments/789/like", 200, "liked"},
		{"DELETE", "/api/v1/comments/789/like", 200, "unliked"},
		{"GET", "/api/v1/comments/789/replies", 200, "reply1"},
		{"POST", "/api/v1/comments/batch", 200, "batch_created"},

		{"GET", "/api/v1/admin/users?token=admin-secret", 200, "admin_user1"},
		{"DELETE", "/api/v1/admin/users/1?token=admin-secret", 200, "1"},
		{"GET", "/api/v1/admin/stats?token=admin-secret", 200, "users"},
		{"POST", "/api/v1/admin/broadcast?token=admin-secret", 200, "sent"},
		{"GET", "/api/v1/admin/logs?token=admin-secret", 200, "log1"},
		{"DELETE", "/api/v1/admin/logs/1?token=admin-secret", 200, "1"},
		{"POST", "/api/v1/admin/config?token=admin-secret", 200, "updated"},
		{"GET", "/api/v1/admin/config?token=admin-secret", 200, "value"},
		{"POST", "/api/v1/admin/cache/clear?token=admin-secret", 200, "cleared"},
		{"GET", "/api/v1/admin/cache/stats?token=admin-secret", 200, "hits"},

		{"GET", "/api/v1/search?q=test", 200, "result1"},
		{"GET", "/api/v1/search/users?q=test", 200, "user_result"},
		{"GET", "/api/v1/search/posts?q=test", 200, "post_result"},
		{"GET", "/api/v1/search/comments?q=test", 200, "comment_result"},
		{"GET", "/api/v1/search/tags?q=test", 200, "tag_result"},

		{"GET", "/api/v1/notifications", 200, "notif1"},
		{"GET", "/api/v1/notifications/1", 200, "1"},
		{"PUT", "/api/v1/notifications/1/read", 200, "1"},
		{"DELETE", "/api/v1/notifications/1", 200, "1"},
		{"POST", "/api/v1/notifications/read-all", 200, "all_read"},

		{"GET", "/api/v1/messages", 200, "msg1"},
		{"POST", "/api/v1/messages", 201, "id"},
		{"GET", "/api/v1/messages/1", 200, "1"},
		{"DELETE", "/api/v1/messages/1", 200, "1"},
		{"GET", "/api/v1/messages/conversations/user1", 200, "user1"},
		{"POST", "/api/v1/messages/1/read", 200, "read"},

		{"GET", "/files/docs/readme.md", 200, "docs/readme.md"},
		{"GET", "/docs/api/v1", 200, "api/v1"},
		{"GET", "/assets/images/logo.png", 200, "images/logo.png"},

		{"GET", "/static/health.txt", 200, "ok"},
		{"GET", "/robots.txt", 200, "User-agent"},
		{"GET", "/manifest.json", 200, "http233"},

		{"GET", "/nonexistent", 404, ""},
		{"POST", "/api/v1/users/123", 405, ""},
		{"DELETE", "/api/v1/auth/login", 405, ""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.method, tt.path), func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("%s %s: status = %d, want %d body=%q", tt.method, tt.path, w.Code, tt.expectedCode, w.Body.String())
			}
			if tt.bodyContains != "" && !strings.Contains(w.Body.String(), tt.bodyContains) {
				t.Errorf("%s %s: body = %q, want containing %q", tt.method, tt.path, w.Body.String(), tt.bodyContains)
			}
		})
	}
}

func TestMiddlewareExecutionOrder(t *testing.T) {
	r := New()
	var order []string

	r.Use(func(c *Context) {
		order = append(order, "global-before")
		c.Next()
		order = append(order, "global-after")
	})

	api := r.Group("/api")
	api.Use(func(c *Context) {
		order = append(order, "group-before")
		c.Next()
		order = append(order, "group-after")
	})

	api.GET("/test", func(c *Context) {
		order = append(order, "handler")
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/test", nil)
	r.ServeHTTP(w, req)

	expected := []string{"global-before", "group-before", "handler", "group-after", "global-after"}
	if len(order) != len(expected) {
		t.Fatalf("order = %v, want %v", order, expected)
	}
	for i, v := range order {
		if v != expected[i] {
			t.Errorf("order[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestAdminMiddlewareBlock(t *testing.T) {
	r := setupBusinessAPI()

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/users", nil))
	if w.Code != 403 {
		t.Errorf("without token: status = %d, want 403", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/users?token=wrong", nil))
	if w.Code != 403 {
		t.Errorf("wrong token: status = %d, want 403", w.Code)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/users?token=admin-secret", nil))
	if w.Code != 200 {
		t.Errorf("correct token: status = %d, want 200", w.Code)
	}
}

func TestBusinessAPIConcurrent(t *testing.T) {
	r := setupBusinessAPI()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
			if w.Code != 200 {
				t.Errorf("goroutine %d: status = %d", id, w.Code)
			}
		}(i)
	}
	wg.Wait()
}

func TestRecoveryMiddleware(t *testing.T) {
	r := New()
	r.Use(Recovery())
	r.GET("/panic", func(c *Context) { panic("test panic") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/panic", nil))
	if w.Code != 500 {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestRouteGroupIsolation(t *testing.T) {
	r := New()
	v1 := r.Group("/api/v1")
	v2 := r.Group("/api/v2")
	v1.GET("/users", func(c *Context) { c.String(200, "v1") })
	v2.GET("/users", func(c *Context) { c.String(200, "v2") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/users", nil))
	if w.Body.String() != "v1" {
		t.Errorf("v1 body = %q", w.Body.String())
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v2/users", nil))
	if w.Body.String() != "v2" {
		t.Errorf("v2 body = %q", w.Body.String())
	}
}

func TestIntegrationMethodNotAllowed(t *testing.T) {
	r := New()
	r.HandleMethodNotAllowed()
	r.GET("/test", func(c *Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/test", nil))
	if w.Code != 405 {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestContextQueryParams(t *testing.T) {
	r := New()
	r.GET("/search", func(c *Context) {
		c.String(200, "q=%s&page=%s", c.Query("q"), c.DefaultQuery("page", "1"))
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/search?q=hello&page=5", nil))
	if w.Body.String() != "q=hello&page=5" {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestContextKeyValue(t *testing.T) {
	r := New()
	r.Use(func(c *Context) {
		c.Set("user_id", "12345")
		c.Next()
	})
	r.GET("/test", func(c *Context) {
		v, ok := c.Get("user_id")
		if !ok || v != "12345" {
			t.Errorf("user_id = %v", v)
		}
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
	if w.Code != 200 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestContextAbort(t *testing.T) {
	r := New()
	r.Use(func(c *Context) { c.AbortWithStatus(401) })
	r.GET("/test", func(c *Context) { c.String(200, "should not reach") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestTrailingSlashRedirect(t *testing.T) {
	r := New()
	r.RedirectTrailingSlash(true)
	r.GET("/users/", func(c *Context) { c.String(200, "users") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/users", nil))
	if w.Code != 301 {
		t.Errorf("status = %d, want 301", w.Code)
	}
}

func TestANYMethod(t *testing.T) {
	r := New()
	r.ANY("/resource", func(c *Context) { c.String(200, "ok") })

	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/resource", nil))
		if w.Code != 200 {
			t.Errorf("%s: status = %d", method, w.Code)
		}
	}
}
