package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTAuth(t *testing.T) {
	secret := "test-secret"
	authMiddleware := JWTAuth(secret)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify claims were injected
		claims := r.Context().Value(ClaimsKey)
		if claims == nil {
			t.Errorf("expected claims in context, got nil")
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := authMiddleware(next)

	// Helper to create token
	createToken := func(s string, claims jwt.MapClaims) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		str, _ := token.SignedString([]byte(s))
		return str
	}

	validToken := createToken(secret, jwt.MapClaims{
		"sub": "user1",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})

	expiredToken := createToken(secret, jwt.MapClaims{
		"sub": "user1",
		"exp": time.Now().Add(-1 * time.Hour).Unix(), // Expired
	})

	wrongSecretToken := createToken("wrong-secret", jwt.MapClaims{
		"sub": "user1",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})

	noExpToken := createToken(secret, jwt.MapClaims{
		"sub": "user1", // Missing "exp" which is required by our implementation
	})

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"Valid Token", "Bearer " + validToken, http.StatusOK},
		{"Missing Header", "", http.StatusUnauthorized},
		{"Invalid Format", "Token " + validToken, http.StatusUnauthorized},
		{"Expired Token", "Bearer " + expiredToken, http.StatusUnauthorized},
		{"Wrong Secret", "Bearer " + wrongSecretToken, http.StatusUnauthorized},
		{"Missing Exp Claim", "Bearer " + noExpToken, http.StatusUnauthorized},
		{"Malformed Token", "Bearer not.a.real.token", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.wantStatus {
				t.Errorf("expected status %v, got %v", tt.wantStatus, status)
			}
		})
	}
}
