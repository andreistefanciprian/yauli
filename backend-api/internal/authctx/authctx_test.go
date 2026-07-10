package authctx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testSigningSecret = "test-signing-key"

// signToken builds a JWT with the given subject/family_id, signed with an
// HS256 key matching the Middleware under test.
func signToken(t *testing.T, subject, familyID string) string {
	t.Helper()
	return signTokenWithSecretAndExpiry(t, subject, familyID, testSigningSecret, time.Now().Add(10*time.Minute))
}

func signTokenWithSecretAndExpiry(t *testing.T, subject, familyID, secret string, expiresAt time.Time) string {
	t.Helper()

	claims := jwtClaims{
		FamilyID: familyID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func handleWithClaims(t *testing.T, r *http.Request) (Claims, bool, int) {
	t.Helper()

	var got Claims
	var ok bool
	rec := httptest.NewRecorder()
	Middleware(testSigningSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = FromContext(r.Context())
	})).ServeHTTP(rec, r)
	return got, ok, rec.Code
}

func TestMiddleware_VerifiesAndDecodesUserIDAndFamilyID(t *testing.T) {
	userID := uuid.New()
	familyID := uuid.New()
	token := signToken(t, userID.String(), familyID.String())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	claims, ok, status := handleWithClaims(t, req)
	if !ok {
		t.Fatal("expected claims in context, got none")
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID)
	}
	if claims.FamilyID == nil || *claims.FamilyID != familyID {
		t.Errorf("FamilyID = %v, want %v", claims.FamilyID, familyID)
	}
}

func TestMiddleware_NoFamilyIDClaimYieldsNilFamilyID(t *testing.T) {
	userID := uuid.New()
	token := signToken(t, userID.String(), "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	claims, ok, status := handleWithClaims(t, req)
	if !ok {
		t.Fatal("expected claims in context, got none")
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if claims.FamilyID != nil {
		t.Errorf("FamilyID = %v, want nil", *claims.FamilyID)
	}
}

// TestMiddleware_ExplicitEmptyStringFamilyIDYieldsNilFamilyID guards against
// a regression where a family_id claim explicitly present as "" (as opposed
// to omitted entirely - signToken's omitempty tag means the two aren't the
// same on the wire) failed the whole claims decode, silently discarding a
// valid UserID along with it. jwt.MapClaims is used here (rather than
// signToken) specifically because it has no omitempty tag to elide the
// empty string.
func TestMiddleware_ExplicitEmptyStringFamilyIDYieldsNilFamilyID(t *testing.T) {
	userID := uuid.New()
	claims := jwt.MapClaims{
		"sub":       userID.String(),
		"family_id": "",
		"exp":       jwt.NewNumericDate(time.Now().Add(10 * time.Minute)).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-signing-key"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	got, ok, status := handleWithClaims(t, req)
	if !ok {
		t.Fatal("expected claims in context, got none")
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if got.UserID != userID {
		t.Errorf("UserID = %v, want %v", got.UserID, userID)
	}
	if got.FamilyID != nil {
		t.Errorf("FamilyID = %v, want nil", *got.FamilyID)
	}
}

func TestMiddleware_NoAuthorizationHeaderYieldsNoClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, ok, status := handleWithClaims(t, req)
	if ok {
		t.Fatal("expected no claims in context, got some")
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestMiddleware_MalformedTokenYieldsNoClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")

	_, ok, status := handleWithClaims(t, req)
	if ok {
		t.Fatal("expected no claims in context, got some")
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestMiddleware_WrongSigningSecretYieldsNoClaims(t *testing.T) {
	userID := uuid.New()
	token := signTokenWithSecretAndExpiry(t, userID.String(), "", "wrong-secret", time.Now().Add(10*time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, ok, status := handleWithClaims(t, req)
	if ok {
		t.Fatal("expected no claims in context, got some")
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestMiddleware_ExpiredTokenYieldsNoClaims(t *testing.T) {
	userID := uuid.New()
	token := signTokenWithSecretAndExpiry(t, userID.String(), "", testSigningSecret, time.Now().Add(-time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, ok, status := handleWithClaims(t, req)
	if ok {
		t.Fatal("expected no claims in context, got some")
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestMiddleware_MissingExpiryYieldsNoClaims(t *testing.T) {
	userID := uuid.New()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID.String(),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSigningSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, ok, status := handleWithClaims(t, req)
	if ok {
		t.Fatal("expected no claims in context, got some")
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestMiddleware_UnsignedTokenYieldsNoClaims(t *testing.T) {
	userID := uuid.New()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign unsigned token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, ok, status := handleWithClaims(t, req)
	if ok {
		t.Fatal("expected no claims in context, got some")
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

// TestMiddleware_LowercaseBearerSchemeIsAccepted guards against a
// regression where the scheme match was case-sensitive; RFC 7235's
// auth-scheme token is case-insensitive.
func TestMiddleware_LowercaseBearerSchemeIsAccepted(t *testing.T) {
	userID := uuid.New()
	token := signToken(t, userID.String(), "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "bearer "+token)

	claims, ok, status := handleWithClaims(t, req)
	if !ok {
		t.Fatal("expected claims in context, got none")
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID)
	}
}
