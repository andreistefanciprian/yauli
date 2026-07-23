package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andreistefanciprian/yauli/frontend/internal/authclient"
)

func TestRequireSessionMissingSessionNavigatesEventStream(t *testing.T) {
	h := &Handlers{}
	nextCalled := false
	handler := h.RequireSession(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/timeline/events/stream", nil)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if nextCalled {
		t.Fatal("next handler was called without a session")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := rec.Body.String(); got != "event: navigate\ndata: /login\n\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestRequireSessionMissingSessionRedirectsOrdinaryRequest(t *testing.T) {
	h := &Handlers{}
	handler := h.RequireSession(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler was called without a session")
	}))

	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if location := rec.Header().Get("Location"); !strings.HasSuffix(location, "/login") {
		t.Fatalf("Location = %q, want /login", location)
	}
}

func TestRequireSessionRevokedSessionNavigatesEventStreamAndClearsCookie(t *testing.T) {
	h := &Handlers{Auth: sessionAuthClient{mintErr: authclient.ErrUnauthorized}}
	handler := h.RequireSession(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler was called with a revoked session")
	}))

	req := httptest.NewRequest(http.MethodGet, "/timeline/events/stream", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "revoked-session"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Body.String(); got != "event: navigate\ndata: /login\n\n" {
		t.Fatalf("body = %q", got)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName || cookies[0].MaxAge >= 0 {
		t.Fatalf("response cookies = %#v, want cleared %s cookie", cookies, sessionCookieName)
	}
}

func TestRequireSessionWithoutFamilyNavigatesEventStreamToOnboarding(t *testing.T) {
	h := &Handlers{Auth: sessionAuthClient{
		mintResult: authclient.MintResult{AccessToken: "access-token"},
	}}
	handler := h.RequireSession(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler was called without a family")
	}))

	req := httptest.NewRequest(http.MethodGet, "/timeline/events/stream", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "new-session"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Body.String(); got != "event: navigate\ndata: /onboarding\n\n" {
		t.Fatalf("body = %q", got)
	}
}

type sessionAuthClient struct {
	mintResult authclient.MintResult
	mintErr    error
}

func (sessionAuthClient) RequestMagicLink(context.Context, string) error {
	return nil
}

func (sessionAuthClient) RequestInviteMagicLink(context.Context, string, string) error {
	return nil
}

func (sessionAuthClient) VerifyMagicLink(context.Context, string) (authclient.VerifyResult, error) {
	return authclient.VerifyResult{}, nil
}

func (sessionAuthClient) Logout(context.Context, string) error {
	return nil
}

func (a sessionAuthClient) MintToken(context.Context, string) (authclient.MintResult, error) {
	return a.mintResult, a.mintErr
}

func (sessionAuthClient) AttachFamily(context.Context, string, string) error {
	return nil
}
