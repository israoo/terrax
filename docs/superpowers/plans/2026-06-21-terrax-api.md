# TerraX Observability API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a REST API backend that receives Terragrunt execution reports and serves stack drift status, supporting multi-tenant SaaS and self-hosted deployment from the same codebase.

**Architecture:** Hono (TypeScript) with schema-per-tenant PostgreSQL multi-tenancy. Each organization gets its own PostgreSQL schema (`tenant_{slug}`) isolated from others. The `platform` schema holds global auth data. Drizzle ORM handles all queries; Atlas CLI handles all migrations (no ORM-managed DDL). Docker Compose provides the runtime for self-hosted deployment.

**Tech Stack:** Hono 4.x · TypeScript 5.x · Drizzle ORM · postgres.js · Atlas CLI · PostgreSQL 16 · Vitest · pnpm

## Global Constraints

- Working directory: `/path/to/terrax-api` (new repo — create it first).
- Package manager: `pnpm`.
- All API keys have format `tx_sk_<64 hex chars>`, hashed with SHA-256 for storage.
- Key prefix for display: first 12 characters of the full key (e.g. `tx_sk_ab12cd`).
- Tenant schema name: `tenant_<org_slug>` (slug replaces `-` with `_` and removes special chars).
- `stack_status` is upserted on every `plan` execution report.
- All endpoints except `POST /v1/organizations` require `X-API-Key` header.
- HTTP errors use `{ error: string }` JSON body.
- All timestamps are UTC ISO 8601 strings in responses.
- Migrations are applied by Atlas CLI, never by Drizzle or TypeORM.

---

### Task 1: Project scaffolding + database schema

**Files:**
- Create: `package.json`
- Create: `tsconfig.json`
- Create: `src/index.ts`
- Create: `src/db/platform/schema.ts`
- Create: `src/db/tenant/schema.ts`
- Create: `src/db/client.ts`
- Create: `atlas/platform.hcl`
- Create: `atlas/tenant.hcl`
- Create: `docker-compose.yml`
- Create: `.env.example`

**Interfaces:**
- Produces: `db` — Drizzle client connected to PostgreSQL
- Produces: `platformSchema` — Drizzle table definitions for platform schema
- Produces: `tenantTables(schemaName: string)` — factory returning Drizzle tables for a tenant schema
- Produces: Hono app on port 3000 with `GET /health` returning `{ status: "ok" }`

- [ ] **Step 1: Create and enter the project directory**

```bash
mkdir -p /path/to/terrax-api
cd /path/to/terrax-api
git init
```

- [ ] **Step 2: Create `package.json`**

```json
{
  "name": "terrax-api",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "tsx watch src/index.ts",
    "build": "tsc",
    "start": "node dist/index.js",
    "test": "vitest run",
    "test:watch": "vitest",
    "db:migrate:platform": "atlas schema apply --url $DATABASE_URL --to file://atlas/platform.hcl --dev-url $DEV_DATABASE_URL",
    "db:migrate:tenant": "atlas schema apply --url $DATABASE_URL --to file://atlas/tenant.hcl --dev-url $DEV_DATABASE_URL"
  },
  "dependencies": {
    "hono": "^4.0.0",
    "drizzle-orm": "^0.36.0",
    "postgres": "^3.4.0",
    "@hono/node-server": "^1.13.0"
  },
  "devDependencies": {
    "@types/node": "^22.0.0",
    "tsx": "^4.0.0",
    "typescript": "^5.6.0",
    "vitest": "^2.0.0"
  }
}
```

- [ ] **Step 3: Install dependencies**

```bash
pnpm install
```

- [ ] **Step 4: Create `tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "outDir": "dist",
    "rootDir": "src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true
  },
  "include": ["src"],
  "exclude": ["node_modules", "dist"]
}
```

- [ ] **Step 5: Create `.env.example`**

```bash
DATABASE_URL=postgres://postgres:password@localhost:5432/terrax
DEV_DATABASE_URL=postgres://postgres:password@localhost:5432/terrax_dev
PORT=3000
```

- [ ] **Step 6: Create `docker-compose.yml`**

```yaml
services:
  api:
    build: .
    environment:
      DATABASE_URL: postgres://postgres:password@db:5432/terrax
      PORT: 3000
    ports:
      - "3000:3000"
    depends_on:
      db:
        condition: service_healthy

  db:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: password
      POSTGRES_DB: terrax
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  pgdata:
```

- [ ] **Step 7: Create `src/db/platform/schema.ts`**

