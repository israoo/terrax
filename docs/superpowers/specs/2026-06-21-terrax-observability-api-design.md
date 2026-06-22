# Design: TerraX Observability API

**Date:** 2026-06-21
**Status:** Approved

## Context

TerraX records execution history locally but has no cross-session, cross-machine, or cross-team visibility. Engineers using the VS Code extension cannot see drift status reported by CI pipelines, and CI pipelines cannot see what other engineers have executed locally. There is no persistent, queryable record of infrastructure execution history across a team.

This document describes a standalone backend API that ingests execution reports from any source (TerraX CLI, GitHub Actions, other CI systems) and serves execution history and stack drift status to consumers (VS Code extension, dashboards, scripts).

## Goals

1. Receive execution reports from TerraX CLI and CI pipelines via a REST API.
2. Persist execution history and compute stack drift status per project.
3. Serve stack status to the VS Code extension for drift visualization.
4. Support SaaS deployment (multi-tenant) and self-hosted deployment (single or multi-org) from the same codebase.
5. Provide a complete audit trail of all API activity.

## Non-Goals (this sub-project)

- TerraX CLI integration (separate sub-project).
- GitHub Actions reporter (separate sub-project).
- VS Code drift overlay (separate sub-project).
- Web UI / dashboard.
- Billing / payment processing.

---

## Architecture

### Stack

| Layer | Technology |
|---|---|
| Runtime | Hono (TypeScript) |
| Queries | Drizzle ORM (CRUD only — no migrations) |
| Migrations | Atlas CLI |
| Database | PostgreSQL 16 (all environments) |
| Deployment (SaaS) | Cloudflare Workers or Node.js container |
| Deployment (self-hosted) | Docker Compose |

### Multi-tenancy: schema-per-tenant

```
PostgreSQL instance
├── platform           ← global: organizations, auth, audit
│   ├── organizations
│   ├── api_keys
│   └── audit_log
├── tenant_acme        ← per org: operational data
│   ├── projects
│   ├── executions
│   └── stack_status
└── tenant_other_corp
    ├── projects
    ├── executions
    └── stack_status
```

Each organization gets its own PostgreSQL schema created at registration time. The platform schema contains only global auth and administrative data. Tenant schemas contain all operational data and are fully isolated — dropping `tenant_acme` removes that organization's data completely.

---

## Data Model

### `platform.organizations`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `slug` | text UNIQUE | URL-safe, used to name the schema: `tenant_{slug}` |
| `name` | text | Display name |
| `plan` | enum | `free \| pro \| enterprise` |
| `schema_name` | text | Exact PostgreSQL schema name assigned |
| `created_at` | timestamptz | |

### `platform.api_keys`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `org_id` | UUID FK → organizations | |
| `name` | text | Human label: `"github-actions-prod"` |
| `key_hash` | text | SHA-256 of the raw key — never stored in clear |
| `key_prefix` | text | First 8 chars for display: `tx_sk_ab12...` |
| `created_at` | timestamptz | |
| `last_used_at` | timestamptz | Updated on every authenticated request |
| `revoked_at` | timestamptz | Nullable — set to revoke |

The raw key (`tx_sk_<random_32_bytes>`) is generated and returned **once** at creation. Only `key_hash` and `key_prefix` are persisted.

### `platform.audit_log`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `org_id` | UUID | Nullable for system events |
| `actor_type` | enum | `api_key \| system \| admin` |
| `actor_id` | UUID | api_key.id or admin user id |
| `event` | text | `key.created`, `key.revoked`, `execution.reported`, `auth.failed`, `org.created`, etc. |
| `resource_type` | text | `api_key`, `execution`, `project`, `organization` |
| `resource_id` | UUID | |
| `metadata` | jsonb | Additional context |
| `ip_address` | text | |
| `occurred_at` | timestamptz | |

### `tenant_x.projects`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `project_id` | text UNIQUE | Slug from `.terrax.yaml` (e.g. `org-iac-cl-aws-caas`) |
| `name` | text | Display name |
| `repo_url` | text | Optional git remote URL |
| `root_config_file` | text | Default: `root.hcl` |
| `created_at` | timestamptz | |

### `tenant_x.executions`

| Column | Type | Notes |
|---|---|---|
| `id` | UUID PK | |
| `project_id` | text FK → projects | |
| `stack_path` | text | Relative: `workloads/dev/vpc` |
| `command` | text | `plan`, `apply`, `destroy`, etc. |
| `exit_code` | int | 0 = success |
| `duration_s` | float | |
| `drift_added` | int | Nullable — only relevant for `plan` |
| `drift_changed` | int | Nullable |
| `drift_destroyed` | int | Nullable |
| `executed_by` | text | OS username or `"github-actions"` |
| `source` | enum | `local \| ci` |
| `commit_sha` | text | Nullable |
| `branch` | text | Nullable |
| `ci_run_url` | text | Nullable — link to GH Actions run |
| `api_key_id` | UUID | Which API key reported this (cross-schema ref, no FK constraint) |
| `executed_at` | timestamptz | |

### `tenant_x.stack_status`

Maintained in-sync with `executions` — updated every time a `plan` execution is received for a given stack.

