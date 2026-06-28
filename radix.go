package http233

func (n *node) findLongestPrefix(path string) uint8 {
	i := uint8(0)
	for i < n.prefixLen && i < uint8(len(path)) {
		if n.prefix[i] != path[i] {
			break
		}
		i++
	}
	return i
}

func (n *node) addChild(child *node) {
	if n.childCount < 8 {
		n.children[n.childCount] = child
		n.childCount++
	} else {
		n.overflow = append(n.overflow, child)
	}
}

func (n *node) setHandler(method string, handler handlerFunc) {
	switch method {
	case "GET":
		n.handle.get = handler
	case "POST":
		n.handle.post = handler
	case "PUT":
		n.handle.put = handler
	case "DELETE":
		n.handle.delete = handler
	case "HEAD":
		n.handle.head = handler
	case "OPTIONS":
		n.handle.options = handler
	case "PATCH":
		n.handle.patch = handler
	case "*":
		n.handle.any = handler
	}
}
