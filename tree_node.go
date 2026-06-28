package http233

// handlerFunc is the handler function type
type handlerFunc func(*Context)

// nodeType represents the type of tree node
type nodeType uint8

const (
	nodeStatic  nodeType = iota // Static path segment
	nodeParam                   // Named parameter (:id)
	nodeWildcard                // Catch-all (*path)
)

// node is a radix tree node optimized for cache locality
type node struct {
	// Inline prefix for common paths (cache line friendly)
	prefix    [16]byte
	prefixLen uint8

	// Children stored inline for small fan-out
	children    [8]*node
	childCount  uint8
	childStatic uint8 // Count of static children (for binary search)

	// For nodes with >8 children, use overflow slice
	overflow []*node

	// Route data
	nType      nodeType
	paramName  string
	handle     methodHandler
	priority   uint16 // For tree balancing
}

// paramNode stores parameter name for :id style routes
type paramNode struct {
	node
}

// wildcardNode handles catch-all *path routes
type wildcardNode struct {
	node
}

// methodHandler stores handlers for different HTTP methods
type methodHandler struct {
	get     handlerFunc
	post    handlerFunc
	put     handlerFunc
	delete  handlerFunc
	head    handlerFunc
	options handlerFunc
	patch   handlerFunc
	any     handlerFunc // Catch-all for any method
}
