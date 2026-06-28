package http233

import (
	"net/http"
	"strings"
	"sync"
)

// HandlerFunc is the handler and middleware function type.
type HandlerFunc func(*Context)

var httpMethods = []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH"}

// Router is a high-performance HTTP router compatible with net/http.
type Router struct {
	root                    node
	pool                    sync.Pool
	maxParams               uint8
	middlewares             []HandlerFunc
	onError                 func(*Context, error)
	redirectTrailingSlash   bool
	handleMethodNotAllowed  bool
	notFoundHandler         HandlerFunc
	noMethodHandler         HandlerFunc
	hr                      hotReload
}

// New creates a new Router instance.
func New() *Router {
	r := &Router{
		maxParams: 16,
	}
	r.pool = sync.Pool{
		New: func() interface{} {
			return &Context{
				params:      make([]param, 0, r.maxParams),
				errors:      make([]*RouteError, 0, 4),
				handlersBuf: make([]HandlerFunc, 0, 32),
			}
		},
	}
	return r
}

// Use registers global middleware.
func (r *Router) Use(middlewares ...HandlerFunc) {
	r.middlewares = append(r.middlewares, middlewares...)
}

// Group creates a route group with a shared prefix and middleware.
func (r *Router) Group(prefix string, middlewares ...HandlerFunc) *RouterGroup {
	return &RouterGroup{
		router:      r,
		prefix:      prefix,
		middlewares: append([]HandlerFunc(nil), middlewares...),
	}
}

func (r *Router) register(method, path string, handler HandlerFunc, mws []HandlerFunc) {
	r.addRoute(method, path, handler, mws)
}

func (r *Router) GET(path string, handler HandlerFunc)    { r.register("GET", path, handler, nil) }
func (r *Router) POST(path string, handler HandlerFunc)   { r.register("POST", path, handler, nil) }
func (r *Router) PUT(path string, handler HandlerFunc)    { r.register("PUT", path, handler, nil) }
func (r *Router) DELETE(path string, handler HandlerFunc) { r.register("DELETE", path, handler, nil) }
func (r *Router) HEAD(path string, handler HandlerFunc)   { r.register("HEAD", path, handler, nil) }
func (r *Router) OPTIONS(path string, handler HandlerFunc) {
	r.register("OPTIONS", path, handler, nil)
}
func (r *Router) PATCH(path string, handler HandlerFunc) { r.register("PATCH", path, handler, nil) }

// ANY registers a handler for all standard HTTP methods.
func (r *Router) ANY(path string, handler HandlerFunc) {
	for _, method := range httpMethods {
		r.register(method, path, handler, nil)
	}
}

// SetNotFoundHandler sets a custom 404 handler.
func (r *Router) SetNotFoundHandler(handler HandlerFunc) {
	r.notFoundHandler = handler
}

// SetNoMethodHandler sets a custom 405 handler.
func (r *Router) SetNoMethodHandler(handler HandlerFunc) {
	r.noMethodHandler = handler
}

// SetOnError sets a custom error handler invoked after the chain completes.
func (r *Router) SetOnError(handler func(*Context, error)) {
	r.onError = handler
}

// HandleMethodNotAllowed enables 405 responses when a path exists but the method does not.
func (r *Router) HandleMethodNotAllowed() {
	r.handleMethodNotAllowed = true
}

// RedirectTrailingSlash enables automatic trailing slash redirects.
func (r *Router) RedirectTrailingSlash(redirect bool) {
	r.redirectTrailingSlash = redirect
}

// Handler returns an http.Handler for the router.
func (r *Router) Handler() http.Handler {
	return http.HandlerFunc(r.ServeHTTP)
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := r.pool.Get().(*Context)
	ctx.Reset(w, req)

	path := req.URL.Path
	node, ok := r.findRoute(path, &ctx.params)
	if node == nil || !ok {
		if r.redirectTrailingSlash {
			if alt, found := r.trailingSlashPath(path); found {
				target := alt
				if req.URL.RawQuery != "" {
					target += "?" + req.URL.RawQuery
				}
				http.Redirect(w, req, target, http.StatusMovedPermanently)
				r.pool.Put(ctx)
				return
			}
		}
		r.serveNotFound(ctx)
		r.pool.Put(ctx)
		return
	}

	entry := node.getRouteEntry(req.Method)
	if entry.handler == nil {
		if r.handleMethodNotAllowed && node.hasAnyHandler() {
			r.serveNoMethod(ctx)
		} else {
			r.serveNotFound(ctx)
		}
		r.pool.Put(ctx)
		return
	}

	r.runHandlers(ctx, entry)
	r.finalize(ctx)
	r.pool.Put(ctx)
}

func (r *Router) runHandlers(ctx *Context, entry routeEntry) {
	if len(r.middlewares) == 0 {
		if entry.chain != nil {
			ctx.handlers = entry.chain
			ctx.index = -1
			ctx.Next()
			return
		}
		entry.handler(ctx)
		return
	}

	ctx.handlers = ctx.handlersBuf[:0]
	ctx.handlers = append(ctx.handlers, r.middlewares...)
	if entry.chain != nil {
		ctx.handlers = append(ctx.handlers, entry.chain...)
	} else {
		ctx.handlers = append(ctx.handlers, entry.handler)
	}
	ctx.index = -1
	ctx.Next()
}

