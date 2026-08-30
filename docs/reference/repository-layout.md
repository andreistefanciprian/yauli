# Repository Layout

## Services and data flow

Yauli is implemented as small Go services. The server-rendered `frontend`
communicates privately with `auth-service` for sessions and with `backend-api`
for baby and timeline data. `backend-api` owns business rules and PostgreSQL
access, and can ask `auth-service` to revoke sessions when timeline access
changes.

```text
Browser
  |
Frontend
  |-- Auth Service
  `-- Backend API
        `-- PostgreSQL

ChatGPT
  |
MCP Server (planned)
  `-- Backend API
```

| Service | Status | Responsibility |
|---|---|---|
| `backend-api` | Active | Business rules, users, baby profiles, timeline access, events, reports, and PostgreSQL access |
| `frontend` | Active | Server-rendered sign-in, onboarding, timeline, event-entry, and settings UI |
| `auth-service` | Active | Magic links, sessions, JWT issuance, logout, invite links, and session revocation |
| `mcp-server` | Planned | OAuth-protected MCP tools that call `backend-api` |

The active services use Go, Chi, PostgreSQL, Go templates, HTMX, plain CSS,
Docker, and Railway. Production email is delivered through Mailgun. OAuth 2.1
with PKCE is planned for the public MCP integration.

## Top-level directories

```text
frontend/       Server-rendered web UI, HTMX handlers, templates, static assets
backend-api/    Baby and family domain logic, validation, events, reports
auth-service/   Magic links, sessions, JWT issuance and revocation
docs/           Architecture, design system, decisions, operational notes
evals/          Version-controlled AI report golden fixtures
branding/       Source brand assets
.github/        CI and repository automation
```

The planned `mcp-server/` service is not implemented yet. When added, it will
expose OAuth-protected MCP tools and call `backend-api` rather than PostgreSQL
directly.

See [Local Development](local-development.md) for setup and service URLs.

When adding a new top-level directory or changing a service boundary,
update this map.
