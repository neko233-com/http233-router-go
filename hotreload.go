package http233

import (
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type hotReload struct {
	enabled  bool
	watcher  *fsnotify.Watcher
	callback func([]fsnotify.Event)
	dirs     []string
	mu       sync.Mutex
	stopCh   chan struct{}
}

// EnableHotReload watches directories for static asset changes.
func (r *Router) EnableHotReload(dirs ...string) {
	r.hr.mu.Lock()
	defer r.hr.mu.Unlock()

	if r.hr.enabled {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("http233: failed to create file watcher: %v", err)
		return
	}

	r.hr.watcher = watcher
	r.hr.dirs = dirs
	r.hr.enabled = true
	r.hr.stopCh = make(chan struct{})

	for _, dir := range dirs {
		if err := watcher.Add(dir); err != nil {
			log.Printf("http233: failed to watch directory %s: %v", dir, err)
		}
	}

	go r.hr.loop()
}

// DisableHotReload stops file watching.
func (r *Router) DisableHotReload() {
	r.hr.mu.Lock()
	defer r.hr.mu.Unlock()

	if !r.hr.enabled {
		return
	}

	close(r.hr.stopCh)
	r.hr.watcher.Close()
	r.hr.enabled = false
}

// SetHotReloadCallback sets a callback invoked when watched files change.
func (r *Router) SetHotReloadCallback(fn func([]fsnotify.Event)) {
	r.hr.mu.Lock()
	defer r.hr.mu.Unlock()
	r.hr.callback = fn
}

// IsHotReloadEnabled reports whether hot reload is active.
func (r *Router) IsHotReloadEnabled() bool {
	r.hr.mu.Lock()
	defer r.hr.mu.Unlock()
	return r.hr.enabled
}

func (hr *hotReload) loop() {
	var events []fsnotify.Event
	for {
		select {
		case event, ok := <-hr.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
				events = append(events, event)
				if hr.callback != nil {
					hr.callback(events)
				}
				events = events[:0]
			}
		case err, ok := <-hr.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("http233: file watcher error: %v", err)
		case <-hr.stopCh:
			return
		}
	}
}