func (r *Router) finalize(ctx *Context) {
	if r.onError != nil {
		for _, e := range ctx.errors {
			r.onError(ctx, e.Err)
		}
	}
}

func (r *Router) serveNotFound(ctx *Context) {
	if r.notFoundHandler != nil {
		r.notFoundHandler(ctx)
		return
	}
	http.NotFound(ctx.Response, ctx.Request)
}

func (r *Router) serveNoMethod(ctx *Context) {
	if r.noMethodHandler != nil {
		r.noMethodHandler(ctx)
		return
	}
	ctx.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	ctx.Response.WriteHeader(http.StatusMethodNotAllowed)
	ctx.Response.Write([]byte("Method Not Allowed"))
}

func (r *Router) trailingSlashPath(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	if strings.HasSuffix(path, "/") {
		trimmed := strings.TrimSuffix(path, "/")
		if trimmed == "" {
			trimmed = "/"
		}
		node, ok := r.findRoute(trimmed, nil)
		if node != nil && ok && node.hasAnyHandler() {
			return trimmed, true
		}
		return "", false
	}
	node, ok := r.findRoute(path+"/", nil)
	if node != nil && ok && node.hasAnyHandler() {
		return path + "/", true
	}
	return "", false
}

func (r *Router) findRoute(path string, params *[]param) (*node, bool) {
	current := &r.root
	i := 0

	if len(current.prefix) > 0 {
		prefixLen := current.findLongestPrefix(path)
		if int(prefixLen) != len(current.prefix) {
			return nil, false
		}
		i += int(prefixLen)
	}

	for i <= len(path) {
		if i == len(path) {
			if current.hasAnyHandler() {
				return current, true
			}
			return current, false
		}

		if next, ok := r.walkSegment(current, path, &i, params); !ok {
			return nil, false
		} else {
			current = next
		}
	}

	if current.hasAnyHandler() {
		return current, true
	}
	return current, false
}

func (r *Router) walkSegment(current *node, path string, i *int, params *[]param) (*node, bool) {
	if child, ok := r.tryStaticChildren(current, path, i); ok {
		return child, true
	}
	if child, ok := r.tryParamChild(current, path, i, params); ok {
		return child, true
	}
	return r.tryWildcardChild(current, path, i, params)
}

func (r *Router) tryStaticChildren(current *node, path string, i *int) (*node, bool) {
	next := path[*i]
	for j := uint8(0); j < current.childCount; j++ {
		child := current.children[j]
		if child.nType != nodeStatic {
			continue
		}
		if len(child.prefix) > 0 && child.prefix[0] != next {
			continue
		}
		prefixLen := child.findLongestPrefix(path[*i:])
		if prefixLen > 0 && int(prefixLen) == len(child.prefix) {
			*i += int(prefixLen)
			return child, true
		}
	}
	for _, child := range current.overflow {
		if child.nType != nodeStatic {
			continue
		}
		if len(child.prefix) > 0 && child.prefix[0] != next {
			continue
		}
		prefixLen := child.findLongestPrefix(path[*i:])
		if prefixLen > 0 && int(prefixLen) == len(child.prefix) {
			*i += int(prefixLen)
			return child, true
		}
	}
	return nil, false
}

func (r *Router) tryParamChild(current *node, path string, i *int, params *[]param) (*node, bool) {
	for j := uint8(0); j < current.childCount; j++ {
		if child, ok := r.matchParamChild(current.children[j], path, i, params); ok {
			return child, true
		}
	}
	for _, child := range current.overflow {
		if child, ok := r.matchParamChild(child, path, i, params); ok {
			return child, true
		}
	}
	return nil, false
}

func (r *Router) matchParamChild(child *node, path string, i *int, params *[]param) (*node, bool) {
	if child.nType != nodeParam {
		return nil, false
	}
	start := *i
	for *i < len(path) && path[*i] != '/' {
		*i++
	}
	if *i == start {
		return nil, false
	}
	if params != nil {
		*params = append(*params, param{key: child.paramName, value: path[start:*i]})
	}
	return child, true
}

func (r *Router) tryWildcardChild(current *node, path string, i *int, params *[]param) (*node, bool) {
	for j := uint8(0); j < current.childCount; j++ {
		if child, ok := r.matchWildcardChild(current.children[j], path, i, params); ok {
			return child, true
		}
	}
	for _, child := range current.overflow {
		if child, ok := r.matchWildcardChild(child, path, i, params); ok {
			return child, true
		}
	}
	return nil, false
}

func (r *Router) matchWildcardChild(child *node, path string, i *int, params *[]param) (*node, bool) {
	if child.nType != nodeWildcard {
		return nil, false
	}
	if params != nil {
		value := path[*i:]
		if len(value) > 0 && value[0] == '/' {
			value = value[1:]
		}
		*params = append(*params, param{key: child.paramName, value: value})
	}
	*i = len(path)
	return child, true
}