```typescript
import {
  pgSchema,
  uuid,
  text,
  timestamp,
  pgEnum,
  jsonb,
} from 'drizzle-orm/pg-core'

export const platformSchema = pgSchema('platform')

export const planEnum = platformSchema.enum('plan', ['free', 'pro', 'enterprise'])
export const actorTypeEnum = platformSchema.enum('actor_type', ['api_key', 'system', 'admin'])

export const organizations = platformSchema.table('organizations', {
  id: uuid('id').primaryKey().defaultRandom(),
  slug: text('slug').notNull().unique(),
  name: text('name').notNull(),
  plan: planEnum('plan').notNull().default('free'),
  schemaName: text('schema_name').notNull(),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
})

export const apiKeys = platformSchema.table('api_keys', {
  id: uuid('id').primaryKey().defaultRandom(),
  orgId: uuid('org_id').notNull().references(() => organizations.id),
  name: text('name').notNull(),
  keyHash: text('key_hash').notNull().unique(),
  keyPrefix: text('key_prefix').notNull(),
  createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  lastUsedAt: timestamp('last_used_at', { withTimezone: true }),
  revokedAt: timestamp('revoked_at', { withTimezone: true }),
})

export const auditLog = platformSchema.table('audit_log', {
  id: uuid('id').primaryKey().defaultRandom(),
  orgId: uuid('org_id'),
  actorType: actorTypeEnum('actor_type').notNull(),
  actorId: uuid('actor_id'),
  event: text('event').notNull(),
  resourceType: text('resource_type'),
  resourceId: uuid('resource_id'),
  metadata: jsonb('metadata'),
  ipAddress: text('ip_address'),
  occurredAt: timestamp('occurred_at', { withTimezone: true }).notNull().defaultNow(),
})
```

- [ ] **Step 8: Create `src/db/tenant/schema.ts`**

```typescript
import {
  pgSchema,
  uuid,
  text,
  timestamp,
  integer,
  real,
  pgEnum,
} from 'drizzle-orm/pg-core'

const sourceEnum = (schema: ReturnType<typeof pgSchema>) =>
  schema.enum('source', ['local', 'ci'])

const statusEnum = (schema: ReturnType<typeof pgSchema>) =>
  schema.enum('stack_status_enum', ['clean', 'changes', 'error', 'unknown'])

export function tenantTables(schemaName: string) {
  const schema = pgSchema(schemaName)
  const source = sourceEnum(schema)
  const status = statusEnum(schema)

  const projects = schema.table('projects', {
    id: uuid('id').primaryKey().defaultRandom(),
    projectId: text('project_id').notNull().unique(),
    name: text('name').notNull(),
    repoUrl: text('repo_url'),
    rootConfigFile: text('root_config_file').notNull().default('root.hcl'),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull().defaultNow(),
  })

  const executions = schema.table('executions', {
    id: uuid('id').primaryKey().defaultRandom(),
    projectId: text('project_id').notNull(),
    stackPath: text('stack_path').notNull(),
    command: text('command').notNull(),
    exitCode: integer('exit_code').notNull(),
    durationS: real('duration_s').notNull(),
    driftAdded: integer('drift_added'),
    driftChanged: integer('drift_changed'),
    driftDestroyed: integer('drift_destroyed'),
    executedBy: text('executed_by').notNull(),
    source: source('source').notNull(),
    commitSha: text('commit_sha'),
    branch: text('branch'),
    ciRunUrl: text('ci_run_url'),
    apiKeyId: uuid('api_key_id'),
    executedAt: timestamp('executed_at', { withTimezone: true }).notNull().defaultNow(),
  })

  const stackStatus = schema.table('stack_status', {
    projectId: text('project_id').notNull(),
    stackPath: text('stack_path').notNull(),
    status: status('status').notNull().default('unknown'),
    driftAdded: integer('drift_added').notNull().default(0),
    driftChanged: integer('drift_changed').notNull().default(0),
    driftDestroyed: integer('drift_destroyed').notNull().default(0),
    lastExecutionId: uuid('last_execution_id'),
    plannedAt: timestamp('planned_at', { withTimezone: true }),
    plannedBy: text('planned_by'),
    plannedSource: source('planned_source'),
  })

  return { projects, executions, stackStatus }
}
```

- [ ] **Step 9: Create `src/db/client.ts`**

```typescript
import { drizzle } from 'drizzle-orm/postgres-js'
import postgres from 'postgres'

const connectionString = process.env.DATABASE_URL
if (!connectionString) {
  throw new Error('DATABASE_URL environment variable is required.')
}

const sql = postgres(connectionString)
export const db = drizzle(sql)
```

- [ ] **Step 10: Create `atlas/platform.hcl`**

```hcl
schema "platform" {}

table "organizations" {
  schema = schema.platform
  column "id" { type = uuid  null = false }
  column "slug" { type = text null = false }
  column "name" { type = text null = false }
  column "plan" { type = text null = false default = "free" }
  column "schema_name" { type = text null = false }
  column "created_at" { type = timestamptz null = false default = sql("now()") }
  primary_key { columns = [column.id] }
  index "organizations_slug_key" { columns = [column.slug] unique = true }
}

table "api_keys" {
  schema = schema.platform
  column "id" { type = uuid null = false }
  column "org_id" { type = uuid null = false }
  column "name" { type = text null = false }
  column "key_hash" { type = text null = false }
  column "key_prefix" { type = text null = false }
  column "created_at" { type = timestamptz null = false default = sql("now()") }
  column "last_used_at" { type = timestamptz null = true }
  column "revoked_at" { type = timestamptz null = true }
  primary_key { columns = [column.id] }
  index "api_keys_key_hash_key" { columns = [column.key_hash] unique = true }
  foreign_key "api_keys_org_id_fkey" {
    columns     = [column.org_id]
    ref_columns = [table.organizations.column.id]
    on_delete   = NO_ACTION
  }
}

table "audit_log" {
  schema = schema.platform
  column "id" { type = uuid null = false }
  column "org_id" { type = uuid null = true }
  column "actor_type" { type = text null = false }
  column "actor_id" { type = uuid null = true }
  column "event" { type = text null = false }
  column "resource_type" { type = text null = true }
  column "resource_id" { type = uuid null = true }
  column "metadata" { type = jsonb null = true }
  column "ip_address" { type = text null = true }
  column "occurred_at" { type = timestamptz null = false default = sql("now()") }
  primary_key { columns = [column.id] }
}
```

