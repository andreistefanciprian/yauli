// Package authctx verifies the Authorization: Bearer JWT that auth-service
// mints, decodes the caller's identity, and makes it available via
// context.Context.
package authctx

import (
	"context"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golang-jwt/jwt/v5/request"
	"github.com/google/uuid"
)

type contextKey int

const claimsContextKey contextKey = iota

// Claims is what backend-api knows about the caller. FamilyID is nil until
// the user has completed onboarding (see docs/auth-magic-link.md's nullable
// session/JWT family_id) - a brand-new user's token carries no family_id
// claim at all.
type Claims struct {
	UserID   uuid.UUID  `json:"user_id"`
	FamilyID *uuid.UUID `json:"family_id,omitempty"`
}

// jwtClaims mirrors the shape auth-service signs: sub=user_id, plus an
// optional family_id claim. FamilyID is decoded as a string, not
// *uuid.UUID, so that an empty-string value (as opposed to an omitted
// field) can be treated as "absent" in Middleware rather than failing the
// whole claims decode.
type jwtClaims struct {
	FamilyID string `json:"family_id,omitempty"`
	jwt.RegisteredClaims
}

// bearerExtractor pulls the token out of "Authorization: Bearer <token>",
// matching the scheme case-insensitively per RFC 7235.
var bearerExtractor = request.BearerExtractor{}

// Middleware verifies the Authorization: Bearer JWT's signature and expiry,
// then stores its claims in context for handlers. It is mounted only on
// user-facing /api/v1 routes: /internal keeps its shared-secret middleware
// and /healthz stays open.
func Middleware(signingSecret string) func(http.Handler) http.Handler {
	parser := jwt.NewParser(
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	secret := []byte(signingSecret)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := bearerExtractor.ExtractToken(r)
			if err != nil {
				writeUnauthorized(w)
				return
			}

			var claims jwtClaims
			if _, err := parser.ParseWithClaims(token, &claims, func(token *jwt.Token) (any, error) {
				return secret, nil
			}); err != nil {
				log.Printf("authctx: verify bearer token: %v", err)
				writeUnauthorized(w)
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				log.Printf("authctx: claims sub is not a valid uuid: %v", err)
				writeUnauthorized(w)
				return
			}

			var familyID *uuid.UUID
			if claims.FamilyID != "" {
				parsed, err := uuid.Parse(claims.FamilyID)
				if err != nil {
					log.Printf("authctx: claims family_id is not a valid uuid: %v", err)
					writeUnauthorized(w)
					return
				}
				familyID = &parsed
			}

			ctx := context.WithValue(r.Context(), claimsContextKey, Claims{
				UserID:   userID,
				FamilyID: familyID,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"authentication required"}`))
}

// FromContext returns the claims Middleware decoded, if any.
func FromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(Claims)
	return claims, ok
}
