package http233

// Router is the main HTTP router
type Router struct {
	tree *node
}

// New creates a new Router instance
func New() *Router {
	return &Router{
		tree: &node{},
	}
}