- [ ] **Step 11: Create `atlas/tenant.hcl`** (template applied to each tenant schema)

```hcl
# Atlas tenant schema template.
# Applied once per organization using the tenant's schema name.
# Variable TENANT_SCHEMA is substituted at apply time.

variable "TENANT_SCHEMA" {
  type = string
}

schema "${var.TENANT_SCHEMA}" {}

table "projects" {
  schema = schema["${var.TENANT_SCHEMA}"]
  column "id" { type = uuid null = false }
  column "project_id" { type = text null = false }
  column "name" { type = text null = false }
  column "repo_url" { type = text null = true }
  column "root_config_file" { type = text null = false default = "root.hcl" }
  column "created_at" { type = timestamptz null = false default = sql("now()") }
  primary_key { columns = [column.id] }
  index "projects_project_id_key" { columns = [column.project_id] unique = true }
}

table "executions" {
  schema = schema["${var.TENANT_SCHEMA}"]
  column "id" { type = uuid null = false }
  column "project_id" { type = text null = false }
  column "stack_path" { type = text null = false }
  column "command" { type = text null = false }
  column "exit_code" { type = int null = false }
  column "duration_s" { type = float null = false }
  column "drift_added" { type = int null = true }
  column "drift_changed" { type = int null = true }
  column "drift_destroyed" { type = int null = true }
  column "executed_by" { type = text null = false }
  column "source" { type = text null = false }
  column "commit_sha" { type = text null = true }
  column "branch" { type = text null = true }
  column "ci_run_url" { type = text null = true }
  column "api_key_id" { type = uuid null = true }
  column "executed_at" { type = timestamptz null = false default = sql("now()") }
  primary_key { columns = [column.id] }
}

table "stack_status" {
  schema = schema["${var.TENANT_SCHEMA}"]
  column "project_id" { type = text null = false }
  column "stack_path" { type = text null = false }
  column "status" { type = text null = false default = "unknown" }
  column "drift_added" { type = int null = false default = 0 }
  column "drift_changed" { type = int null = false default = 0 }
  column "drift_destroyed" { type = int null = false default = 0 }
  column "last_execution_id" { type = uuid null = true }
  column "planned_at" { type = timestamptz null = true }
  column "planned_by" { type = text null = true }
  column "planned_source" { type = text null = true }
  primary_key { columns = [column.project_id, column.stack_path] }
}
```

- [ ] **Step 12: Create `src/index.ts`**

```typescript
import { serve } from '@hono/node-server'
import { Hono } from 'hono'

const app = new Hono()

app.get('/health', (c) => c.json({ status: 'ok' }))

const port = parseInt(process.env.PORT ?? '3000', 10)
serve({ fetch: app.fetch, port }, () => {
  console.log(`terrax-api listening on port ${port}`)
})

export default app
```

- [ ] **Step 13: Start PostgreSQL and verify app runs**

```bash
cp .env.example .env
docker compose up db -d
pnpm dev
```

Expected: `terrax-api listening on port 3000`

```bash
curl http://localhost:3000/health
```

Expected: `{"status":"ok"}`

- [ ] **Step 14: Apply platform migrations**

```bash
# Apply platform schema to running PostgreSQL
atlas schema apply \
  --url "postgres://postgres:password@localhost:5432/terrax?search_path=public&sslmode=disable" \
  --to "file://atlas/platform.hcl" \
  --dev-url "postgres://postgres:password@localhost:5432/terrax_dev?sslmode=disable" \
  --auto-approve
```

Expected: Atlas creates the `platform` schema with `organizations`, `api_keys`, `audit_log` tables.

- [ ] **Step 15: Initial commit**

```bash
git add .
git commit -m "chore: scaffold terrax-api with Hono, Drizzle, Atlas, Docker Compose"
```

---

### Task 2: API key library + auth middleware

**Files:**
- Create: `src/lib/keys.ts`
- Create: `src/middleware/auth.ts`
- Create: `src/test/auth.test.ts`

**Interfaces:**
- Consumes: `db` from `../db/client`
- Consumes: `organizations`, `apiKeys` from `../db/platform/schema`
- Produces: `generateApiKey(): { key: string; hash: string; prefix: string }`
- Produces: `hashApiKey(key: string): string`
- Produces: `authMiddleware` — Hono middleware that sets `orgId`, `tenantSchema`, `apiKeyId` on context

- [ ] **Step 1: Write failing tests in `src/test/auth.test.ts`**

