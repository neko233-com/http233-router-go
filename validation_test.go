package http233

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func benchOnce(tb testing.TB, handler http.Handler, requestLine string) testing.BenchmarkResult {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatal(err)
	}
	defer ln.Close()

	go http.Serve(ln, handler)

	return testing.Benchmark(func(b *testing.B) {
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
	})
}

func TestPerformanceTargets(t *testing.T) {
	router := New()
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("/route%d", i)
		router.GET(path, func(c *Context) {
			c.String(http.StatusOK, "ok")
		})
	}

	result := benchOnce(t, router.Handler(), "GET /route50 HTTP/1.1")

	qps := float64(result.N) / result.T.Seconds()
	if qps < 30000 {
		t.Errorf("QPS target not met: %.0f < 30000", qps)
	}
	if result.AllocedBytesPerOp() > 10*1024*1024 {
		t.Errorf("Memory target not met: %d > 10MB", result.AllocedBytesPerOp())
	}

	t.Logf("QPS: %.0f", qps)
	t.Logf("Memory: %d bytes/op", result.AllocedBytesPerOp())
	t.Logf("Allocs: %d allocs/op", result.AllocsPerOp())
}

func TestFasterThanHttpRouter(t *testing.T) {
	const requestLine = "GET /route50 HTTP/1.1"

	http233Router := New()
	httpRouter := httprouter.New()
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("/route%d", i)
		http233Router.GET(path, func(c *Context) {
			c.Response.WriteHeader(http.StatusOK)
			c.Response.Write([]byte("ok"))
		})
		httpRouter.GET(path, func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})
	}

	http233Result := benchOnce(t, http233Router.Handler(), requestLine)
	httpRouterResult := benchOnce(t, httpRouter, requestLine)

	http233QPS := float64(http233Result.N) / http233Result.T.Seconds()
	httpRouterQPS := float64(httpRouterResult.N) / httpRouterResult.T.Seconds()

	t.Logf("http233 QPS: %.0f (%d allocs/op, %d B/op)", http233QPS, http233Result.AllocsPerOp(), http233Result.AllocedBytesPerOp())
	t.Logf("httprouter QPS: %.0f (%d allocs/op, %d B/op)", httpRouterQPS, httpRouterResult.AllocsPerOp(), httpRouterResult.AllocedBytesPerOp())

	if http233QPS < httpRouterQPS*0.95 {
		t.Errorf("http233 QPS %.0f should be within 95%% of httprouter QPS %.0f", http233QPS, httpRouterQPS)
	}
	if http233Result.AllocsPerOp() > httpRouterResult.AllocsPerOp()+2 {
		t.Errorf("http233 allocs/op %d should be within 2 of httprouter %d", http233Result.AllocsPerOp(), httpRouterResult.AllocsPerOp())
	}
}

func TestConcurrentAccess(t *testing.T) {
	router := New()
	router.GET("/test", func(c *Context) {
		c.String(http.StatusOK, "ok")
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go http.Serve(ln, router.Handler())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()

			for j := 0; j < 1000; j++ {
				fmt.Fprintf(conn, "GET /test HTTP/1.1\r\nHost: localhost\r\n\r\n")
				reader := bufio.NewReader(conn)
				resp, err := http.ReadResponse(reader, nil)
				if err != nil {
					t.Error(err)
					return
				}
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()
}

func TestMemoryPerRequest(t *testing.T) {
	router := New()
	for i := 0; i < 1000; i++ {
		path := fmt.Sprintf("/route%d", i)
		router.GET(path, func(c *Context) {
			c.String(http.StatusOK, "ok")
		})
	}

	for i := 0; i < 1000; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/route500", nil)
		router.ServeHTTP(w, req)
	}

	result := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/route500", nil)
			router.ServeHTTP(w, req)
		}
	})

	if result.AllocedBytesPerOp() > 10*1024*1024 {
		t.Errorf("Memory per request too high: %d bytes/op > 10MB", result.AllocedBytesPerOp())
	}

	t.Logf("Memory: %d bytes/op", result.AllocedBytesPerOp())
	t.Logf("Allocs: %d allocs/op", result.AllocsPerOp())
}
