package http233

import (
	"net/http"
	"os"
	"strings"
)

// Static serves files from a directory under urlPrefix.
func (r *Router) Static(urlPrefix, rootDir string) {
	r.StaticFS(urlPrefix, http.Dir(rootDir))
}

// StaticFS serves files from a custom http.FileSystem.
func (r *Router) StaticFS(urlPrefix string, fs http.FileSystem) {
	if !strings.HasSuffix(urlPrefix, "/") {
		urlPrefix += "/"
	}
	fileServer := http.StripPrefix(urlPrefix, http.FileServer(fs))
	r.GET(urlPrefix+"*filepath", func(c *Context) {
		path := c.Param("filepath")
		if strings.Contains(path, "..") {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		fileServer.ServeHTTP(c.Response, c.Request)
	})
}

// StaticFile binds a single file to a URL path.
func (r *Router) StaticFile(url, filepath string) {
	r.GET(url, func(c *Context) {
		file, err := os.Open(filepath)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil || stat.IsDir() {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		http.ServeFile(c.Response, c.Request, filepath)
	})
}

// Static serves files from a directory within this group.
func (g *RouterGroup) Static(urlPrefix, rootDir string) {
	g.StaticFS(urlPrefix, http.Dir(rootDir))
}

// StaticFS serves files from a custom http.FileSystem within this group.
func (g *RouterGroup) StaticFS(urlPrefix string, fs http.FileSystem) {
	if !strings.HasSuffix(urlPrefix, "/") {
		urlPrefix += "/"
	}
	fileServer := http.StripPrefix(g.prefix+urlPrefix, http.FileServer(fs))
	g.GET(urlPrefix+"*filepath", func(c *Context) {
		path := c.Param("filepath")
		if strings.Contains(path, "..") {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		fileServer.ServeHTTP(c.Response, c.Request)
	})
}
