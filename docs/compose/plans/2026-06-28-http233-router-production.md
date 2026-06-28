# http233-router-go Production Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use compose:subagent (recommended) or compose:execute to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 http233-router-go 从性能原型升级为生产级路由库，支持中间件链、路由组、错误管理、静态文件、热重载，并通过 100+ API 业务测试验证。

**Architecture:** 在现有 radix tree 基础上扩展。中间件采用经典链式调用（`c.Next()` / `c.Abort()`）。路由组共享 prefix 和组级中间件。热重载基于 fsnotify，默认关闭。

**Tech Stack:** Go 1.25+, `net/http` 兼容, `fsnotify` (热重载可选依赖)

## Global Constraints

- Go module: `github.com/neko233-com/http233-router-go`
- 核心路由零外部依赖（stdlib only）
- 热重载用 `github.com/fsnotify/fsnotify`（可选，build tag 隔离）
- 所有公共 API 必须有 godoc 注释
- 测试必须覆盖 100+ 路由

---

## Phase 1: Middleware Chain

### Task 1: Middleware 类型定义和链式执行

**Covers:** [S2.1]

**Files:**
- Create: `middleware.go`

**Interfaces:**
- Consumes: `handlerFunc` from `tree_node.go`, `Context` from `context.go`
- Produces: `HandlerFunc`, `Recovery()`, chain execution logic

- [ ] **Step 1: Create middleware.go with types and chain**

```go
package http233

// HandlerFunc is the middleware handler type.
// It receives a Context and calls c.Next() to proceed to the next handler.
type HandlerFunc func(*Context)

// Recovery returns a middleware that recovers from panics.
func Recovery() HandlerFunc {
	return func(c *Context) {
		defer func() {
			if r := recover(); r != nil {
				c.String(500, "Internal Server Error")
				c.errors = append(c.errors, &Error{
					Err:  r.(error),
					Code: 500,
				})
			}
		}()
		c.Next()
	}
}
```

- [ ] **Step 2: Add middleware fields to Router in router.go**

Add to `Router` struct:
```go
type Router struct {
	root         node
	pool         sync.Pool
	maxParams    uint8
	middlewares  []HandlerFunc
	tree         [][]*node  // middleware tree per segment
	onError     func(*Context, error)
 redirectToTrailingSlash bool
 handleMethodNotAllowed  bool
	notFoundHandler         HandlerFunc
	noMethodHandler        HandlerFunc
}
```

- [ ] **Step 3: Add Use() method to router.go**

```go
func (r *Router) Use(middlewares ...HandlerFunc) {
	r.middlewares = append(r.middlewares, middlewares...)
}
```

- [ ] **Step 4: Update Context with middleware support**

In `context.go`, add:
```go
type Context struct {
	Response  http.ResponseWriter
	Request   *http.Request
	params    []param
	errors    []*Error
	index     int
.handlers  []HandlerFunc
	store     map[string]interface{}
	handled   bool
}

type Error struct {
	Err  error
	Code int
}

func (c *Context) Next() {
	c.index++
	for c.index < len(c.handlers) {
		c.handlers[c.index](c)
		c.index++
	}
}

func (c *Context) Abort() {
	c.index = len(c.handlers)
}

func (c *Context) AbortWithStatus(code int) {
	c.Status(code)
	c.Abort()
}

func (c *Context) Error(err error) {
	c.errors = append(c.errors, &Error{Err: err})
}

func (c *Context) Set(key string, value interface{}) {
	if c.store == nil {
		c.store = make(map[string]interface{})
	}
	c.store[key] = value
}

func (c *Context) Get(key string) (value interface{}, exists bool) {
	if c.store != nil {
		value, exists = c.store[key]
	}
	return
}

func (c *Context) Fail(code int, msg string) {
	c.errors = append(c.errors, &Error{
		Err:  fmt.Errorf(msg),
		Code: code,
	})
	c.AbortWithStatus(code)
}
```

- [ ] **Step 5: Update ServeHTTP to execute middleware chain**