```typescript
import { describe, it, expect } from 'vitest'
import { generateApiKey, hashApiKey } from '../lib/keys'

describe('generateApiKey', () => {
  it('returns a key starting with tx_sk_', () => {
    const { key } = generateApiKey()
    expect(key).toMatch(/^tx_sk_[a-f0-9]{64}$/)
  })

  it('prefix is first 12 characters of key', () => {
    const { key, prefix } = generateApiKey()
    expect(prefix).toBe(key.substring(0, 12))
  })

  it('hash is deterministic SHA-256 of key', () => {
    const { key, hash } = generateApiKey()
    expect(hashApiKey(key)).toBe(hash)
  })

  it('two calls produce different keys', () => {
    const a = generateApiKey()
    const b = generateApiKey()
    expect(a.key).not.toBe(b.key)
    expect(a.hash).not.toBe(b.hash)
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
pnpm test
```

Expected: FAIL — `generateApiKey` not defined.

- [ ] **Step 3: Create `src/lib/keys.ts`**

```typescript
import { randomBytes, createHash } from 'node:crypto'

export function generateApiKey(): { key: string; hash: string; prefix: string } {
  const raw = randomBytes(32).toString('hex')
  const key = `tx_sk_${raw}`
  const hash = createHash('sha256').update(key).digest('hex')
  const prefix = key.substring(0, 12)
  return { key, hash, prefix }
}

export function hashApiKey(key: string): string {
  return createHash('sha256').update(key).digest('hex')
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
pnpm test
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Create `src/middleware/auth.ts`**

```typescript
import { createMiddleware } from 'hono/factory'
import { eq, isNull, and } from 'drizzle-orm'
import { db } from '../db/client'
import { organizations, apiKeys } from '../db/platform/schema'
import { hashApiKey } from '../lib/keys'

type AuthVariables = {
  orgId: string
  tenantSchema: string
  apiKeyId: string
}

export const authMiddleware = createMiddleware<{ Variables: AuthVariables }>(
  async (c, next) => {
    const rawKey = c.req.header('X-API-Key')
    if (!rawKey) {
      return c.json({ error: 'Missing X-API-Key header.' }, 401)
    }

    const keyHash = hashApiKey(rawKey)

    const [keyRecord] = await db
      .select()
      .from(apiKeys)
      .where(and(eq(apiKeys.keyHash, keyHash), isNull(apiKeys.revokedAt)))
      .limit(1)

    if (!keyRecord) {
      return c.json({ error: 'Invalid or revoked API key.' }, 401)
    }

    const [org] = await db
      .select()
      .from(organizations)
      .where(eq(organizations.id, keyRecord.orgId))
      .limit(1)

    if (!org) {
      return c.json({ error: 'Organization not found.' }, 401)
    }

    // Update last_used_at — fire and forget, never blocks the request.
    db.update(apiKeys)
      .set({ lastUsedAt: new Date() })
      .where(eq(apiKeys.id, keyRecord.id))
      .execute()
      .catch(() => {})

    c.set('orgId', org.id)
    c.set('tenantSchema', org.schemaName)
    c.set('apiKeyId', keyRecord.id)

    await next()
  }
)
```

- [ ] **Step 6: Commit**

```bash
git add src/lib/keys.ts src/middleware/auth.ts src/test/auth.test.ts
git commit -m "feat: add API key generation and auth middleware"
```

---

### Task 3: Organization management + API key endpoints

**Files:**
- Create: `src/lib/tenant.ts`
- Create: `src/routes/organizations.ts`
- Create: `src/routes/api-keys.ts`
- Modify: `src/index.ts`

**Interfaces:**
- Consumes: `db`, `organizations`, `apiKeys` from platform schema
- Consumes: `generateApiKey` from `../lib/keys`
- Produces: `POST /v1/organizations`, `POST/GET/DELETE /v1/organizations/:id/api-keys`, `POST/GET /v1/organizations/:id/projects`

- [ ] **Step 1: Create `src/lib/tenant.ts`**

Responsible for provisioning a new tenant schema in PostgreSQL and applying the Atlas tenant migrations.

```typescript
import { execSync } from 'node:child_process'

export function slugToSchemaName(slug: string): string {
  return `tenant_${slug.toLowerCase().replace(/[^a-z0-9]/g, '_')}`
}

export async function provisionTenantSchema(schemaName: string): Promise<void> {
  const dbUrl = process.env.DATABASE_URL
  const devUrl = process.env.DEV_DATABASE_URL
  if (!dbUrl || !devUrl) {
    throw new Error('DATABASE_URL and DEV_DATABASE_URL must be set.')
  }

  // Atlas applies the tenant template with the schema name substituted.
  execSync(
    [
      'atlas schema apply',
      `--url "${dbUrl}?search_path=public&sslmode=disable"`,
      '--to "file://atlas/tenant.hcl"',
      `--dev-url "${devUrl}?sslmode=disable"`,
      `--var TENANT_SCHEMA=${schemaName}`,
      '--auto-approve',
    ].join(' '),
    { stdio: 'inherit' }
  )
}
```

- [ ] **Step 2: Create `src/routes/organizations.ts`**

```typescript
import { Hono } from 'hono'
import { eq } from 'drizzle-orm'
import { db } from '../db/client'
import { organizations, apiKeys } from '../db/platform/schema'
import { tenantTables } from '../db/tenant/schema'
import { slugToSchemaName, provisionTenantSchema } from '../lib/tenant'
import { generateApiKey } from '../lib/keys'

