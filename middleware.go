package http233

import (
	"fmt"
	"log"
)

// Recovery returns middleware that recovers from panics.
func Recovery() HandlerFunc {
	return func(c *Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("http233: panic recovered: %v", r)
				var err error
				switch v := r.(type) {
				case error:
					err = v
				default:
					err = fmt.Errorf("%v", v)
				}
				c.Error(err)
				c.String(500, "Internal Server Error")
				c.Abort()
			}
		}()
		c.Next()
	}
}
