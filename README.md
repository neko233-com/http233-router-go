# http233-router-go

High-performance HTTP router for Go with a Gin-like API. Optimized radix-tree matching with middleware chains, route groups, and `net/http` compatibility.

**Documentation:** [https://neko233-com.github.io/http233-router-go/](https://neko233-com.github.io/http233-router-go/)

## Install

```bash
go get github.com/neko233-com/http233-router-go
```

## Quick Start

```go
package main

import (
	"net/http"

	http233 "github.com/neko233-com/http233-router-go"
)

func main() {
	r := http233.New()
	r.Use(http233.Recovery())

	r.GET("/", func(c *http233.Context) {
		c.String(http.StatusOK, "Hello, World!")
	})

	r.GET("/users/:id", func(c *http233.Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	})

	api := r.Group("/api/v1")
	api.GET("/health", func(c *http233.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	http.ListenAndServe(":8080", r.Handler())
}
```

## Features

- Radix-tree routing (static, `:param`, `*wildcard`)
- Middleware chain (`Use`, `Next`, `Abort`, `Recovery`)
- Route groups with group-level middleware
- All standard HTTP methods + `ANY()`
- 405 Method Not Allowed / trailing slash redirect
- Context helpers: Query, BindJSON, Redirect, File, Static, HTML, JSON
- Static file serving (`Static`, `StaticFS`, `StaticFile`)
- Optional hot reload via `fsnotify`
- `sync.Pool` context reuse on the hot path

## Performance

Benchmarks use direct TCP sockets (includes full HTTP stack). On typical dev hardware, http233 matches or exceeds [httprouter](https://github.com/julienschmidt/httprouter) on QPS while providing a richer API.

```bash
go test -bench=. -benchmem ./...
go test -run TestFasterThanHttpRouter -v ./...
```

## License

MIT — see [LICENSE](LICENSE).
