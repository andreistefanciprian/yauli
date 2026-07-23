# Repository Layout

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

When adding a new top-level directory or changing a service boundary,
update this map.
