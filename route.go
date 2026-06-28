package http233

func (r *Router) addRoute(method, path string, handler HandlerFunc, mws []HandlerFunc) {
	if len(path) == 0 {
		panic("http233: route path cannot be empty")
	}
	r.insertRoute(&r.root, method, path, handler, mws)
}

func (r *Router) insertRoute(slot *node, method, path string, handler HandlerFunc, mws []HandlerFunc) {
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
					child.setRoute(method, mws, handler)
				} else {
					r.insertRoute(child, method, remaining, handler, mws)
				}
				return
			}
		}
		for _, child := range slot.overflow {
			if child.nType == nType && child.paramName == paramName {
				if len(remaining) == 0 {
					child.setRoute(method, mws, handler)
				} else {
					r.insertRoute(child, method, remaining, handler, mws)
				}
				return
			}
		}

		child := &node{nType: nType, paramName: paramName}
		slot.addChild(child)
		if len(remaining) == 0 {
			child.setRoute(method, mws, handler)
		} else {
			r.insertRoute(child, method, remaining, handler, mws)
		}
		return
	}

	prefixLen := slot.findLongestPrefix(path)

	if prefixLen == 0 && len(slot.prefix) > 0 {
		r.createChildOrRecurse(slot, method, path, handler, mws)
		return
	}

	if int(prefixLen) == len(path) && int(prefixLen) == len(slot.prefix) {
		slot.setRoute(method, mws, handler)
		return
	}

	if int(prefixLen) < len(slot.prefix) {
		child := &node{
			nType:     slot.nType,
			prefix:    slot.prefix[prefixLen:],
			paramName: slot.paramName,
			handle:    slot.handle,
		}
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
		slot.prefix = path[:prefixLen]
		slot.paramName = ""
		slot.handle = methodHandler{}
		slot.addChild(child)

		if int(prefixLen) == len(path) {
			slot.setRoute(method, mws, handler)
		} else {
			r.createChildOrRecurse(slot, method, path[prefixLen:], handler, mws)
		}
		return
	}

	if int(prefixLen) == len(slot.prefix) && int(prefixLen) < len(path) {
		remaining := path[prefixLen:]
		if len(remaining) > 0 && (remaining[0] == ':' || remaining[0] == '*') {
			r.insertRoute(slot, method, remaining, handler, mws)
			return
		}
	}

	next := path[prefixLen]
	for i := uint8(0); i < slot.childCount; i++ {
		if slot.children[i].nType == nodeStatic && len(slot.children[i].prefix) > 0 && slot.children[i].prefix[0] == next {
			r.insertRoute(slot.children[i], method, path[prefixLen:], handler, mws)
			return
		}
	}
	for _, child := range slot.overflow {
		if child.nType == nodeStatic && len(child.prefix) > 0 && child.prefix[0] == next {
			r.insertRoute(child, method, path[prefixLen:], handler, mws)
			return
		}
	}

	r.createChildOrRecurse(slot, method, path[prefixLen:], handler, mws)
}

func (r *Router) allocateNode(nType nodeType, prefix string) *node {
	return &node{nType: nType, prefix: prefix}
}

func (r *Router) createChildOrRecurse(slot *node, method, path string, handler HandlerFunc, mws []HandlerFunc) {
	if len(path) > 0 && (path[0] == ':' || path[0] == '*') {
		r.insertRoute(slot, method, path, handler, mws)
		return
	}

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
		r.insertRoute(child, method, path[splitIdx:], handler, mws)
	} else {
		child := r.allocateNode(nodeStatic, path)
		slot.addChild(child)
		child.setRoute(method, mws, handler)
	}
}
