package auth_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/osamah22/payments-microservice/internal/auth"
)

// newAPI returns the auth API on the test database, wired up like main does.
func newAPI(t *testing.T) http.Handler {
	t.Helper()
	svc, verifier := newService(t)
	mux := http.NewServeMux()
	auth.NewHandler(svc, verifier).Register(mux)
	return mux
}

// call sends one request to api, with an access token when bearer isn't
// empty, and returns the recorded response.
func call(api http.Handler, method, path, bearer, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rr := httptest.NewRecorder()
	api.ServeHTTP(rr, req)
	return rr
}

// decode reads a JSON response into a T, the way a client would.
func decode[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rr.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rr.Body, err)
	}
	return v
}

type tokensJSON struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func credentialsBody(email, password string) string {
	return fmt.Sprintf(`{"email": %q, "password": %q}`, email, password)
}

func refreshBody(token string) string {
	return fmt.Sprintf(`{"refresh_token": %q}`, token)
}

// TestSession walks one user through the whole API: register, log in, use
// the access token, refresh, and log out.
func TestSession(t *testing.T) {
	t.Parallel()
	api := newAPI(t)
	email := uuid.NewString() + "@example.com"

	rr := call(api, "POST", "/v1/auth/register", "", credentialsBody(email, password))
	if rr.Code != http.StatusCreated {
		t.Fatalf("register: status %d: %s", rr.Code, rr.Body)
	}
	if strings.Contains(rr.Body.String(), "argon2") {
		t.Fatalf("register response leaks the password hash: %s", rr.Body)
	}
	if rr := call(api, "POST", "/v1/auth/register", "", credentialsBody(email, password)); rr.Code != http.StatusConflict {
		t.Errorf("register twice: status %d, want 409", rr.Code)
	}

	rr = call(api, "POST", "/v1/auth/login", "", credentialsBody(email, password))
	if rr.Code != http.StatusOK {
		t.Fatalf("login: status %d: %s", rr.Code, rr.Body)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("login: Cache-Control = %q, want no-store", cc)
	}
	tokens := decode[tokensJSON](t, rr)
	if tokens.TokenType != "Bearer" || tokens.ExpiresIn != 15*60 || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("login: unexpected tokens %+v", tokens)
	}

	rr = call(api, "GET", "/v1/auth/me", tokens.AccessToken, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("me: status %d: %s", rr.Code, rr.Body)
	}
	if got := decode[struct {
		Email string `json:"email"`
	}](t, rr).Email; got != email {
		t.Errorf("me: email = %q, want %q", got, email)
	}

	rr = call(api, "POST", "/v1/auth/refresh", "", refreshBody(tokens.RefreshToken))
	if rr.Code != http.StatusOK {
		t.Fatalf("refresh: status %d: %s", rr.Code, rr.Body)
	}
	next := decode[tokensJSON](t, rr)

	rr = call(api, "POST", "/v1/auth/logout", "", refreshBody(next.RefreshToken))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("logout: status %d: %s", rr.Code, rr.Body)
	}
	if rr := call(api, "POST", "/v1/auth/refresh", "", refreshBody(next.RefreshToken)); rr.Code != http.StatusUnauthorized {
		t.Errorf("refresh after logout: status %d, want 401", rr.Code)
	}
}

func TestAuthAPIRefusals(t *testing.T) {
	t.Parallel()
	api := newAPI(t)
	email := uuid.NewString() + "@example.com"
	if rr := call(api, "POST", "/v1/auth/register", "", credentialsBody(email, password)); rr.Code != http.StatusCreated {
		t.Fatalf("register: status %d: %s", rr.Code, rr.Body)
	}

	tests := []struct {
		name, method, path, bearer, body string
		want                             int
	}{
		{"short password", "POST", "/v1/auth/register", "", credentialsBody(uuid.NewString()+"@example.com", "short"), http.StatusUnprocessableEntity},
		{"bad email", "POST", "/v1/auth/register", "", credentialsBody("not-an-email", password), http.StatusUnprocessableEntity},
		{"unknown field", "POST", "/v1/auth/register", "", `{"email": "a@example.com", "password": "x", "admin": true}`, http.StatusBadRequest},
		{"wrong password", "POST", "/v1/auth/login", "", credentialsBody(email, "wrong password, long enough"), http.StatusUnauthorized},
		{"unknown email", "POST", "/v1/auth/login", "", credentialsBody(uuid.NewString()+"@example.com", password), http.StatusUnauthorized},
		{"unknown refresh token", "POST", "/v1/auth/refresh", "", refreshBody("nope"), http.StatusUnauthorized},
		{"me without a token", "GET", "/v1/auth/me", "", "", http.StatusUnauthorized},
		{"me with a bad token", "GET", "/v1/auth/me", "not-a-jwt", "", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rr := call(api, tt.method, tt.path, tt.bearer, tt.body); rr.Code != tt.want {
				t.Errorf("status %d, want %d: %s", rr.Code, tt.want, rr.Body)
			}
		})
	}
}