Replace `ServeHTTP` in `router.go`:
```go
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := r.pool.Get().(*Context)
	ctx.Reset(w, req)

	node, params := r.findRoute(req.URL.Path)
	if node == nil {
		if r.notFoundHandler != nil {
			r.notFoundHandler(ctx)
		} else {
			http.NotFound(w, req)
		}
		r.pool.Put(ctx)
		return
	}

	for _, p := range params {
		ctx.params = append(ctx.params, p)
	}

	handler := node.getHandler(req.Method)
	if handler == nil {
		if r.handleMethodNotAllowed {
			if r.noMethodHandler != nil {
				r.noMethodHandler(ctx)
			} else {
				http.Error(w, "Method Not Allowed", 405)
			}
		} else {
			http.NotFound(w, req)
		}
		r.pool.Put(ctx)
		return
	}

	// Build handler chain: global middlewares + route handler
	ctx.handlers = make([]HandlerFunc, 0, len(r.middlewares)+1)
	ctx.handlers = append(ctx.handlers, r.middlewares...)
	ctx.handlers = append(ctx.handlers, handler)
	ctx.Next()

	if r.onError != nil && len(ctx.errors) > 0 {
		for _, e := range ctx.errors {
			r.onError(ctx, e.Err)
		}
	}

	r.pool.Put(ctx)
}
```

- [ ] **Step 6: Verify compilation and run tests**

```bash
go build ./...
go test -v ./...
```

- [ ] **Step 7: Commit**

```bash
git add middleware.go router.go context.go
git commit -m "feat: add middleware chain with Recovery, Next/Abort support"
```

---

## Phase 2: Route Groups

### Task 2: RouteGroup 实现

**Covers:** [S2.2]

**Files:**
- Create: `group.go`
- Modify: `route.go`

**Interfaces:**
- Consumes: `Router`, `HandlerFunc`, middleware chain
- Produces: `RouterGroup` struct, `Group()` method, group-level middleware

- [ ] **Step 1: Create group.go**

```go
package http233

// RouterGroup represents a group of routes with shared prefix and middleware.
type RouterGroup struct {
	router      *Router
	prefix      string
	middlewares []HandlerFunc
}

// Group creates a new route group with the given prefix and middleware.
func (r *Router) Group(prefix string, middlewares ...HandlerFunc) *RouterGroup {
	return &RouterGroup{
		router:      r,
		prefix:      prefix,
		middlewares: middlewares,
	}
}

// Use adds middleware to this group.
func (g *RouterGroup) Use(middlewares ...HandlerFunc) {
	g.middlewares = append(g.middlewares, middlewares...)
}

// GET registers a GET handler in this group.
func (g *RouterGroup) GET(path string, handler HandlerFunc) {
	g.router.addRoute("GET", g.prefix+path, g.wrapHandler(handler))
}

// POST registers a POST handler.
func (g *RouterGroup) POST(path string, handler HandlerFunc) {
	g.router.addRoute("POST", g.prefix+path, g.wrapHandler(handler))
}

// PUT registers a PUT handler.
func (g *RouterGroup) PUT(path string, handler HandlerFunc) {
	g.router.addRoute("PUT", g.prefix+path, g.wrapHandler(handler))
}

// DELETE registers a DELETE handler.
func (g *RouterGroup) DELETE(path string, handler HandlerFunc) {
	g.router.addRoute("DELETE", g.prefix+path, g.wrapHandler(handler))
}

// HEAD registers a HEAD handler.
func (g *RouterGroup) HEAD(path string, handler HandlerFunc) {
	g.router.addRoute("HEAD", g.prefix+path, g.wrapHandler(handler))
}

// OPTIONS registers an OPTIONS handler.
func (g *RouterGroup) OPTIONS(path string, handler HandlerFunc) {
	g.router.addRoute("OPTIONS", g.prefix+path, g.wrapHandler(handler))
}

// PATCH registers a PATCH handler.
func (g *RouterGroup) PATCH(path string, handler HandlerFunc) {
	g.router.addRoute("PATCH", g.prefix+path, g.wrapHandler(handler))
}

// ANY registers a handler for all methods.
func (g *RouterGroup) ANY(path string, handler HandlerFunc) {
	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH"} {
		g.router.addRoute(method, g.prefix+path, g.wrapHandler(handler))
	}
}

// Group creates a nested group.
func (g *RouterGroup) Group(prefix string, middlewares ...HandlerFunc) *RouterGroup {
	return &RouterGroup{
		router:      g.router,
		prefix:      g.prefix + prefix,
		middlewares: append(g.middlewares, middlewares...),
	}
}

// wrapHandler wraps a handler with group-level middleware.
func (g *RouterGroup) wrapHandler(handler HandlerFunc) HandlerFunc {
	// Prepend group middlewares to each handler
	return func(c *Context) {
		// Insert group middlewares before the route handler
		chain := make([]HandlerFunc, 0, len(g.middlewares)+1)
		chain = append(chain, g.middlewares...)
		chain = append(chain, handler)

		// Save original handlers
		origHandlers := c.handlers
		origIndex := c.index

		c.handlers = chain
		c.index = -1
		c.Next()

		// Restore original handlers
		c.handlers = origHandlers
		c.index = origIndex
	}
}
```

