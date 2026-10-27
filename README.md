# Renora backend

Go service that serves Renora's GraphQL API and runs its background jobs.

One image, three binaries:

| Binary | Role | Ports |
|---|---|---|
| `api` | Public GraphQL API at `/graphql` | `8080` public, `9090` admin (`/metrics`, probes) |
| `worker` | River job queue: step clocks, notifications, AI calls | `9090` admin |
| `migrate` | Applies schema migrations; run as a release step before rollout | none |

## Run locally

Needs Go 1.26+, PostgreSQL 15+ and Redis 7+.

```sh
cp .env.example .env
make deps          # optional: Postgres 18 on :55432, Redis 8 on :56379 in Docker
make migrate-up
make api           # http://localhost:8080 (GraphQL playground at /)
make worker        # in another terminal; set ADMIN_ADDR=:9091 if the api uses :9090
```

To use a Postgres or Redis you already run, skip `make deps` and point `DATABASE_URL` and `REDIS_URL` in `.env` at them. Use a dedicated database (e.g. `renora`) and a Redis DB number that other projects don't use (e.g. `/1`).

`make help` lists every target.

## Layout

```
cmd/{api,worker,migrate}   entrypoints; wiring only
internal/api               router, admin router
internal/graph             schema (schema/*.graphqls), resolvers, GraphQL limits
internal/apperr            errors that are safe to show to clients
internal/config            env config with production safety checks
internal/platform/...      logging, telemetry, postgres, redis, queue, health, httpx
migrations                 goose SQL migrations, embedded in every binary
```

Domain modules (job, agreement, progress, handover, ...) go under `internal/<module>`.
Resolvers stay thin and call into them.

## Conventions

- **Schema first.** Edit `internal/graph/schema/*.graphqls`, then run `make generate`. CI fails if generated code is stale.
- **Errors.** Return `*apperr.Error` for anything the client should see. Every other error is logged and masked as `INTERNAL`. Fixable mutation input problems go in the payload's `userErrors`, not in top-level errors.
- **Database schemas.** Each Postgres schema has one owner. See [Database schemas](#database-schemas).
- **Migrations** are forward-only in production. Every migration needs a working `Down` for local use, and `TestUpDownUp` checks it.
- **Background work** is enqueued with River inside the same transaction as the data change it follows.
- **Time** is stored as `timestamptz` in UTC, and the connection time zone is pinned to UTC.

## Database schemas

The database is split into Postgres schemas by owner. Each schema is changed by exactly one migration system:

| Schema | Contents | Changed by |
|---|---|---|
| `public` | `goose_db_version` only: which app migrations have run | goose (`migrate up`) |
| `app` | Renora's own tables: users, jobs, agreements, ... | this repo's `migrations/` |
| `river` | Job queue tables (`river_job`, `river_leader`, ...) | River's migrator, run by `migrate up` |
| `auth` | Better Auth tables (`user`, `session`, `account`, ...) | Better Auth's migrations, in the frontend repo |

Why not put everything in `public`:

1. **No name clashes.** Better Auth creates tables named `user`, `session` and `account`. In one shared namespace, Renora could not use those names, and two migration systems could collide.
2. **Clear ownership.** River's tables never appear in this repo's migrations, and Better Auth never touches `app`. When a table changes, its schema shows who changed it.
3. **Tighter permissions.** In production, the Go service's database role can write only to `app` and `river`, with read-only or no access to `auth`.
4. **Easier operations.** Back up, restore or reset one part by schema name, e.g. `pg_dump -n app`.

Schemas cost nothing at query time. Queries name the schema explicitly (`app.jobs`), so nothing depends on `search_path`.

## Production settings

| Variable | Notes |
|---|---|
| `APP_ENV=production` | Introspection and playground default off. The playground cannot be turned on, and a `*` CORS origin is rejected at startup. |
| `HTTP_TRUSTED_PROXY_HOPS` | Number of proxies that append to `X-Forwarded-For`, e.g. `1` behind one load balancer. Leave at `0` when there is no proxy. |
| `HTTP_SHUTDOWN_DELAY` | How long `/readyz` reports draining before connections close. Set it to about your load balancer's probe interval, e.g. `10s`. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Turns on trace export. `OTEL_TRACES_SAMPLER_ARG` sets the sampling ratio. |
| `GRAPHQL_COMPLEXITY_LIMIT`, `GRAPHQL_DEPTH_LIMIT` | Query cost limits. |

Alert when `time() - max(queue_heartbeat_last_success_timestamp_seconds) > 300`: no worker is processing jobs.
