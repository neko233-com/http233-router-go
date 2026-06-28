package http233

type nodeType uint8

const (
	nodeStatic nodeType = iota
	nodeParam
	nodeWildcard
)

type node struct {
	prefix string

	children    [8]*node
	childCount  uint8
	childStatic uint8

	overflow []*node

	nType     nodeType
	paramName string
	handle    methodHandler
	priority  uint16
}

type methodHandler struct {
	get     routeEntry
	post    routeEntry
	put     routeEntry
	delete  routeEntry
	head    routeEntry
	options routeEntry
	patch   routeEntry
	any     routeEntry
}

type routeEntry struct {
	handler HandlerFunc
	chain   []HandlerFunc
}

func (n *node) prefixLen() uint8 {
	if len(n.prefix) > 255 {
		return 255
	}
	return uint8(len(n.prefix))
}

func (n *node) findLongestPrefix(path string) uint8 {
	pl := n.prefixLen()
	i := uint8(0)
	for i < pl && int(i) < len(path) {
		if n.prefix[i] != path[i] {
			break
		}
		i++
	}
	return i
}
