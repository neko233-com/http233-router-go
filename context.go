package http233

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type param struct {
	key   string
	value string
}

type Context struct {
	Response http.ResponseWriter
	Request  *http.Request
	params   []param
	index    int
}

func (c *Context) Reset(w http.ResponseWriter, req *http.Request) {
	c.Response = w
	c.Request = req
	c.params = c.params[:0]
	c.index = 0
}

func (c *Context) Param(key string) string {
	for _, p := range c.params {
		if p.key == key {
			return p.value
		}
	}
	return ""
}

func (c *Context) Status(code int) {
	c.Response.WriteHeader(code)
}

func (c *Context) String(code int, format string, values ...interface{}) {
	c.Status(code)
	c.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(c.Response, format, values...)
}

func (c *Context) JSON(code int, obj interface{}) {
	c.Status(code)
	c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(c.Response).Encode(obj)
}
