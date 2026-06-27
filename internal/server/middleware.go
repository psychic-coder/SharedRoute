package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

func RequestIDMiddleware(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.NewString()
		}
		ctx := context.WithValue(r.Context(), requestIDKey, rid)
		w.Header().Set("X-Request-ID", rid)
		start := time.Now()
		next.ServeHTTP(w, r.WithContext(ctx))
		log.Info("request", "request_id", rid, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}
