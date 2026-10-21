package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const compressionThreshold = 1024

var gzipPool = sync.Pool{New: func() any { w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed); return w }}

func acceptsGzip(values []string) bool {
	explicit, wildcard := -1.0, -1.0
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			parts := strings.Split(item, ";")
			q := 1.0
			for _, parameter := range parts[1:] {
				key, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
				if ok && strings.EqualFold(key, "q") {
					n, err := strconv.ParseFloat(value, 64)
					if err != nil || !(n >= 0 && n <= 1) {
						q = 0
					} else {
						q = n
					}
				}
			}
			switch strings.ToLower(strings.TrimSpace(parts[0])) {
			case "gzip":
				explicit = q
			case "*":
				wildcard = q
			}
		}
	}
	if explicit >= 0 {
		return explicit > 0
	}
	return wildcard > 0
}

func addEncodingVary(h http.Header) {
	for _, value := range h.Values("Vary") {
		for _, name := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(name), "Accept-Encoding") || strings.TrimSpace(name) == "*" {
				return
			}
		}
	}
	h.Add("Vary", "Accept-Encoding")
}

// Compression buffers at most one small prefix; large JSON responses stream.
func Compression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		addEncodingVary(w.Header())
		if r.Method == http.MethodHead || r.Header.Get("Range") != "" || !acceptsGzip(r.Header.Values("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}
		cw := &compressedWriter{ResponseWriter: w, status: http.StatusOK}
		defer cw.close()
		next.ServeHTTP(cw, r)
	})
}

type compressedWriter struct {
	http.ResponseWriter
	status        int
	headerWritten bool
	started       bool
	pending       bytes.Buffer
	gzip          *gzip.Writer
}

func (w *compressedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *compressedWriter) WriteHeader(code int) {
	if w.headerWritten || w.started {
		return
	}
	if code < 200 {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	w.status = code
	w.headerWritten = true
}
func (w *compressedWriter) start(compress bool) {
	if w.started {
		return
	}
	w.started = true
	h := w.Header()
	compress = compress && strings.HasPrefix(strings.ToLower(h.Get("Content-Type")), "application/json") && h.Get("Content-Encoding") == "" && h.Get("Content-Range") == "" && !strings.Contains(strings.ToLower(h.Get("Cache-Control")), "no-transform") && w.status != 204 && w.status != 304
	if compress {
		h.Del("Content-Length")
		h.Del("Accept-Ranges")
		h.Del("ETag")
		h.Set("Content-Encoding", "gzip")
		w.gzip = gzipPool.Get().(*gzip.Writer)
		w.gzip.Reset(w.ResponseWriter)
	}
	w.ResponseWriter.WriteHeader(w.status)
}
func (w *compressedWriter) write(p []byte) (int, error) {
	if w.gzip != nil {
		return w.gzip.Write(p)
	}
	return w.ResponseWriter.Write(p)
}
func (w *compressedWriter) Write(p []byte) (int, error) {
	if w.started {
		return w.write(p)
	}
	w.headerWritten = true
	if w.pending.Len()+len(p) < compressionThreshold {
		return w.pending.Write(p)
	}
	w.start(true)
	if _, err := w.write(w.pending.Bytes()); err != nil {
		return 0, err
	}
	w.pending.Reset()
	return w.write(p)
}
func (w *compressedWriter) Flush() {
	if !w.started {
		w.start(false)
		_, _ = w.write(w.pending.Bytes())
		w.pending.Reset()
	}
	if w.gzip != nil {
		_ = w.gzip.Flush()
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *compressedWriter) close() {
	if !w.started {
		w.start(false)
		_, _ = w.write(w.pending.Bytes())
	}
	if w.gzip != nil {
		_ = w.gzip.Close()
		w.gzip.Reset(io.Discard)
		gzipPool.Put(w.gzip)
	}
}
