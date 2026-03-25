package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthHandler struct {
	jwtSecret []byte
	issuer    string
	audience  string
	logger    *slog.Logger
}

func NewAuthHandler(secret, issuer, audience string, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		jwtSecret: []byte(secret),
		issuer:    issuer,
		audience:  audience,
		logger:    logger,
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
	TokenType string `json:"token_type"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode login request", "error", err)
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_credentials", "Username and password are required")
		return
	}

	if len(req.Password) < 4 {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
		return
	}

	token, err := h.generateToken(req.Username)
	if err != nil {
		h.logger.Error("Failed to generate token", "error", err, "username", req.Username)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to generate token")
		return
	}

	h.logger.Info("User logged in", "username", req.Username)

	resp := LoginResponse{
		Token:     token,
		ExpiresIn: 86400,
		TokenType: "Bearer",
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) generateToken(userID string) (string, error) {
	expiresAt := time.Now().Add(time.Hour * 24)

	claims := jwt.MapClaims{
		"iss": h.issuer,
		"aud": h.audience,
		"sub": userID,
		"iat": time.Now().Unix(),
		"exp": expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
