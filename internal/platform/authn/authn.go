package authn

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/osamah22/payments-microservice/internal/platform/apperr"
	"github.com/osamah22/payments-microservice/internal/platform/httpx"
	"github.com/osamah22/payments-microservice/internal/platform/logging"
	"golang.org/x/crypto/ed25519"
)

const (
	Issuer   = "payments-auth"
	Audience = "payments-api"
)

var (
	ErrMissingToken = fmt.Errorf("%w: missing bearer token", apperr.ErrUnauthenticated)
	ErrInvalidToken = fmt.Errorf("%w: invalid or expired access token", apperr.ErrUnauthenticated)
)

// KeyID is the public key used to validate the token
// returns the header of the token (first 8 characters).
func KeyID(key ed25519.PublicKey) string {
	sum := sha256.Sum256(key)
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

type Verifier struct {
	key    ed25519.PublicKey
	keyID  string
	parser *jwt.Parser
}

// NewVerifier creates  a new verifier with the provided key
func NewVerifier(key ed25519.PublicKey) *Verifier {
	return &Verifier{
		key:   key,
		keyID: KeyID(key),
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
			jwt.WithIssuer(Issuer),
			jwt.WithAudience(Audience),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
			jwt.WithLeeway(time.Second*30), // clocks on different servers drift
		),
	}
}

func (v *Verifier) Verify(token string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := v.parser.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if kid, _ := t.Header["kid"].(string); kid != v.keyID {
			return nil, fmt.Errorf("unknown key ID %q", kid)
		}
		return v.key, nil
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: subject %q is not a user ID", ErrInvalidToken, claims.Subject)
	}
	return id, nil
}

type userIDKey struct{}

// Require Middleware and validates token. Only if a token is valid next is called, otherwise
// it returns 401.
func (v *Verifier) Require(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.Header.Get("Authorization"), " ")
		if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			httpx.Error(w, r, ErrMissingToken)
			return
		}
		id, err := v.Verify(parts[1])
		if err != nil {
			slog.InfoContext(r.Context(), "access token refused", "err", err)
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			httpx.Error(w, r, ErrInvalidToken)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey{}, id)
		ctx = logging.With(ctx, slog.String("user_id", id.String()))
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// UserID returns the ID of the current user (from ctx)
func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey{}).(uuid.UUID)
	return id, ok
}
