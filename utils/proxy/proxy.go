package main

import (
	"bytes"
	"log"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"

	"github.com/andybalholm/brotli"
)

// target is the backend server to which requests will be proxied.
const target = "https://api.openai.com"


type teeReader struct {
	rc io.ReadCloser
	w io.Writer
}

func (tr teeReader) Read(p []byte) (int, error) {
	n, err := tr.rc.Read(p)
	if n >= 0 {
		tr.w.Write(p[:n])
	}
	return n, err
}

func (tr teeReader) Close() error {
	return tr.rc.Close()
}

type responseWriter struct {
	once sync.Once
	http.ResponseWriter
	io.Writer
	newReader func([]byte) io.Reader
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	//log.Println("\nWriting header", rw.ResponseWriter.Header())
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(p []byte) (int, error) {
	rw.once.Do(func() {
		rw.Writer.Write([]byte("\nResponse:\n"))
	})
	p2 := p
	if rw.newReader != nil {
		r := rw.newReader(p)
		io.Copy(rw.Writer, r)
		if rc, ok := r.(io.Closer); ok {
			rc.Close()
		}
	} else if _, err := rw.Writer.Write(p2); err != nil {
		log.Printf("\ncould not write %q", p)
	}
	return rw.ResponseWriter.Write(p)
}

func main() {
	// Parse the target URL.
	targetURL, err := url.Parse(target)
	if err != nil {
		log.Fatalf("\nFailed to parse target URL: %v", err)
	}

	// Create the reverse proxy.
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Customize the director to preserve the original request path and query
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Host = targetURL.Host // optional, depending on backend expectations
	}

	// Handle all incoming requests with the proxy.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("\nProxying request: %s %s", r.Method, r.URL)
		r.Body = teeReader{r.Body, os.Stdout}
		proxy.ServeHTTP(&responseWriter{ResponseWriter: w, Writer: os.Stderr, newReader: func(p []byte) io.Reader {
			return brotli.NewReader(bytes.NewReader(p))
		}}, r)
	})

	// Start the proxy server.
	log.Println("Starting proxy server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
