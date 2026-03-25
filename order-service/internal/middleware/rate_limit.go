package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimitMiddleware struct {
	redis     *redis.Client
	keyPrefix string
	limit     int
	window    time.Duration
	logger    *slog.Logger
}

func NewRateLimitMiddleware(
	redis *redis.Client,
	keyPrefix string,
	limit int,
	window time.Duration,
	logger *slog.Logger,
) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		redis:     redis,
		keyPrefix: keyPrefix,
		limit:     limit,
		window:    window,
		logger:    logger,
	}
}

func (m *RateLimitMiddleware) Limit() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := GetTraceIDContext(r.Context())

			var userID string
			if uid, ok := r.Context().Value("user_id").(string); ok {
				userID = uid
			}

			if userID == "" {
				userID = r.RemoteAddr
			}

			key := m.keyPrefix + userID

			allowed, err := m.checkRateLimit(r.Context(), key)
			if err != nil {
				m.logger.Error("Rate limit check failed", "trace_id", traceID, "error", err)
				http.Error(w, `{"error": "service_unavailable", "message": "Rate limit service unavailable"}`, http.StatusServiceUnavailable)
				return
			}

			if !allowed {
				m.logger.Warn("Rate limit exceeded", "trace_id", traceID, "user_id", userID)
				http.Error(w, `{"error": "rate_limit_exceeded", "message": "Too many requests"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (m *RateLimitMiddleware) checkRateLimit(ctx context.Context, key string) (bool, error) {
	script := redis.NewScript(`
		local current = redis.call("INCR", KEYS[1])
		if current == 1 then
			redis.call("EXPIRE", KEYS[1], ARGV[1])
		end
		return current
	`)

	result, err := script.Run(ctx, m.redis, []string{key}, int(m.window.Seconds())).Result()
	if err != nil {
		return false, err
	}

	count, ok := result.(int64)
	if !ok {
		return false, err
	}

	return count <= int64(m.limit), nil
}
