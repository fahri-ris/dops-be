package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type JWTMiddleware struct {
	issuer   string
	audience string
	secret   []byte
	logger   *slog.Logger
}

func NewJWTMiddleware(issuer, audience, secret string, logger *slog.Logger) *JWTMiddleware {
	return &JWTMiddleware{
		issuer:   issuer,
		audience: audience,
		secret:   []byte(secret),
		logger:   logger,
	}
}

func (m *JWTMiddleware) Authenticate() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := GetTraceIDContext(r.Context())

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				m.logger.Warn("Missing Authorization header", "trace_id", traceID)
				http.Error(w, `{"error": "unauthorized", "message": "Missing Authorization header"}`, http.StatusUnauthorized)
				return
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				m.logger.Warn("Invalid Authorization format", "trace_id", traceID)
				http.Error(w, `{"error": "unauthorized", "message": "Invalid Authorization format"}`, http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")

			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return m.secret, nil
			})

			if err != nil || !token.Valid {
				m.logger.Warn("Invalid JWT token", "trace_id", traceID, "error", err)
				http.Error(w, `{"error": "unauthorized", "message": "Invalid token"}`, http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, `{"error": "unauthorized", "message": "Invalid claims"}`, http.StatusUnauthorized)
				return
			}

			if iss, ok := claims["iss"].(string); !ok || iss != m.issuer {
				m.logger.Warn("Invalid JWT issuer", "trace_id", traceID)
				http.Error(w, `{"error": "unauthorized", "message": "Invalid issuer"}`, http.StatusUnauthorized)
				return
			}

			if aud, ok := claims["aud"].(string); !ok || aud != m.audience {
				m.logger.Warn("Invalid JWT audience", "trace_id", traceID)
				http.Error(w, `{"error": "unauthorized", "message": "Invalid audience"}`, http.StatusUnauthorized)
				return
			}

			sub, _ := claims["sub"].(string)
			ctx := context.WithValue(r.Context(), "user_id", sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