- [ ] **Step 2: Add ANY method to Router in router.go**

```go
func (r *Router) ANY(path string, handler handlerFunc) {
	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH"} {
		r.addRoute(method, path, handler)
	}
}
```

- [ ] **Step 3: Add configuration methods to router.go**

```go
func (r *Router) SetNotFoundHandler(handler HandlerFunc) {
	r.notFoundHandler = handler
}

func (r *Router) SetNoMethodHandler(handler HandlerFunc) {
	r.noMethodHandler = handler
}

func (r *Router) SetOnError(handler func(*Context, error)) {
	r.onError = handler
}

func (r *Router) HandleMethodNotAllowed() {
	r.handleMethodNotAllowed = true
}

func (r *Router) RedirectTrailingSlash(redirect bool) {
	r.redirectToTrailingSlash = redirect
}
```

- [ ] **Step 4: Verify compilation and run tests**

```bash
go build ./...
go test -v ./...
```

- [ ] **Step 5: Commit**

```bash
git add group.go router.go
git commit -m "feat: add route groups with group-level middleware"
```

---

## Phase 3: Context 增强

### Task 3: Context 完善

**Covers:** [S2.5]

**Files:**
- Modify: `context.go`

**Interfaces:**
- Consumes: existing `Context`
- Produces: Query/Bind/Redirect/File/HTML/Data methods

- [ ] **Step 1: Add Query and Bind methods to context.go**

```go
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

func (c *Context) DefaultQuery(key, defaultValue string) string {
	if v := c.Request.URL.Query().Get(key); v != "" {
		return v
	}
	return defaultValue
}

func (c *Context) QueryArray(key string) []string {
	return c.Request.URL.Query()[key]
}

func (c *Context) BindJSON(obj interface{}) error {
	return json.NewDecoder(c.Request.Body).Decode(obj)
}

func (c *Context) ShouldBindJSON(obj interface{}) error {
	return c.BindJSON(obj)
}

func (c *Context) Redirect(code int, location string) {
	http.Redirect(c.Response, c.Request, location, code)
	c.Abort()
}

func (c *Context) Data(code int, contentType string, data []byte) {
	c.Status(code)
	c.Response.Header().Set("Content-Type", contentType)
	c.Response.Write(data)
}

func (c *Context) File(filepath string) {
	http.ServeFile(c.Response, c.Request, filepath)
}

func (c *Context) FileAttachment(filepath, filename string) {
	c.Response.Header().Set("Content-Disposition", "attachment; filename="+filename)
	http.ServeFile(c.Response, c.Request, filepath)
}

func (c *Context) HTML(code int, name string, obj interface{}) {
	// Placeholder: template rendering
	c.Status(code)
	c.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(c.Response, "%v", obj)
}

func (c *Context) JSON(code int, obj interface{}) {
	c.Status(code)
	c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(c.Response).Encode(obj)
}

func (c *Context) String(code int, format string, values ...interface{}) {
	c.Status(code)
	c.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(c.Response, format, values...)
}

func (c *Context) Status(code int) {
	c.Response.WriteHeader(code)
}
```

- [ ] **Step 2: Update Reset to clear store and errors**