export const organizationsRouter = new Hono()

// Create organization — no auth required (signup endpoint).
organizationsRouter.post('/', async (c) => {
  const body = await c.req.json<{ name: string; slug: string }>()

  if (!body.name || !body.slug) {
    return c.json({ error: 'name and slug are required.' }, 400)
  }

  const slugRegex = /^[a-z0-9-]+$/
  if (!slugRegex.test(body.slug)) {
    return c.json({ error: 'slug must contain only lowercase letters, numbers, and hyphens.' }, 400)
  }

  const schemaName = slugToSchemaName(body.slug)

  const [existing] = await db
    .select()
    .from(organizations)
    .where(eq(organizations.slug, body.slug))
    .limit(1)

  if (existing) {
    return c.json({ error: 'Organization slug already taken.' }, 409)
  }

  const [org] = await db
    .insert(organizations)
    .values({ name: body.name, slug: body.slug, schemaName })
    .returning()

  // Provision the tenant PostgreSQL schema.
  try {
    await provisionTenantSchema(schemaName)
  } catch (err) {
    // Roll back the org record if schema provisioning fails.
    await db.delete(organizations).where(eq(organizations.id, org.id))
    throw err
  }

  return c.json({ id: org.id, slug: org.slug, schemaName: org.schemaName }, 201)
})

// Register a project within an org (requires auth — mounted with authMiddleware).
organizationsRouter.post('/:orgId/projects', async (c) => {
  const { orgId, tenantSchema } = c.var as { orgId: string; tenantSchema: string }
  const paramOrgId = c.req.param('orgId')

  if (paramOrgId !== orgId) {
    return c.json({ error: 'Forbidden.' }, 403)
  }

  const body = await c.req.json<{
    project_id: string
    name: string
    repo_url?: string
    root_config_file?: string
  }>()

  if (!body.project_id || !body.name) {
    return c.json({ error: 'project_id and name are required.' }, 400)
  }

  const { projects } = tenantTables(tenantSchema)

  const [project] = await db
    .insert(projects)
    .values({
      projectId: body.project_id,
      name: body.name,
      repoUrl: body.repo_url ?? null,
      rootConfigFile: body.root_config_file ?? 'root.hcl',
    })
    .returning()

  return c.json({ id: project.id, project_id: project.projectId }, 201)
})

organizationsRouter.get('/:orgId/projects', async (c) => {
  const { orgId, tenantSchema } = c.var as { orgId: string; tenantSchema: string }
  const paramOrgId = c.req.param('orgId')

  if (paramOrgId !== orgId) {
    return c.json({ error: 'Forbidden.' }, 403)
  }

  const { projects } = tenantTables(tenantSchema)
  const rows = await db.select().from(projects)

  return c.json(rows)
})
```

- [ ] **Step 3: Create `src/routes/api-keys.ts`**

```typescript
import { Hono } from 'hono'
import { eq, and, isNull } from 'drizzle-orm'
import { db } from '../db/client'
import { apiKeys } from '../db/platform/schema'
import { generateApiKey } from '../lib/keys'

export const apiKeysRouter = new Hono()

// Generate a new API key — key value returned ONCE.
apiKeysRouter.post('/:orgId/api-keys', async (c) => {
  const { orgId } = c.var as { orgId: string }
  const paramOrgId = c.req.param('orgId')

  if (paramOrgId !== orgId) {
    return c.json({ error: 'Forbidden.' }, 403)
  }

  const body = await c.req.json<{ name: string }>()
  if (!body.name) {
    return c.json({ error: 'name is required.' }, 400)
  }

  const { key, hash, prefix } = generateApiKey()

  const [record] = await db
    .insert(apiKeys)
    .values({ orgId, name: body.name, keyHash: hash, keyPrefix: prefix })
    .returning()

  return c.json({ id: record.id, name: record.name, key, key_prefix: prefix }, 201)
})

// List keys — never returns the key value, only prefix.
apiKeysRouter.get('/:orgId/api-keys', async (c) => {
  const { orgId } = c.var as { orgId: string }
  const paramOrgId = c.req.param('orgId')

  if (paramOrgId !== orgId) {
    return c.json({ error: 'Forbidden.' }, 403)
  }

  const rows = await db
    .select({
      id: apiKeys.id,
      name: apiKeys.name,
      key_prefix: apiKeys.keyPrefix,
      created_at: apiKeys.createdAt,
      last_used_at: apiKeys.lastUsedAt,
      revoked_at: apiKeys.revokedAt,
    })
    .from(apiKeys)
    .where(eq(apiKeys.orgId, orgId))

  return c.json(rows)
})

