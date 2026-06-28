package http233

func (r *Router) addRoute(method, path string, handler handlerFunc) {
	if len(path) == 0 {
		panic("http233: route path cannot be empty")
	}
	r.insertRoute(&r.root, method, path, handler)
}

func (r *Router) insertRoute(slot *node, method, path string, handler handlerFunc) {
	if len(path) > 0 && (path[0] == ':' || path[0] == '*') {
		isWildcard := path[0] == '*'
		rest := path[1:]
		end := 0
		for end < len(rest) && rest[end] != '/' {
			end++
		}
		paramName := rest[:end]
		remaining := rest[end:]

		nType := nodeParam
		if isWildcard {
			nType = nodeWildcard
		}

		for j := uint8(0); j < slot.childCount; j++ {
			child := slot.children[j]
			if child.nType == nType && child.paramName == paramName {
				if len(remaining) == 0 {
					child.setHandler(method, handler)
				} else {
					r.insertRoute(child, method, remaining, handler)
				}
				return
			}
		}
		for _, child := range slot.overflow {
			if child.nType == nType && child.paramName == paramName {
				if len(remaining) == 0 {
					child.setHandler(method, handler)
				} else {
					r.insertRoute(child, method, remaining, handler)
				}
				return
			}
		}

		child := &node{nType: nType, paramName: paramName}
		slot.addChild(child)
		if len(remaining) == 0 {
			child.setHandler(method, handler)
		} else {
			r.insertRoute(child, method, remaining, handler)
		}
		return
	}

	prefixLen := slot.findLongestPrefix(path)

	if prefixLen == 0 && slot.prefixLen > 0 {
		r.createChildOrRecurse(slot, method, path, handler)
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
		child.paramName = slot.paramName
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
		slot.paramName = ""
		copy(slot.prefix[:], path[:prefixLen])
		slot.handle = methodHandler{}
		slot.addChild(child)

		if prefixLen == uint8(len(path)) {
			slot.setHandler(method, handler)
		} else {
			r.createChildOrRecurse(slot, method, path[prefixLen:], handler)
		}
		return
	}

	next := path[prefixLen]
	for i := uint8(0); i < slot.childCount; i++ {
		if slot.children[i].nType == nodeStatic && slot.children[i].prefix[0] == next {
			r.insertRoute(slot.children[i], method, path[prefixLen:], handler)
			return
		}
	}
	for _, child := range slot.overflow {
		if child.nType == nodeStatic && child.prefix[0] == next {
			r.insertRoute(child, method, path[prefixLen:], handler)
			return
		}
	}

	r.createChildOrRecurse(slot, method, path[prefixLen:], handler)
}

func (r *Router) allocateNode(nType nodeType, prefix string) *node {
	n := &node{
		nType:     nType,
		prefixLen: uint8(len(prefix)),
	}
	copy(n.prefix[:], prefix)
	return n
}

func (r *Router) createChildOrRecurse(slot *node, method, path string, handler handlerFunc) {
	splitIdx := -1
	for i := 0; i < len(path); i++ {
		if path[i] == ':' || path[i] == '*' {
			splitIdx = i
			break
		}
	}
	if splitIdx > 0 {
		child := r.allocateNode(nodeStatic, path[:splitIdx])
		slot.addChild(child)
		r.insertRoute(child, method, path[splitIdx:], handler)
	} else {
		child := r.allocateNode(nodeStatic, path)
		slot.addChild(child)
		child.setHandler(method, handler)
	}
}