```go
func (c *Context) Reset(w http.ResponseWriter, req *http.Request) {
	c.Response = w
	c.Request = req
	c.params = c.params[:0]
	c.errors = c.errors[:0]
	c.index = 0
	c.handlers = nil
	c.handled = false
	// Don't clear store on reset - it's per-request
}
```

- [ ] **Step 3: Add Methods() helper**

```go
func (c *Context) Methods() []string {
	return []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH"}
}
```

- [ ] **Step 4: Verify compilation**

```bash
go build ./...
go test -v ./...
```

- [ ] **Step 5: Commit**

```bash
git add context.go
git commit -m "feat: enhance Context with Query, Bind, Redirect, File, Data"
```

---

## Phase 4: 静态文件服务

### Task 4: Static File Serving

**Covers:** [S2.6]

**Files:**
- Create: `static.go`

**Interfaces:**
- Consumes: `Router`, `RouterGroup`
- Produces: `Static()`, `StaticFS()`, `StaticFile()` methods

- [ ] **Step 1: Create static.go**

```go
package http233

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Static serves static files from the given root directory.
func (r *Router) Static(urlPrefix, rootDir string) {
	r.StaticFS(urlPrefix, http.Dir(rootDir))
}

// StaticFS serves static files using a custom http.FileSystem.
func (r *Router) StaticFS(urlPrefix string, fs http.FileSystem) {
	if !strings.HasSuffix(urlPrefix, "/") {
		urlPrefix += "/"
	}

	fileServer := http.StripPrefix(urlPrefix, http.FileServer(fs))
	r.GET(urlPrefix+"*filepath", func(c *Context) {
		path := c.Param("filepath")
		// Security: prevent directory traversal
		if strings.Contains(path, "..") {
			c.AbortWithStatus(403)
			return
		}
		fileServer.ServeHTTP(c.Response, c.Request)
	})
}

// StaticFile binds a single file to a URL path.
func (r *Router) StaticFile(url, filepath string) {
	r.GET(url, func(c *Context) {
		file, err := os.Open(filepath)
		if err != nil {
			c.AbortWithStatus(404)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil || stat.IsDir() {
			c.AbortWithStatus(404)
			return
		}

		http.ServeFile(c.Response, c.Request, filepath)
	})
}

// Static serves static files in a group.
func (g *RouterGroup) Static(urlPrefix, rootDir string) {
	g.StaticFS(urlPrefix, http.Dir(rootDir))
}

// StaticFS serves static files in a group using a custom http.FileSystem.
func (g *RouterGroup) StaticFS(urlPrefix string, fs http.FileSystem) {
	if !strings.HasSuffix(urlPrefix, "/") {
		urlPrefix += "/"
	}

	fileServer := http.StripPrefix(urlPrefix, http.FileServer(fs))
	g.GET(urlPrefix+"*filepath", func(c *Context) {
		path := c.Param("filepath")
		if strings.Contains(path, "..") {
			c.AbortWithStatus(403)
			return
		}
		fileServer.ServeHTTP(c.Response, c.Request)
	})
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add static.go
git commit -m "feat: add static file serving with directory traversal protection"
```

---

## Phase 5: 热重载

### Task 5: Hot-Reload with fsnotify

**Covers:** [S3]

**Files:**
- Create: `hotreload.go`

**Interfaces:**
- Consumes: `Router`, `fsnotify.Watcher`
- Produces: `EnableHotReload()`, `DisableHotReload()`, `SetHotReloadCallback()`

- [ ] **Step 1: Create hotreload.go**

