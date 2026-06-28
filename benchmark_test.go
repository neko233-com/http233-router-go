package http233

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

func BenchmarkStaticRoutes(b *testing.B) {
	router := New()

	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("/route%d", i)
		router.GET(path, func(c *Context) {
			c.String(http.StatusOK, "ok")
		})
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	go http.Serve(ln, router.Handler())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			b.Fatal(err)
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)

		for pb.Next() {
			_, err := fmt.Fprintf(conn, "GET /route50 HTTP/1.1\r\nHost: localhost\r\n\r\n")
			if err != nil {
				b.Fatal(err)
			}

			resp, err := http.ReadResponse(reader, nil)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

func BenchmarkParameterRoutes(b *testing.B) {
	router := New()

	router.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "user %s", c.Param("id"))
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	go http.Serve(ln, router.Handler())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			b.Fatal(err)
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)

		for pb.Next() {
			_, err := fmt.Fprintf(conn, "GET /users/12345 HTTP/1.1\r\nHost: localhost\r\n\r\n")
			if err != nil {
				b.Fatal(err)
			}

			resp, err := http.ReadResponse(reader, nil)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

func BenchmarkMemoryUsage(b *testing.B) {
	router := New()

	for i := 0; i < 1000; i++ {
		path := fmt.Sprintf("/route%d", i)
		router.GET(path, func(c *Context) {
			c.String(http.StatusOK, "ok")
		})
	}

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/route500", nil)
		router.ServeHTTP(w, req)
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
}