// Revoke a key by setting revoked_at.
apiKeysRouter.delete('/:orgId/api-keys/:keyId', async (c) => {
  const { orgId } = c.var as { orgId: string }
  const paramOrgId = c.req.param('orgId')
  const keyId = c.req.param('keyId')

  if (paramOrgId !== orgId) {
    return c.json({ error: 'Forbidden.' }, 403)
  }

  const result = await db
    .update(apiKeys)
    .set({ revokedAt: new Date() })
    .where(and(eq(apiKeys.id, keyId), eq(apiKeys.orgId, orgId), isNull(apiKeys.revokedAt)))
    .returning()

  if (result.length === 0) {
    return c.json({ error: 'API key not found or already revoked.' }, 404)
  }

  return new Response(null, { status: 204 })
})
```

- [ ] **Step 4: Wire routes into `src/index.ts`**

The org signup (`POST /v1/organizations`) is public. All other routes require auth.
`organizationsRouter` handles project management (`/:orgId/projects`) — only mounted under the authenticated prefix.

```typescript
import { serve } from '@hono/node-server'
import { Hono } from 'hono'
import { authMiddleware } from './middleware/auth'
import { organizationsRouter } from './routes/organizations'
import { apiKeysRouter } from './routes/api-keys'
import { db } from './db/client'
import { organizations } from './db/platform/schema'
import { slugToSchemaName, provisionTenantSchema } from './lib/tenant'
import { eq } from 'drizzle-orm'

const app = new Hono()

app.get('/health', (c) => c.json({ status: 'ok' }))

// Public: org signup only.
app.post('/v1/organizations', async (c) => {
  const body = await c.req.json<{ name: string; slug: string }>()
  if (!body.name || !body.slug) return c.json({ error: 'name and slug are required.' }, 400)
  const slugRegex = /^[a-z0-9-]+$/
  if (!slugRegex.test(body.slug)) return c.json({ error: 'Invalid slug format.' }, 400)
  const schemaName = slugToSchemaName(body.slug)
  const [existing] = await db.select().from(organizations).where(eq(organizations.slug, body.slug)).limit(1)
  if (existing) return c.json({ error: 'Organization slug already taken.' }, 409)
  const [org] = await db.insert(organizations).values({ name: body.name, slug: body.slug, schemaName }).returning()
  try {
    await provisionTenantSchema(schemaName)
  } catch (err) {
    await db.delete(organizations).where(eq(organizations.id, org.id))
    throw err
  }
  return c.json({ id: org.id, slug: org.slug, schemaName: org.schemaName }, 201)
})

// Authenticated routes.
const api = new Hono()
api.use('*', authMiddleware)
api.route('/organizations', organizationsRouter)  // /:orgId/projects
api.route('/organizations', apiKeysRouter)         // /:orgId/api-keys

app.route('/v1', api)

const port = parseInt(process.env.PORT ?? '3000', 10)
serve({ fetch: app.fetch, port }, () => {
  console.log(`terrax-api listening on port ${port}`)
})

export default app
```

Also remove the `organizationsRouter.post('/')` handler — that logic now lives in index.ts above.

- [ ] **Step 5: Restart and smoke test**

```bash
pnpm dev
```

```bash
# Create an org
curl -X POST http://localhost:3000/v1/organizations \
  -H "Content-Type: application/json" \
  -d '{"name": "Acme Corp", "slug": "acme"}'
```

Expected: `{"id":"...","slug":"acme","schemaName":"tenant_acme"}`

```bash
# Generate an API key (use the org ID from previous response)
curl -X POST http://localhost:3000/v1/organizations/<orgId>/api-keys \
  -H "Content-Type: application/json" \
  -H "X-API-Key: <any-valid-key>" \
  -d '{"name": "local-dev"}'
```

- [ ] **Step 6: Commit**

```bash
git add src/lib/tenant.ts src/routes/organizations.ts src/routes/api-keys.ts src/index.ts
git commit -m "feat: add organization management and API key endpoints"
```

---

### Task 4: Execution reporting

**Files:**
- Create: `src/routes/executions.ts`
- Create: `src/test/executions.test.ts`
- Modify: `src/index.ts`

**Interfaces:**
- Consumes: `tenantTables(schemaName)` from tenant schema
- Produces: `POST /v1/executions` → `{ id, created_at }`
- Produces: `POST /v1/executions/batch` → `{ created: number, ids: string[] }`
- Side-effect: upserts `stack_status` when `command === "plan"`

- [ ] **Step 1: Create `src/routes/executions.ts`**

```typescript
import { Hono } from 'hono'
import { db } from '../db/client'
import { tenantTables } from '../db/tenant/schema'

type ExecutionBody = {
  project_id: string
  stack_path: string
  command: string
  exit_code: number
  duration_s: number
  drift_added?: number
  drift_changed?: number
  drift_destroyed?: number
  executed_by: string
  source: 'local' | 'ci'
  commit_sha?: string
  branch?: string
  ci_run_url?: string
}