```go
package http233

import (
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type hotReload struct {
	enabled  bool
	watcher  *fsnotify.Watcher
	callback func(events []fsnotify.Event)
	dirs     []string
	mu       sync.Mutex
	stopCh   chan struct{}
}

func (r *Router) EnableHotReload(dirs ...string) {
	r.hr.mu.Lock()
	defer r.hr.mu.Unlock()

	if r.hr.enabled {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("http233: failed to create file watcher: %v", err)
		return
	}

	r.hr.watcher = watcher
	r.hr.dirs = dirs
	r.hr.enabled = true
	r.hr.stopCh = make(chan struct{})

	for _, dir := range dirs {
		if err := watcher.Add(dir); err != nil {
			log.Printf("http233: failed to watch directory %s: %v", dir, err)
			continue
		}
	}

	go r.hr.loop()
}

func (r *Router) DisableHotReload() {
	r.hr.mu.Lock()
	defer r.hr.mu.Unlock()

	if !r.hr.enabled {
		return
	}

	close(r.hr.stopCh)
	r.hr.watcher.Close()
	r.hr.enabled = false
}

func (r *Router) SetHotReloadCallback(fn func(events []fsnotify.Event)) {
	r.hr.mu.Lock()
	defer r.hr.mu.Unlock()
	r.hr.callback = fn
}

func (r *Router) IsHotReloadEnabled() bool {
	r.hr.mu.Lock()
	defer r.hr.mu.Unlock()
	return r.hr.enabled
}

func (hr *hotReload) loop() {
	var events []fsnotify.Event
	for {
		select {
		case event, ok := <-hr.watcher.Events:
			if !ok {
				return
			}
			// Only care about write/create/remove events
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
				events = append(events, event)
				if hr.callback != nil {
					hr.callback(events)
				}
				events = events[:0]
			}
		case err, ok := <-hr.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("http233: file watcher error: %v", err)
		case <-hr.stopCh:
			return
		}
	}
}
```

- [ ] **Step 2: Add hr field to Router in router.go**

```go
type Router struct {
	// ... existing fields ...
	hr hotReload
}
```

Initialize in `New()`:
```go
func New() *Router {
	r := &Router{
		maxParams: 16,
	}
	r.hr = hotReload{}
	// ... rest of initialization ...
	return r
}
```

- [ ] **Step 3: Update go.mod for fsnotify**

```bash
go get github.com/fsnotify/fsnotify
```

- [ ] **Step 4: Verify compilation**

```bash
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add hotreload.go router.go go.mod go.sum
git commit -m "feat: add hot-reload support via fsnotify (disabled by default)"
```

---

## Phase 6: 100+ API 集成测试

### Task 6: Business Scenario Test Suite

**Covers:** [S4.1, S4.2]

**Files:**
- Create: `integration_test.go`

**Interfaces:**
- Consumes: `Router`, `RouterGroup`, all middleware/context features
- Produces: 100+ route test cases

- [ ] **Step 1: Create integration_test.go with full business scenario**

