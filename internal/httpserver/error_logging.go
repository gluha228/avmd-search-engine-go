package httpserver

import (
	"bytes"
	"log/slog"
	"net/http"
	"runtime/debug"
)

const maxLoggedResponseBodyBytes = 4096

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	body        bytes.Buffer
}

func (w *statusCapturingResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusCapturingResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.body.Len() < maxLoggedResponseBodyBytes {
		remaining := maxLoggedResponseBodyBytes - w.body.Len()
		if len(data) > remaining {
			_, _ = w.body.Write(data[:remaining])
		} else {
			_, _ = w.body.Write(data)
		}
	}
	return w.ResponseWriter.Write(data)
}

func (w *statusCapturingResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func withServerErrorLogging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturingWriter := &statusCapturingResponseWriter{ResponseWriter: w}
			defer func() {
				if recovered := recover(); recovered != nil {
					if logger != nil {
						logger.ErrorContext(
							r.Context(),
							"unexpected panic while handling request",
							"method", r.Method,
							"path", r.URL.Path,
							"query", r.URL.RawQuery,
							"panic", recovered,
							"stack", string(debug.Stack()),
						)
					}
					if !capturingWriter.wroteHeader {
						http.Error(capturingWriter, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					}
				}
				if capturingWriter.status >= http.StatusInternalServerError && logger != nil {
					logger.ErrorContext(
						r.Context(),
						"request completed with unexpected server error",
						"method", r.Method,
						"path", r.URL.Path,
						"query", r.URL.RawQuery,
						"status", capturingWriter.status,
						"response_body", capturingWriter.body.String(),
					)
				}
			}()
			next.ServeHTTP(capturingWriter, r)
		})
	}
}
