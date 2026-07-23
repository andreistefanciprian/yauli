# Authentication

## Current implementation

The web application uses passwordless magic-link authentication:

* `auth-service` creates single-use, 15-minute magic links and sends them
  through Mailgun in production or logs them in local development.
* A verified link creates an opaque, revocable session. The browser stores
  only its session ID in an `HttpOnly`, `SameSite=Lax` cookie.
* `frontend` exchanges the session for short-lived JWT access tokens when it
  calls `backend-api`.
* `backend-api` verifies JWT signature and expiry and scopes baby/family access
  from the token claims.
* Logout, timeline-member removal, and baby-timeline archive paths revoke the
  affected sessions.

The detailed flow, trust boundaries, cookie behavior, and invite activation
are documented in [Magic Link Auth](auth-magic-link.md).

## Planned MCP authentication

OAuth 2.1 Authorization Code Flow with PKCE is planned for the future public
MCP/ChatGPT surface. OAuth clients, authorization codes, access tokens, and
refresh tokens are not implemented or migrated yet.

Google Sign-In and Apple Sign-In are possible future web login methods, not
current authentication options.