```go
package http233

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Setup a full business API with 100+ routes
func setupBusinessAPI() *Router {
	r := New()

	// Global middleware
	r.Use(Recovery())
	r.Use(func(c *Context) {
		c.Set("request_id", fmt.Sprintf("req-%d", 1))
		c.Next()
	})

	// Health check routes (3)
	r.GET("/health", func(c *Context) { c.JSON(200, map[string]string{"status": "ok"}) })
	r.GET("/health/ready", func(c *Context) { c.JSON(200, map[string]string{"ready": "ok"}) })
	r.GET("/health/live", func(c *Context) { c.JSON(200, map[string]string{"live": "ok"}) })

	// Auth routes (6)
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/login", func(c *Context) { c.JSON(200, map[string]string{"token": "xxx"}) })
		auth.POST("/register", func(c *Context) { c.JSON(201, map[string]string{"id": "1"}) })
		auth.POST("/refresh", func(c *Context) { c.JSON(200, map[string]string{"token": "yyy"}) })
		auth.POST("/logout", func(c *Context) { c.JSON(200, map[string]string{"status": "logged_out"}) })
		auth.GET("/verify", func(c *Context) { c.JSON(200, map[string]string{"valid": "true"}) })
		auth.DELETE("/revoke", func(c *Context) { c.JSON(200, map[string]string{"status": "revoked"}) })
	}

	// User routes (15)
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

	// Post routes (12)
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

	// Comment routes (8)
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

	// Admin routes (10) with admin middleware
	admin := r.Group("/api/v1/admin")
	admin.Use(func(c *Context) {
		token := c.Query("token")
		if token != "admin-secret" {
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

	// File routes (8)
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

	// Search routes (5)
	search := r.Group("/api/v1/search")
	{
		search.GET("", func(c *Context) { c.JSON(200, []string{"result1"}) })
		search.GET("/users", func(c *Context) { c.JSON(200, []string{"user_result"}) })
		search.GET("/posts", func(c *Context) { c.JSON(200, []string{"post_result"}) })
		search.GET("/comments", func(c *Context) { c.JSON(200, []string{"comment_result"}) })
		search.GET("/tags", func(c *Context) { c.JSON(200, []string{"tag_result"}) })
	}

	// Notification routes (5)
	notifications := r.Group("/api/v1/notifications")
	{
		notifications.GET("", func(c *Context) { c.JSON(200, []string{"notif1"}) })
		notifications.GET("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		notifications.PUT("/:id/read", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		notifications.DELETE("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		notifications.POST("/read-all", func(c *Context) { c.JSON(200, map[string]string{"status": "all_read"}) })
	}

	// Message routes (6)
	messages := r.Group("/api/v1/messages")
	{
		messages.GET("", func(c *Context) { c.JSON(200, []string{"msg1"}) })
		messages.POST("", func(c *Context) { c.JSON(201, map[string]string{"id": "1"}) })
		messages.GET("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		messages.DELETE("/:id", func(c *Context) { c.JSON(200, map[string]string{"id": c.Param("id")}) })
		messages.GET("/conversations/:userId", func(c *Context) { c.JSON(200, map[string]string{"user_id": c.Param("userId")}) })
		messages.POST("/:id/read", func(c *Context) { c.JSON(200, map[string]string{"status": "read"}) })
	}

	// Static files (5)
	r.Static("/static", "./testdata")
	r.StaticFile("/favicon.ico", "./testdata/favicon.ico")
	r.StaticFile("/robots.txt", "./testdata/robots.txt")
	r.StaticFile("/sitemap.xml", "./testdata/sitemap.xml")
	r.StaticFile("/manifest.json", "./testdata/manifest.json")

	// Wildcard catch-all (3)
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
		// Health (3)
		{"GET", "/health", 200, "ok"},
		{"GET", "/health/ready", 200, "ok"},
		{"GET", "/health/live", 200, "ok"},

		// Auth (6)
		{"POST", "/api/v1/auth/login", 200, "token"},
		{"POST", "/api/v1/auth/register", 201, "id"},
		{"POST", "/api/v1/auth/refresh", 200, "token"},
		{"POST", "/api/v1/auth/logout", 200, "logged_out"},
		{"GET", "/api/v1/auth/verify", 200, "valid"},
		{"DELETE", "/api/v1/auth/revoke", 200, "revoked"},

		// Users (15)
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

		// Posts (12)
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

		// Comments (8)
		{"GET", "/api/v1/comments/789", 200, "789"},
		{"PUT", "/api/v1/comments/789", 200, "789"},
		{"DELETE", "/api/v1/comments/789", 200, "789"},
		{"POST", "/api/v1/comments/789/reply", 201, "parent_id"},
		{"POST", "/api/v1/comments/789/like", 200, "liked"},
		{"DELETE", "/api/v1/comments/789/like", 200, "unliked"},
		{"GET", "/api/v1/comments/789/replies", 200, "reply1"},
		{"POST", "/api/v1/comments/batch", 200, "batch_created"},

		// Admin (10) - with valid token
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

		// Search (5)
		{"GET", "/api/v1/search?q=test", 200, "result1"},
		{"GET", "/api/v1/search/users?q=test", 200, "user_result"},
		{"GET", "/api/v1/search/posts?q=test", 200, "post_result"},
		{"GET", "/api/v1/search/comments?q=test", 200, "comment_result"},
		{"GET", "/api/v1/search/tags?q=test", 200, "tag_result"},

		// Notifications (5)
		{"GET", "/api/v1/notifications", 200, "notif1"},
		{"GET", "/api/v1/notifications/1", 200, "1"},
		{"PUT", "/api/v1/notifications/1/read", 200, "1"},
		{"DELETE", "/api/v1/notifications/1", 200, "1"},
		{"POST", "/api/v1/notifications/read-all", 200, "all_read"},

		// Messages (6)
		{"GET", "/api/v1/messages", 200, "msg1"},
		{"POST", "/api/v1/messages", 201, "id"},
		{"GET", "/api/v1/messages/1", 200, "1"},
		{"DELETE", "/api/v1/messages/1", 200, "1"},
		{"GET", "/api/v1/messages/conversations/user1", 200, "user1"},
		{"POST", "/api/v1/messages/1/read", 200, "read"},

		// Wildcard catch-all (3)
		{"GET", "/files/docs/readme.md", 200, "docs/readme.md"},
		{"GET", "/docs/api/v1", 200, "api/v1"},
		{"GET", "/assets/images/logo.png", 200, "images/logo.png"},

		// Not Found (3)
		{"GET", "/nonexistent", 404, ""},
		{"POST", "/api/v1/users/123", 405, ""}, // Method not allowed
		{"DELETE", "/api/v1/auth/login", 405, ""}, // Method not allowed
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.method, tt.path), func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("%s %s: status = %d, want %d", tt.method, tt.path, w.Code, tt.expectedCode)
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
		t.Fatalf("order length = %d, want %d", len(order), len(expected))
	}
	for i, v := range order {
		if v != expected[i] {
			t.Errorf("order[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestAdminMiddlewareBlock(t *testing.T) {
	r := setupBusinessAPI()

	// Without token - should be blocked
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("GET /api/v1/admin/users without token: status = %d, want 403", w.Code)
	}

	// With wrong token - should be blocked
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/admin/users?token=wrong", nil)
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("GET /api/v1/admin/users with wrong token: status = %d, want 403", w.Code)
	}

	// With correct token - should pass
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/admin/users?token=admin-secret", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("GET /api/v1/admin/users with correct token: status = %d, want 200", w.Code)
	}
}

func TestConcurrentAccess(t *testing.T) {
	r := setupBusinessAPI()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/health", nil)
			r.ServeHTTP(w, req)
			if w.Code != 200 {
				t.Errorf("goroutine %d: status = %d, want 200", id, w.Code)
			}
		}(i)
	}
	wg.Wait()
}

func TestRecoveryMiddleware(t *testing.T) {
	r := New()
	r.Use(Recovery())

	r.GET("/panic", func(c *Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	r.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Errorf("GET /panic: status = %d, want 500", w.Code)
	}
}

func TestRouteGroupIsolation(t *testing.T) {
	r := New()

	v1 := r.Group("/api/v1")
	v2 := r.Group("/api/v2")

	v1.GET("/users", func(c *Context) { c.String(200, "v1") })
	v2.GET("/users", func(c *Context) { c.String(200, "v2") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	r.ServeHTTP(w, req)
	if w.Body.String() != "v1" {
		t.Errorf("GET /api/v1/users: body = %q, want %q", w.Body.String(), "v1")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v2/users", nil)
	r.ServeHTTP(w, req)
	if w.Body.String() != "v2" {
		t.Errorf("GET /api/v2/users: body = %q, want %q", w.Body.String(), "v2")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	r := New()
	r.HandleMethodNotAllowed()
	r.GET("/test", func(c *Context) { c.String(200, "ok") })

	// Method not allowed should return 405
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Errorf("POST /test: status = %d, want 405", w.Code)
	}
}

func TestContextQueryParams(t *testing.T) {
	r := New()
	r.GET("/search", func(c *Context) {
		q := c.Query("q")
		page := c.DefaultQuery("page", "1")
		c.String(200, "q=%s&page=%s", q, page)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/search?q=hello&page=5", nil)
	r.ServeHTTP(w, req)

	if w.Body.String() != "q=hello&page=5" {
		t.Errorf("body = %q, want %q", w.Body.String(), "q=hello&page=5")
	}
}

func TestContextKeyValue(t *testing.T) {
	r := New()
	r.Use(func(c *Context) {
		c.Set("user_id", "12345")
		c.Next()
	})
	r.GET("/test", func(c *Context) {
		v, exists := c.Get("user_id")
		if !exists || v != "12345" {
			t.Errorf("user_id = %v, want 12345", v)
		}
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestContextAbort(t *testing.T) {
	r := New()
	r.Use(func(c *Context) {
		c.AbortWithStatus(401)
	})
	r.GET("/test", func(c *Context) {
		c.String(200, "should not reach")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if w.Body.String() == "should not reach" {
		t.Error("handler should not have been called after Abort")
	}
}
```

