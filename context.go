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

// RouteError represents a collected request error.
type RouteError struct {
	Err  error
	Code int
}

// Context carries request-scoped state through the handler chain.
type Context struct {
	Response http.ResponseWriter
	Request  *http.Request

	params      []param
	errors      []*RouteError
	index       int
	handlers    []HandlerFunc
	handlersBuf []HandlerFunc
	store       map[string]interface{}
	aborted     bool
}

func (c *Context) Reset(w http.ResponseWriter, req *http.Request) {
	c.Response = w
	c.Request = req
	c.params = c.params[:0]
	c.errors = c.errors[:0]
	c.index = 0
	c.handlers = nil
	c.aborted = false
	if c.store != nil {
		for k := range c.store {
			delete(c.store, k)
		}
	}
}

// Next advances the middleware chain.
func (c *Context) Next() {
	c.index++
	for c.index < len(c.handlers) && !c.aborted {
		c.handlers[c.index](c)
		c.index++
	}
}

// Abort stops the middleware chain.
func (c *Context) Abort() {
	c.aborted = true
	c.index = len(c.handlers)
}

// AbortWithStatus aborts and writes an HTTP status code.
func (c *Context) AbortWithStatus(code int) {
	c.Status(code)
	c.Abort()
}

// Error collects an error for later processing.
func (c *Context) Error(err error) {
	if err == nil {
		return
	}
	c.errors = append(c.errors, &RouteError{Err: err})
}

// Fail aborts with an error status and message.
func (c *Context) Fail(code int, msg string) {
	c.errors = append(c.errors, &RouteError{
		Err:  fmt.Errorf("%s", msg),
		Code: code,
	})
	c.AbortWithStatus(code)
}

// Set stores a request-scoped value.
func (c *Context) Set(key string, value interface{}) {
	if c.store == nil {
		c.store = make(map[string]interface{})
	}
	c.store[key] = value
}

// Get retrieves a request-scoped value.
func (c *Context) Get(key string) (value interface{}, exists bool) {
	if c.store != nil {
		value, exists = c.store[key]
	}
	return
}

// Param returns a route parameter by name.
func (c *Context) Param(key string) string {
	for _, p := range c.params {
		if p.key == key {
			return p.value
		}
	}
	return ""
}

// Query returns a query string parameter.
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

// DefaultQuery returns a query parameter or a default value.
func (c *Context) DefaultQuery(key, defaultValue string) string {
	if v := c.Request.URL.Query().Get(key); v != "" {
		return v
	}
	return defaultValue
}

// QueryArray returns all values for a query key.
func (c *Context) QueryArray(key string) []string {
	return c.Request.URL.Query()[key]
}

// BindJSON decodes the request body as JSON.
func (c *Context) BindJSON(obj interface{}) error {
	return json.NewDecoder(c.Request.Body).Decode(obj)
}

// ShouldBindJSON is an alias for BindJSON.
func (c *Context) ShouldBindJSON(obj interface{}) error {
	return c.BindJSON(obj)
}

// Status sets the HTTP status code.
func (c *Context) Status(code int) {
	c.Response.WriteHeader(code)
}

// String sends a plain text response.
func (c *Context) String(code int, format string, values ...interface{}) {
	c.Status(code)
	c.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if len(values) == 0 {
		c.Response.Write([]byte(format))
		return
	}
	fmt.Fprintf(c.Response, format, values...)
}

// JSON sends a JSON response.
func (c *Context) JSON(code int, obj interface{}) {
	c.Status(code)
	c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(c.Response).Encode(obj)
}

// Data sends a raw response body.
func (c *Context) Data(code int, contentType string, data []byte) {
	c.Status(code)
	c.Response.Header().Set("Content-Type", contentType)
	c.Response.Write(data)
}

// Redirect issues an HTTP redirect.
func (c *Context) Redirect(code int, location string) {
	http.Redirect(c.Response, c.Request, location, code)
	c.Abort()
}

// File serves a file from disk.
func (c *Context) File(filepath string) {
	http.ServeFile(c.Response, c.Request, filepath)
}

// FileAttachment serves a file as a download attachment.
func (c *Context) FileAttachment(filepath, filename string) {
	c.Response.Header().Set("Content-Disposition", "attachment; filename="+filename)
	http.ServeFile(c.Response, c.Request, filepath)
}

// HTML renders HTML content (simple placeholder; pass rendered HTML as obj).
func (c *Context) HTML(code int, _ string, obj interface{}) {
	c.Status(code)
	c.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(c.Response, "%v", obj)
}
