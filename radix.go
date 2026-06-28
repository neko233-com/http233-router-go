package http233

func (n *node) addChild(child *node) {
	if n.childCount < 8 {
		n.children[n.childCount] = child
		n.childCount++
	} else {
		n.overflow = append(n.overflow, child)
	}
}

func (n *node) setRoute(method string, mws []HandlerFunc, handler HandlerFunc) {
	entry := routeEntry{handler: handler}
	if len(mws) > 0 {
		entry.chain = make([]HandlerFunc, len(mws)+1)
		copy(entry.chain, mws)
		entry.chain[len(mws)] = handler
	}

	switch method {
	case "GET":
		n.handle.get = entry
	case "POST":
		n.handle.post = entry
	case "PUT":
		n.handle.put = entry
	case "DELETE":
		n.handle.delete = entry
	case "HEAD":
		n.handle.head = entry
	case "OPTIONS":
		n.handle.options = entry
	case "PATCH":
		n.handle.patch = entry
	case "*":
		n.handle.any = entry
	}
}

func (n *node) getRouteEntry(method string) routeEntry {
	switch method {
	case "GET":
		if n.handle.get.handler != nil {
			return n.handle.get
		}
	case "POST":
		if n.handle.post.handler != nil {
			return n.handle.post
		}
	case "PUT":
		if n.handle.put.handler != nil {
			return n.handle.put
		}
	case "DELETE":
		if n.handle.delete.handler != nil {
			return n.handle.delete
		}
	case "HEAD":
		if n.handle.head.handler != nil {
			return n.handle.head
		}
	case "OPTIONS":
		if n.handle.options.handler != nil {
			return n.handle.options
		}
	case "PATCH":
		if n.handle.patch.handler != nil {
			return n.handle.patch
		}
	}
	return n.handle.any
}

func (n *node) hasAnyHandler() bool {
	return n.handle.get.handler != nil ||
		n.handle.post.handler != nil ||
		n.handle.put.handler != nil ||
		n.handle.delete.handler != nil ||
		n.handle.head.handler != nil ||
		n.handle.options.handler != nil ||
		n.handle.patch.handler != nil ||
		n.handle.any.handler != nil
}

func (n *node) iterChildren(fn func(*node) bool) {
	for j := uint8(0); j < n.childCount; j++ {
		if fn(n.children[j]) {
			return
		}
	}
	for _, child := range n.overflow {
		if fn(child) {
			return
		}
	}
}