| Column | Type | Notes |
|---|---|---|
| `project_id` | text | Composite PK with stack_path |
| `stack_path` | text | Composite PK |
| `status` | enum | `clean \| changes \| error \| unknown` |
| `drift_added` | int | From last plan |
| `drift_changed` | int | |
| `drift_destroyed` | int | |
| `last_execution_id` | UUID | Reference to the execution that set this status |
| `planned_at` | timestamptz | |
| `planned_by` | text | |
| `planned_source` | enum | `local \| ci` |

`unknown` = no plan has ever been reported for this stack.

---

## API Endpoints

All endpoints require `X-API-Key: tx_sk_...` header except organization creation.

### Execution reporting

```
POST /v1/executions
Body: {
  project_id: string
  stack_path: string
  command: string
  exit_code: number
  duration_s: number
  drift_added?: number
  drift_changed?: number
  drift_destroyed?: number
  executed_by: string
  source: "local" | "ci"
  commit_sha?: string
  branch?: string
  ci_run_url?: string
}
Response 201: { id: string, created_at: string }

POST /v1/executions/batch
Body: {
  project_id: string
  command: string
  source: "local" | "ci"
  branch?: string
  commit_sha?: string
  stacks: Array<{
    stack_path: string
    exit_code: number
    duration_s: number
    drift_added?: number
    drift_changed?: number
    drift_destroyed?: number
  }>
}
Response 201: { created: number, ids: string[] }
```

Both endpoints update `stack_status` when `command === "plan"`.

### Stack status

```
GET /v1/projects/:project_id/stacks
Response 200: Array<{
  stack_path: string
  status: "clean" | "changes" | "error" | "unknown"
  drift_added: number
  drift_changed: number
  drift_destroyed: number
  planned_at: string | null
  planned_by: string | null
  planned_source: "local" | "ci" | null
}>

GET /v1/projects/:project_id/stacks?path=workloads%2Fdev%2Fvpc
Response 200: {
  ...stack_status,
  last_executions: Execution[]  // 5 most recent
}
Note: stack_path values contain "/" and must be URL-encoded in query params.
```

### Organization management

```
POST /v1/organizations
  Body: { name: string, slug: string }
  Response 201: { id, slug, schema_name }
  Side-effect: creates tenant_{slug} PostgreSQL schema + runs tenant migrations via Atlas

POST /v1/organizations/:org_id/projects
  Body: { project_id, name, repo_url?, root_config_file? }
  Response 201: { id, project_id }

GET /v1/organizations/:org_id/projects
  Response 200: Project[]

POST /v1/organizations/:org_id/api-keys
  Body: { name: string }
  Response 201: { id, name, key: "tx_sk_...", key_prefix }  ← key shown ONCE

GET /v1/organizations/:org_id/api-keys
  Response 200: Array<{ id, name, key_prefix, created_at, last_used_at, revoked_at }>

DELETE /v1/organizations/:org_id/api-keys/:key_id
  Response 204
```

---

## Project Structure

```
terrax-api/
├── src/
│   ├── index.ts
│   ├── routes/
│   │   ├── executions.ts
│   │   ├── stacks.ts
│   │   ├── organizations.ts
│   │   └── api-keys.ts
│   ├── middleware/
│   │   ├── auth.ts          # X-API-Key → org + tenantSchema
│   │   └── audit.ts         # writes platform.audit_log
│   ├── db/
│   │   ├── platform/
│   │   │   └── schema.ts    # Drizzle schema: organizations, api_keys, audit_log
│   │   ├── tenant/
│   │   │   └── schema.ts    # Drizzle table factory: projects, executions, stack_status
│   │   └── client.ts        # drizzle(pool) + withTenant(schema) helper
│   └── lib/
│       ├── keys.ts          # generate tx_sk_... + SHA-256 hash
│       └── tenant.ts        # CREATE SCHEMA + apply Atlas migrations
├── atlas/
│   ├── platform.hcl         # Atlas schema for platform
│   └── tenant.hcl           # Atlas schema template for new tenants
├── docker-compose.yml
├── package.json
└── wrangler.toml            # optional CF Workers deployment
```

## Auth Middleware Flow

```
Request →
  auth.ts:
    1. Extract X-API-Key header
    2. hash(key) → query platform.api_keys
    3. If not found or revoked → 401
    4. Update last_used_at
    5. ctx.set({ orgId, tenantSchema }) →
  route handler:
    db.withTenant(ctx.tenantSchema).insert(executions).values(...)
  audit.ts:
    platform.audit_log.insert({ event: "execution.reported", ... })
```

## Self-Hosted Deployment

```yaml
# docker-compose.yml
services:
  api:
    image: ghcr.io/israoo/terrax-api:latest
    environment:
      DATABASE_URL: postgres://postgres:password@db:5432/terrax
      PORT: 3000
    ports:
      - "3000:3000"
    depends_on:
      - db
  db:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: password
      POSTGRES_DB: terrax
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
```

## `.terrax.yaml` Integration (future sub-project)

```yaml
observability:
  api_url: "https://api.terrax.io"   # or self-hosted URL
  project_id: "org-iac-cl-aws-caas"
  api_key: "tx_sk_..."
```

TerraX CLI will post to `POST /v1/executions/batch` after each `executor.Run` call when this section is present — fire-and-forget, never blocks execution.
