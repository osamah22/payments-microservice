package authn_test

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/osamah22/payments-microservice/internal/auth"
	"github.com/osamah22/payments-microservice/internal/platform/authn"
)

func TestRequire(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(nil)
	_, otherKey, _ := ed25519.GenerateKey(nil)
	verifier := authn.NewVerifier(pub)
	userID := uuid.New()

	token := func(key ed25519.PrivateKey, ttl time.Duration) string {
		t.Helper()
		s, err := auth.NewSigner(key, ttl).Issue(userID, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"valid token", "Bearer " + token(key, time.Minute), http.StatusOK},
		{"no header", "", http.StatusUnauthorized},
		{"not bearer", "Basic dXNlcjpwYXNz", http.StatusUnauthorized},
		{"garbage", "Bearer not-a-jwt", http.StatusUnauthorized},
		{"someone else's key", "Bearer " + token(otherKey, time.Minute), http.StatusUnauthorized},
		{"expired", "Bearer " + token(key, -time.Hour), http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called bool
			var got uuid.UUID
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				got, _ = authn.UserID(r.Context())
			})
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rr := httptest.NewRecorder()
			verifier.Require(next).ServeHTTP(rr, req)

			if rr.Code != tt.want {
				t.Errorf("status = %d, want %d: %s", rr.Code, tt.want, rr.Body)
			}
			// A refused request must never reach the handler behind Require.
			if wantCalled := tt.want == http.StatusOK; called != wantCalled {
				t.Errorf("next called = %v, want %v", called, wantCalled)
			}
			if called && got != userID {
				t.Errorf("user ID = %s, want %s", got, userID)
			}
			if !called && rr.Header().Get("WWW-Authenticate") == "" {
				t.Error("401 without a WWW-Authenticate header")
			}
		})
	}
}
