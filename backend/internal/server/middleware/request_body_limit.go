package middleware

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type requestBodyReadCounter struct {
	io.ReadCloser
	n atomic.Int64
}

func (r *requestBodyReadCounter) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.n.Add(int64(n))
	}
	return n, err
}

func (r *requestBodyReadCounter) BytesRead() int64 {
	if r == nil {
		return 0
	}
	return r.n.Load()
}

// RequestBodyLimit uses MaxBytesReader to cap request bodies and logs the
// observed body size for gateway diagnostics.
func RequestBodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		var counter *requestBodyReadCounter
		if c.Request != nil && c.Request.Body != nil {
			counter = &requestBodyReadCounter{ReadCloser: c.Request.Body}
			c.Request.Body = http.MaxBytesReader(c.Writer, counter, maxBytes)
			c.Request = c.Request.WithContext(pkghttputil.WithRequestBodyLimit(c.Request.Context(), maxBytes))
		}
		c.Next()

		if c.Request == nil {
			return
		}
		if !shouldLogRequestBodySize(c.Request.Method, c.Request.URL.Path, c.Writer.Status()) {
			return
		}
		fields := []zap.Field{
			zap.String("component", "http.request_body"),
			zap.String("request_id", requestBodyContextString(c, ctxkey.RequestID)),
			zap.String("client_request_id", requestBodyContextString(c, ctxkey.ClientRequestID)),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status_code", c.Writer.Status()),
			zap.Int64("content_length", c.Request.ContentLength),
			zap.String("content_encoding", c.GetHeader("Content-Encoding")),
			zap.Int64("body_limit_bytes", maxBytes),
			zap.Int64("body_read_bytes", counter.BytesRead()),
		}
		if c.Request.ContentLength > maxBytes {
			fields = append(fields, zap.Bool("content_length_exceeds_limit", true))
		}
		l := logger.With(fields...)
		if c.Writer.Status() == http.StatusRequestEntityTooLarge {
			l.Warn("request body size observed")
			return
		}
		l.Info("request body size observed")
	}
}

func requestBodyContextString(c *gin.Context, key ctxkey.Key) string {
	if c == nil || c.Request == nil {
		return ""
	}
	value, _ := c.Request.Context().Value(key).(string)
	return strings.TrimSpace(value)
}

func shouldLogRequestBodySize(method, path string, statusCode int) bool {
	if statusCode == http.StatusRequestEntityTooLarge {
		return true
	}
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
	default:
		return false
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	if strings.Contains(path, "/responses") {
		return true
	}
	switch {
	case strings.Contains(path, "/messages"),
		strings.Contains(path, "/chat/completions"),
		strings.Contains(path, "/images/"),
		strings.Contains(path, "/embeddings"):
		return true
	default:
		return false
	}
}