async function upsertStackStatus(
  tenantSchema: string,
  projectId: string,
  stackPath: string,
  exec: {
    id: string
    exitCode: number
    driftAdded: number | null
    driftChanged: number | null
    driftDestroyed: number | null
    executedBy: string
    source: 'local' | 'ci'
  }
): Promise<void> {
  const { stackStatus } = tenantTables(tenantSchema)

  const status =
    exec.exitCode !== 0
      ? 'error'
      : (exec.driftAdded ?? 0) + (exec.driftChanged ?? 0) + (exec.driftDestroyed ?? 0) > 0
      ? 'changes'
      : 'clean'

  await db
    .insert(stackStatus)
    .values({
      projectId,
      stackPath,
      status,
      driftAdded: exec.driftAdded ?? 0,
      driftChanged: exec.driftChanged ?? 0,
      driftDestroyed: exec.driftDestroyed ?? 0,
      lastExecutionId: exec.id,
      plannedAt: new Date(),
      plannedBy: exec.executedBy,
      plannedSource: exec.source,
    })
    .onConflictDoUpdate({
      target: [stackStatus.projectId, stackStatus.stackPath],
      set: {
        status,
        driftAdded: exec.driftAdded ?? 0,
        driftChanged: exec.driftChanged ?? 0,
        driftDestroyed: exec.driftDestroyed ?? 0,
        lastExecutionId: exec.id,
        plannedAt: new Date(),
        plannedBy: exec.executedBy,
        plannedSource: exec.source,
      },
    })
}

export const executionsRouter = new Hono()

executionsRouter.post('/', async (c) => {
  const { tenantSchema, apiKeyId } = c.var as { tenantSchema: string; apiKeyId: string }
  const body = await c.req.json<ExecutionBody>()

  if (!body.project_id || !body.stack_path || !body.command || body.exit_code === undefined) {
    return c.json({ error: 'project_id, stack_path, command, and exit_code are required.' }, 400)
  }

  const { executions } = tenantTables(tenantSchema)

  const [record] = await db
    .insert(executions)
    .values({
      projectId: body.project_id,
      stackPath: body.stack_path,
      command: body.command,
      exitCode: body.exit_code,
      durationS: body.duration_s,
      driftAdded: body.drift_added ?? null,
      driftChanged: body.drift_changed ?? null,
      driftDestroyed: body.drift_destroyed ?? null,
      executedBy: body.executed_by,
      source: body.source,
      commitSha: body.commit_sha ?? null,
      branch: body.branch ?? null,
      ciRunUrl: body.ci_run_url ?? null,
      apiKeyId,
    })
    .returning()

  if (body.command === 'plan') {
    await upsertStackStatus(tenantSchema, body.project_id, body.stack_path, {
      id: record.id,
      exitCode: record.exitCode,
      driftAdded: record.driftAdded,
      driftChanged: record.driftChanged,
      driftDestroyed: record.driftDestroyed,
      executedBy: record.executedBy,
      source: record.source as 'local' | 'ci',
    })
  }

  return c.json({ id: record.id, created_at: record.executedAt }, 201)
})

executionsRouter.post('/batch', async (c) => {
  const { tenantSchema, apiKeyId } = c.var as { tenantSchema: string; apiKeyId: string }

  const body = await c.req.json<{
    project_id: string
    command: string
    source: 'local' | 'ci'
    branch?: string
    commit_sha?: string
    stacks: Array<{
      stack_path: string
      exit_code: number
      duration_s: number
      drift_added?: number
      drift_changed?: number
      drift_destroyed?: number
      executed_by: string
    }>
  }>()

  if (!body.project_id || !body.command || !body.stacks?.length) {
    return c.json({ error: 'project_id, command, and stacks are required.' }, 400)
  }

  const { executions } = tenantTables(tenantSchema)

  const records = await db
    .insert(executions)
    .values(
      body.stacks.map((s) => ({
        projectId: body.project_id,
        stackPath: s.stack_path,
        command: body.command,
        exitCode: s.exit_code,
        durationS: s.duration_s,
        driftAdded: s.drift_added ?? null,
        driftChanged: s.drift_changed ?? null,
        driftDestroyed: s.drift_destroyed ?? null,
        executedBy: s.executed_by,
        source: body.source,
        commitSha: body.commit_sha ?? null,
        branch: body.branch ?? null,
        apiKeyId,
      }))
    )
    .returning()

  if (body.command === 'plan') {
    await Promise.all(
      records.map((r) =>
        upsertStackStatus(tenantSchema, body.project_id, r.stackPath, {
          id: r.id,
          exitCode: r.exitCode,
          driftAdded: r.driftAdded,
          driftChanged: r.driftChanged,
          driftDestroyed: r.driftDestroyed,
          executedBy: r.executedBy,
          source: r.source as 'local' | 'ci',
        })
      )
    )
  }

  return c.json({ created: records.length, ids: records.map((r) => r.id) }, 201)
})
```

- [ ] **Step 2: Wire executions route into `src/index.ts`**

Add after existing route registrations:

```typescript
import { executionsRouter } from './routes/executions'

// inside the authenticated `api` Hono instance:
api.route('/executions', executionsRouter)
```

- [ ] **Step 3: Restart and smoke test**

```bash
pnpm dev
# POST a single plan execution (replace with real API key and org values)
curl -X POST http://localhost:3000/v1/executions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: tx_sk_..." \
  -d '{
    "project_id": "my-project",
    "stack_path": "workloads/dev/vpc",
    "command": "plan",
    "exit_code": 0,
    "duration_s": 12.3,
    "drift_added": 2,
    "drift_changed": 0,
    "drift_destroyed": 0,
    "executed_by": "isra",
    "source": "local"
  }'
