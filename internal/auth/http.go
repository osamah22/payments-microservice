package auth

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/osamah22/payments-microservice/internal/platform/authn"
	"github.com/osamah22/payments-microservice/internal/platform/httpx"
)

// The API's JSON shapes. They're separate from the database models on
// purpose: those follow the schema, these are a contract with our clients.
type (
	credentials struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	refreshRequest struct {
		RefreshToken string `json:"refresh_token"`
	}
	userResponse struct {
		ID        uuid.UUID `json:"id"`
		Email     string    `json:"email"`
		CreatedAt time.Time `json:"created_at"`
	}
	// tokensResponse has the shape of an OAuth 2.0 token response (RFC 6749,
	// section 5.1), which clients and tools already know.
	tokensResponse struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"` // seconds
		RefreshToken string `json:"refresh_token"`
	}
)

func toUserResponse(u *User) userResponse {
	return userResponse{ID: u.ID, Email: u.Email, CreatedAt: u.CreatedAt}
}

// Handler serves the auth API over HTTP. It only translates: JSON in, calls
// to the Service, JSON out. The rules live in the Service.
type Handler struct {
	svc      *Service
	verifier *authn.Verifier
}

// NewHandler returns a Handler that serves svc. verifier checks the access
// tokens on the routes that need one.
func NewHandler(svc *Service, verifier *authn.Verifier) *Handler {
	return &Handler{svc: svc, verifier: verifier}
}

// Register adds the auth API's routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /v1/auth/register", httpx.HandlerFunc(h.register))
	mux.Handle("POST /v1/auth/login", httpx.HandlerFunc(h.login))
	mux.Handle("POST /v1/auth/refresh", httpx.HandlerFunc(h.refresh))
	mux.Handle("POST /v1/auth/logout", httpx.HandlerFunc(h.logout))
	mux.Handle("GET /v1/auth/me", h.verifier.Require(httpx.HandlerFunc(h.me)))
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) error {
	var req credentials
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return err
	}
	u, err := h.svc.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, r, http.StatusCreated, toUserResponse(u))
	return nil
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) error {
	var req credentials
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return err
	}
	tokens, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		return err
	}
	writeTokens(w, r, tokens)
	return nil
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) error {
	var req refreshRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return err
	}
	tokens, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		return err
	}
	writeTokens(w, r, tokens)
	return nil
}

// logout always answers 204, even for an unknown token: either way, there's
// no session left.
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) error {
	var req refreshRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return err
	}
	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// me returns the user that the access token belongs to.
func (h *Handler) me(w http.ResponseWriter, r *http.Request) error {
	id, ok := authn.UserID(r.Context())
	if !ok {
		// Require always sets it. Without it, we don't know who's asking.
		return authn.ErrMissingToken
	}
	u, err := h.svc.User(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, r, http.StatusOK, toUserResponse(u))
	return nil
}

// writeTokens sends a new pair of tokens. Responses with credentials in them
// must never be cached, by the browser or by anything in between.
func writeTokens(w http.ResponseWriter, r *http.Request, t Tokens) {
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, r, http.StatusOK, tokensResponse{
		AccessToken:  t.Access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(t.ExpiresIn / time.Second),
		RefreshToken: t.Refresh,
	})
}