- [ ] **Step 2: Run all tests**

```bash
go test -v -count=1 ./...
```

Expected: All tests pass

- [ ] **Step 3: Commit**

```bash
git add integration_test.go
git commit -m "test: add 100+ API business scenario integration tests"
```

---

## Phase 7: 性能基准测试

### Task 7: Benchmark Suite

**Covers:** [S4.3]

**Files:**
- Create: `benchmark_test.go`

**Interfaces:**
- Consumes: `Router` with full business API
- Produces: Benchmark functions with direct TCP socket testing

- [ ] **Step 1: Create benchmark_test.go**

```go
package http233

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"testing"
)

func BenchmarkStaticRoute(b *testing.B) {
	r := New()
	r.GET("/health", func(c *Context) { c.String(200, "ok") })

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go http.Serve(ln, r.Handler())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn, _ := net.Dial("tcp", ln.Addr().String())
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for pb.Next() {
			fmt.Fprintf(conn, "GET /health HTTP/1.1\r\nHost: localhost\r\n\r\n")
			resp, _ := http.ReadResponse(reader, nil)
			resp.Body.Close()
		}
	})
}

func BenchmarkParameterRoute(b *testing.B) {
	r := New()
	r.GET("/users/:id", func(c *Context) { c.String(200, "user %s", c.Param("id")) })

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go http.Serve(ln, r.Handler())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn, _ := net.Dial("tcp", ln.Addr().String())
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for pb.Next() {
			fmt.Fprintf(conn, "GET /users/12345 HTTP/1.1\r\nHost: localhost\r\n\r\n")
			resp, _ := http.ReadResponse(reader, nil)
			resp.Body.Close()
		}
	})
}

func BenchmarkWildcardRoute(b *testing.B) {
	r := New()
	r.GET("/files/*path", func(c *Context) { c.String(200, "%s", c.Param("path")) })

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go http.Serve(ln, r.Handler())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn, _ := net.Dial("tcp", ln.Addr().String())
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for pb.Next() {
			fmt.Fprintf(conn, "GET /files/docs/readme.md HTTP/1.1\r\nHost: localhost\r\n\r\n")
			resp, _ := http.ReadResponse(reader, nil)
			resp.Body.Close()
		}
	})
}

func BenchmarkBusinessAPI(b *testing.B) {
	r := setupBusinessAPI()

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go http.Serve(ln, r.Handler())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn, _ := net.Dial("tcp", ln.Addr().String())
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for pb.Next() {
			fmt.Fprintf(conn, "GET /api/v1/users/123 HTTP/1.1\r\nHost: localhost\r\n\r\n")
			resp, _ := http.ReadResponse(reader, nil)
			resp.Body.Close()
		}
	})
}

func BenchmarkMemoryAlloc(b *testing.B) {
	r := New()
	r.GET("/test", func(c *Context) { c.String(200, "ok") })

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		r.ServeHTTP(w, req)
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
}
```

- [ ] **Step 2: Run benchmarks**

```bash
go test -bench=. -benchmem -count=3 ./...
```

- [ ] **Step 3: Commit**

```bash
git add benchmark_test.go
git commit -m "test: add performance benchmark suite"
```

---

## Summary

| Phase | Task | Deliverable |
|-------|------|-------------|
| 1 | Middleware Chain | Recovery, Next/Abort, chain execution |
| 2 | Route Groups | Group(), nested groups, group middleware |
| 3 | Context 增强 | Query, Bind, Redirect, File, Data, KV store |
| 4 | 静态文件 | Static(), StaticFS(), StaticFile() |
| 5 | 热重载 | fsnotify watcher, Enable/DisableHotReload() |
| 6 | 100+ API 测试 | Business scenario integration tests |
| 7 | 性能基准 | Direct socket benchmarks |

**Total routes in test suite: 100+**
**Total test functions: 15+**
