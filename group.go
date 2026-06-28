package http233

// RouterGroup groups routes under a shared prefix and middleware chain.
type RouterGroup struct {
	router      *Router
	prefix      string
	middlewares []HandlerFunc
}

// Use adds middleware to this group.
func (g *RouterGroup) Use(middlewares ...HandlerFunc) {
	g.middlewares = append(g.middlewares, middlewares...)
}

// Group creates a nested route group.
func (g *RouterGroup) Group(prefix string, middlewares ...HandlerFunc) *RouterGroup {
	combined := append(append([]HandlerFunc(nil), g.middlewares...), middlewares...)
	return &RouterGroup{
		router:      g.router,
		prefix:      g.prefix + prefix,
		middlewares: combined,
	}
}

func (g *RouterGroup) register(method, path string, handler HandlerFunc) {
	g.router.addRoute(method, g.prefix+path, handler, g.middlewares)
}

func (g *RouterGroup) GET(path string, handler HandlerFunc)    { g.register("GET", path, handler) }
func (g *RouterGroup) POST(path string, handler HandlerFunc)   { g.register("POST", path, handler) }
func (g *RouterGroup) PUT(path string, handler HandlerFunc)    { g.register("PUT", path, handler) }
func (g *RouterGroup) DELETE(path string, handler HandlerFunc) { g.register("DELETE", path, handler) }
func (g *RouterGroup) HEAD(path string, handler HandlerFunc)   { g.register("HEAD", path, handler) }
func (g *RouterGroup) OPTIONS(path string, handler HandlerFunc) {
	g.register("OPTIONS", path, handler)
}
func (g *RouterGroup) PATCH(path string, handler HandlerFunc) { g.register("PATCH", path, handler) }

// ANY registers a handler for all standard HTTP methods in this group.
func (g *RouterGroup) ANY(path string, handler HandlerFunc) {
	for _, method := range httpMethods {
		g.register(method, path, handler)
	}
}
