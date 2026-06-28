package http233

func (r *Router) addRoute(method, path string, handler handlerFunc) {
	if len(path) == 0 {
		panic("http233: route path cannot be empty")
	}
	r.insertRoute(&r.root, method, path, handler)
}

func (r *Router) insertRoute(slot *node, method, path string, handler handlerFunc) {
	prefixLen := slot.findLongestPrefix(path)

	if prefixLen == 0 && slot.prefixLen > 0 {
		child := r.allocateNode(nodeStatic, path)
		slot.addChild(child)
		child.setHandler(method, handler)
		return
	}

	if prefixLen == uint8(len(path)) && prefixLen == slot.prefixLen {
		slot.setHandler(method, handler)
		return
	}

	if prefixLen < slot.prefixLen {
		remaining := slot.prefixLen - prefixLen
		child := r.allocateNode(slot.nType, "")
		copy(child.prefix[:], slot.prefix[prefixLen:slot.prefixLen])
		child.prefixLen = remaining
		child.handle = slot.handle
		for i := uint8(0); i < slot.childCount; i++ {
			child.addChild(slot.children[i])
			slot.children[i] = nil
		}
		child.childStatic = slot.childStatic
		if len(slot.overflow) > 0 {
			child.overflow = append(child.overflow, slot.overflow...)
			slot.overflow = nil
		}
		slot.childCount = 0
		slot.childStatic = 0

		slot.nType = nodeStatic
		slot.prefixLen = prefixLen
		copy(slot.prefix[:], path[:prefixLen])
		slot.handle = methodHandler{}
		slot.addChild(child)

		if prefixLen == uint8(len(path)) {
			slot.setHandler(method, handler)
		} else {
			newChild := r.allocateNode(nodeStatic, path[prefixLen:])
			slot.addChild(newChild)
			newChild.setHandler(method, handler)
		}
		return
	}

	next := path[prefixLen]
	for i := uint8(0); i < slot.childCount; i++ {
		if slot.children[i].prefix[0] == next {
			r.insertRoute(slot.children[i], method, path[prefixLen:], handler)
			return
		}
	}
	for _, child := range slot.overflow {
		if child.prefix[0] == next {
			r.insertRoute(child, method, path[prefixLen:], handler)
			return
		}
	}

	child := r.allocateNode(nodeStatic, path[prefixLen:])
	slot.addChild(child)
	child.setHandler(method, handler)
}

func (r *Router) allocateNode(nType nodeType, prefix string) *node {
	n := &node{
		nType:     nType,
		prefixLen: uint8(len(prefix)),
	}
	copy(n.prefix[:], prefix)
	return n
}
