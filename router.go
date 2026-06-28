package http233

import (
	"net/http"
	"sync"
)

type Router struct {
	root      node
	pool      sync.Pool
	maxParams uint8
}

func New() *Router {
	r := &Router{
		maxParams: 16,
	}
	r.pool = sync.Pool{
		New: func() interface{} {
			return &Context{
				params: make([]param, 0, r.maxParams),
			}
		},
	}
	return r
}

func (r *Router) GET(path string, handler handlerFunc) {
	r.addRoute("GET", path, handler)
}

func (r *Router) POST(path string, handler handlerFunc) {
	r.addRoute("POST", path, handler)
}

func (r *Router) PUT(path string, handler handlerFunc) {
	r.addRoute("PUT", path, handler)
}

func (r *Router) DELETE(path string, handler handlerFunc) {
	r.addRoute("DELETE", path, handler)
}

func (r *Router) Handler() http.Handler {
	return http.HandlerFunc(r.ServeHTTP)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := r.pool.Get().(*Context)
	ctx.Reset(w, req)

	node, params := r.findRoute(req.URL.Path)
	if node == nil {
		http.NotFound(w, req)
		r.pool.Put(ctx)
		return
	}

	for _, p := range params {
		ctx.params = append(ctx.params, p)
	}

	handler := node.getHandler(req.Method)
	if handler == nil {
		http.NotFound(w, req)
		r.pool.Put(ctx)
		return
	}

	handler(ctx)

	r.pool.Put(ctx)
}

func (r *Router) findRoute(path string) (*node, []param) {
	current := &r.root
	i := 0

	for i < len(path) {
		found := false

		for j := uint8(0); j < current.childCount; j++ {
			child := current.children[j]
			if child.nType == nodeStatic {
				prefixLen := child.findLongestPrefix(path[i:])
				if prefixLen > 0 && prefixLen == child.prefixLen {
					current = child
					i += int(prefixLen)
					found = true
					break
				}
			}
		}
		if !found {
			for _, child := range current.overflow {
				if child.nType == nodeStatic {
					prefixLen := child.findLongestPrefix(path[i:])
					if prefixLen > 0 && prefixLen == child.prefixLen {
						current = child
						i += int(prefixLen)
						found = true
						break
					}
				}
			}
		}
		if !found {
			return nil, nil
		}
	}

	return current, nil
}

func (n *node) getHandler(method string) handlerFunc {
	switch method {
	case "GET":
		if n.handle.get != nil {
			return n.handle.get
		}
	case "POST":
		if n.handle.post != nil {
			return n.handle.post
		}
	case "PUT":
		if n.handle.put != nil {
			return n.handle.put
		}
	case "DELETE":
		if n.handle.delete != nil {
			return n.handle.delete
		}
	case "HEAD":
		if n.handle.head != nil {
			return n.handle.head
		}
	case "OPTIONS":
		if n.handle.options != nil {
			return n.handle.options
		}
	case "PATCH":
		if n.handle.patch != nil {
			return n.handle.patch
		}
	}
	return n.handle.any
}