```

Expected: `{"id":"...","created_at":"..."}`

- [ ] **Step 4: Commit**

```bash
git add src/routes/executions.ts src/index.ts
git commit -m "feat: add execution reporting with stack_status upsert"
```

---

### Task 5: Stack status queries

**Files:**
- Create: `src/routes/stacks.ts`
- Modify: `src/index.ts`

**Interfaces:**
- Consumes: `tenantTables(schemaName)` + `executions` table
- Produces: `GET /v1/projects/:project_id/stacks` → `StackStatus[]`
- Produces: `GET /v1/projects/:project_id/stacks?path=<encoded>` → `StackStatus & { last_executions: [] }`

- [ ] **Step 1: Create `src/routes/stacks.ts`**

```typescript
import { Hono } from 'hono'
import { eq, and, desc } from 'drizzle-orm'
import { db } from '../db/client'
import { tenantTables } from '../db/tenant/schema'

export const stacksRouter = new Hono()

stacksRouter.get('/:projectId/stacks', async (c) => {
  const { tenantSchema } = c.var as { tenantSchema: string }
  const projectId = c.req.param('projectId')
  const stackPath = c.req.query('path')

  const { stackStatus, executions } = tenantTables(tenantSchema)

  if (stackPath) {
    // Single stack with recent executions.
    const [status] = await db
      .select()
      .from(stackStatus)
      .where(and(eq(stackStatus.projectId, projectId), eq(stackStatus.stackPath, stackPath)))
      .limit(1)

    const recentExecutions = await db
      .select()
      .from(executions)
      .where(and(eq(executions.projectId, projectId), eq(executions.stackPath, stackPath)))
      .orderBy(desc(executions.executedAt))
      .limit(5)

    if (!status) {
      return c.json({
        project_id: projectId,
        stack_path: stackPath,
        status: 'unknown',
        drift_added: 0,
        drift_changed: 0,
        drift_destroyed: 0,
        planned_at: null,
        planned_by: null,
        planned_source: null,
        last_executions: recentExecutions,
      })
    }

    return c.json({ ...status, last_executions: recentExecutions })
  }

  // All stacks for this project.
  const rows = await db
    .select()
    .from(stackStatus)
    .where(eq(stackStatus.projectId, projectId))

  return c.json(rows)
})
```

- [ ] **Step 2: Wire route into `src/index.ts`**

```typescript
import { stacksRouter } from './routes/stacks'

// inside the authenticated `api` Hono instance:
api.route('/projects', stacksRouter)
```

- [ ] **Step 3: Smoke test**

```bash
# List all stacks (should see the vpc stack from Task 4 smoke test)
curl http://localhost:3000/v1/projects/my-project/stacks \
  -H "X-API-Key: tx_sk_..."
```

Expected: JSON array with the vpc stack status showing `status: "changes"`.

- [ ] **Step 4: Commit**

```bash
git add src/routes/stacks.ts src/index.ts
git commit -m "feat: add stack status query endpoints"
```

---

### Task 6: Audit middleware

**Files:**
- Create: `src/middleware/audit.ts`
- Modify: `src/index.ts`

**Interfaces:**
- Consumes: `db`, `auditLog` from platform schema
- Produces: automatic audit log entry for every authenticated request with method, path, status code, and actor info

- [ ] **Step 1: Create `src/middleware/audit.ts`**

```typescript
import { createMiddleware } from 'hono/factory'
import { db } from '../db/client'
import { auditLog } from '../db/platform/schema'

export const auditMiddleware = createMiddleware(async (c, next) => {
  await next()

  const orgId = (c.var as Record<string, string>).orgId
  const apiKeyId = (c.var as Record<string, string>).apiKeyId

  if (!orgId) return // unauthenticated request — skip audit

  const method = c.req.method
  const path = c.req.path
  const status = c.res.status

  const event = `${method.toLowerCase()}.${path.replace(/\//g, '.').replace(/^\./, '')}`

  // Fire and forget — never delays the response.
  db.insert(auditLog)
    .values({
      orgId,
      actorType: 'api_key',
      actorId: apiKeyId ?? null,
      event,
      metadata: { status, path, method },
      ipAddress: c.req.header('x-forwarded-for') ?? c.req.header('x-real-ip') ?? null,
    })
    .execute()
    .catch(() => {})
})
```

- [ ] **Step 2: Add audit middleware to authenticated routes in `src/index.ts`**

```typescript
import { auditMiddleware } from './middleware/audit'

// Add after authMiddleware:
api.use('*', authMiddleware)
api.use('*', auditMiddleware)
```

- [ ] **Step 3: Smoke test**

```bash
# Make an authenticated request
curl http://localhost:3000/v1/projects/my-project/stacks \
  -H "X-API-Key: tx_sk_..."

# Check audit log in PostgreSQL
docker compose exec db psql -U postgres terrax \
  -c "SELECT event, occurred_at FROM platform.audit_log ORDER BY occurred_at DESC LIMIT 5;"
```

Expected: rows in `audit_log` for each request made.

- [ ] **Step 4: Final commit**

```bash
git add src/middleware/audit.ts src/index.ts
git commit -m "feat: add audit middleware for authenticated request logging"
```
