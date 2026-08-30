# Local Development

## Prerequisites

Install [Docker](https://www.docker.com/) and Docker Compose.

## Run Yauli locally

```bash
git clone https://github.com/andreistefanciprian/yauli.git
cd yauli
cp .env.example .env
docker compose up --build
```

This starts PostgreSQL, `backend-api`, `auth-service`, `frontend`, and Adminer.
With the default values from `.env.example`, they are available at:

| Service | URL |
|---|---|
| Frontend | [http://localhost:8080](http://localhost:8080) |
| Backend API | [http://localhost:8081](http://localhost:8081) |
| Auth Service | [http://localhost:8082](http://localhost:8082) |
| Adminer | [http://localhost:8083](http://localhost:8083) |

For Adminer, use `PostgreSQL` as the system, `postgres` as the server, and the
`POSTGRES_USER`, `POSTGRES_PASSWORD`, and `POSTGRES_DB` values from `.env`.

In local development, magic links are written to `auth-service` standard
output instead of being emailed:

```bash
docker compose logs auth-service
```

To rebuild only the frontend after changing templates or CSS:

```bash
docker compose up --build frontend
```

## Logging

All active services emit structured JSON logs. Set `LOG_LEVEL` to `debug`,
`info`, `warn`, or `error`; it defaults to `info` when unset.

HTTP completion logs include the service, request ID, method, path, status,
response size, and duration. Query strings are excluded so magic-link tokens
do not appear in request logs. Request IDs are forwarded between Yauli services
for correlation, and health-check requests are logged only at `debug` level.

For example:

```bash
LOG_LEVEL=debug docker compose up --build
```
