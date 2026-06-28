package http233

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func benchmarkRouter(b *testing.B, path, requestLine string) {
	router := New()
	router.GET(path, func(c *Context) {
		c.String(http.StatusOK, "ok")
	})
	benchServe(b, router.Handler(), requestLine)
}

func benchmarkHttpRouter(b *testing.B, path, requestLine string) {
	router := httprouter.New()
	router.GET(path, func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	benchServe(b, router, requestLine)
}

func benchServe(b *testing.B, handler http.Handler, requestLine string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	go http.Serve(ln, handler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			b.Fatal(err)
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		for pb.Next() {
			if _, err := fmt.Fprintf(conn, "%s\r\nHost: localhost\r\n\r\n", requestLine); err != nil {
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

func BenchmarkStaticRoutes(b *testing.B) {
	benchmarkRouter(b, "/route50", "GET /route50 HTTP/1.1")
}

func BenchmarkHttpRouterStaticRoutes(b *testing.B) {
	benchmarkHttpRouter(b, "/route50", "GET /route50 HTTP/1.1")
}

func BenchmarkStaticRoutes100(b *testing.B) {
	router := New()
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("/route%d", i)
		router.GET(path, func(c *Context) {
			c.String(http.StatusOK, "ok")
		})
	}
	benchServe(b, router.Handler(), "GET /route50 HTTP/1.1")
}

func BenchmarkHttpRouterStaticRoutes100(b *testing.B) {
	router := httprouter.New()
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("/route%d", i)
		router.GET(path, func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})
	}
	benchServe(b, router, "GET /route50 HTTP/1.1")
}

func BenchmarkParameterRoutes(b *testing.B) {
	router := New()
	router.GET("/users/:id", func(c *Context) {
		c.String(http.StatusOK, "user %s", c.Param("id"))
	})
	benchServe(b, router.Handler(), "GET /users/12345 HTTP/1.1")
}

func BenchmarkHttpRouterParameterRoutes(b *testing.B) {
	benchmarkHttpRouter(b, "/users/:id", "GET /users/12345 HTTP/1.1")
}

func BenchmarkWildcardRoute(b *testing.B) {
	router := New()
	router.GET("/files/*path", func(c *Context) {
		c.String(http.StatusOK, "%s", c.Param("path"))
	})
	benchServe(b, router.Handler(), "GET /files/docs/readme.md HTTP/1.1")
}

func BenchmarkHttpRouterWildcardRoute(b *testing.B) {
	router := httprouter.New()
	router.GET("/files/*path", func(w http.ResponseWriter, _ *http.Request, ps httprouter.Params) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "%s", ps.ByName("path"))
	})
	benchServe(b, router, "GET /files/docs/readme.md HTTP/1.1")
}

func BenchmarkBusinessAPI(b *testing.B) {
	router := setupBusinessAPI()
	benchServe(b, router.Handler(), "GET /api/v1/users/123 HTTP/1.1")
}

func BenchmarkMemoryUsage(b *testing.B) {
	router := New()
	router.GET("/route500", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

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
