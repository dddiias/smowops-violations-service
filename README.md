# Snowops Violations Service

Snowops Violations Service implements EPIC 7 requirements: tracking trip violations, managing multi-role appeals, storing attachments/comments, and synchronizing violation statuses with appeal decisions. The service follows the same Go + Gin + GORM stack as other Snowops microservices and plugs into the shared PostgreSQL schema (`trips`, `tickets`, `drivers`, `vehicles`, `organizations`, `cleaning_areas`, etc.).

## Highlights

- **Violation registry** – `violations` table stores per-trip issues with type, severity, detected source (LPR/VOLUME/GPS/SYSTEM) and lifecycle (`OPEN`, `CANCELED`, `FIXED`). Manual violations can be created by KGU/Akimat.
- **Appeal workflow** – `violation_appeals`, `*_attachments`, `*_comments` tables capture submissions, evidence and threaded discussion history per violation.
- **Role-aware API** – JWT principal determines scope:
  - `AKIMAT_ADMIN`: full read/write, status overrides.
  - `KGU_ZKH_ADMIN`: manages contractors in its hierarchy, creates violations, resolves appeals.
  - `CONTRACTOR_ADMIN`: sees own trips, files/answers appeals, uploads evidence.
  - `DRIVER`: sees own trips, files appeals, responds to `NEED_INFO`.
  - `TOO_ADMIN`: sees only camera-related violations (detected_by = LPR/VOLUME/SYSTEM with CAMERA_ERROR appeals), can comment for diagnostics.
- **Lifecycle enforcement** – one active appeal per violation; transitions follow PDF spec (SUBMITTED→UNDER_REVIEW→NEED_INFO/APPROVED/REJECTED→CLOSED). Approvals cancel violations, rejections fix them.
- **Attachment guardrails** – configurable max attachments per action, strict enum for file types (IMAGE/VIDEO/DOC).
- **Automation + audit** – DB triggers create violations automatically when `trips.status != 'OK'`, populate `trip.violation_reason`, and log every violation/appeal status change in dedicated history tables.

## Database objects

Migrations (`internal/db/migrations.go`) provision:

- Enums: `violation_status`, `violation_severity`, `violation_detected_by`, `appeal_status`, `appeal_reason_code`, `attachment_file_type`.
- `violations`: FK to `trips`, type/detected_by/severity/status/description, timestamps + indexes.
- `violation_appeals`: FK to `violations`, `trips`, `tickets`, `drivers`, `organizations`, lifecycle fields, partial unique index forbidding multiple active appeals.
- `violation_appeal_attachments` & `violation_appeal_comments`.
- `trips.violation_reason` column addition so ticket-service can keep a human-readable reason.

All statements are idempotent for shared-schema usage.

## API surface

All endpoints require `Authorization: Bearer <jwt>` issued by snowops-auth-service.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/violations` | List violations (filters: status/type/severity/detected_by/contractor/driver/ticket/area/date/search). Scope auto-applied. |
| `GET` | `/violations/:id` | Detailed card (trip/ticket/context + full appeal history). |
| `POST` | `/violations` | KGU/Akimat manual violation creation (body: `trip_id`, `type`, `detected_by`, `severity`, `description`). |
| `PUT` | `/violations/:id/status` | KGU/Akimat mark as `FIXED` or `CANCELED`. |
| `GET` | `/appeals` | List appeals (filters: status, reason_code, violation_type, contractor, date). Technical users auto-filtered to CAMERA_ERROR. |
| `GET` | `/appeals/:id` | Appeal card with attachments/comments. |
| `POST` | `/violations/:id/appeals` | Driver/contractor submit appeal (`reason_code`, `reason_text`, attachments). |
| `POST` | `/appeals/:id/comments` | Participants add comment + attachments. Driver/contractor replies from `NEED_INFO` return status to `UNDER_REVIEW`. |
| `POST` | `/appeals/:id/actions` | KGU/Akimat actions: `UNDER_REVIEW`, `NEED_INFO`, `APPROVE`, `REJECT`, `CLOSE`. Approve→violation CANCELED, Reject→violation FIXED. |

Responses follow `{ "data": ... }` envelope. Errors use `{ "error": "<message>" }`.

## Quick start

```bash
# start postgres (uses postgis image for geometry compatibility)
cd deploy
docker compose up -d

# run service
cd ..
APP_ENV=development \
DB_DSN="postgres://postgres:postgres@localhost:5445/violations_db?sslmode=disable" \
JWT_ACCESS_SECRET="secret" \
go run ./cmd/violation-service
```

### Local verification / demo data

If you want to run the service standalone (without the rest of Snowops) use the helper scripts:

```bash
# assuming Postgres from deploy/ is running
cd snowops-violations-service
psql "postgres://postgres:postgres@localhost:5445/violations_db?sslmode=disable" -f scripts/demo_schema.sql
psql "postgres://postgres:postgres@localhost:5445/violations_db?sslmode=disable" -f scripts/demo_seed.sql

APP_ENV=development \
JWT_ACCESS_SECRET="secret" \
go run ./cmd/violation-service
```

The seed updates a demo trip status to `ROUTE_VIOLATION`, which fires the DB trigger and creates an `OPEN` violation automatically. Validate via:

```bash
psql "postgres://postgres:postgres@localhost:5445/violations_db?sslmode=disable" \
  -c "SELECT id, trip_id, type, status FROM violations"
```

To call the HTTP API issue a JWT (e.g. through `snowops-auth-service`) for the Akimat user `00000000-0000-0000-0000-000000000011` and run:

```bash
curl -H "Authorization: Bearer <jwt>" http://localhost:7086/violations
curl -X POST -H "Authorization: Bearer <jwt>" \
     -H "Content-Type: application/json" \
     -d '{"trip_id":"00000000-0000-0000-0000-000000000071","type":"FOREIGN_AREA","detected_by":"GPS","severity":"HIGH","description":"manual test"}' \
     http://localhost:7086/violations
```

The first request lists the auto-created violation, the second creates a manual one (and instantly logs the status change). Contractor/driver tokens can exercise `/violations/:id/appeals`, `/appeals`, `/appeals/:id/comments`, while KGU/Akimat tokens go through `/appeals/:id/actions` to test the lifecycle.

### Configuration

| Env var | Description | Default |
|---------|-------------|---------|
| `APP_ENV` | Environment (`development` / `production`) | `development` |
| `HTTP_HOST` / `HTTP_PORT` | Bind address/port | `0.0.0.0` / `7086` |
| `DB_DSN` | PostgreSQL DSN | required |
| `DB_MAX_OPEN_CONNS` / `DB_MAX_IDLE_CONNS` | Connection pool | `25` / `10` |
| `DB_CONN_MAX_LIFETIME` | Max connection lifetime | `1h` |
| `JWT_ACCESS_SECRET` | JWT verification secret | required |
| `APPEAL_MAX_ATTACHMENTS` | Max attachments per action (create/comment) | `5` |

## Implementation notes

- `internal/model` mirrors Snowops domain snippets (trip/ticket/driver/vehicle/area) so Gin can preload context without importing other services.
- `ViolationService` orchestrates scope resolution, violation list/detail, manual creation, status overrides and composes DTOs with last appeal summary.
- `AppealService` enforces one-active rule, validates roles, transitions statuses per spec, and keeps violation statuses in sync.
- Handler returns consistent envelopes and maps service errors to HTTP codes (400/403/404/409).
- The service is fully self-contained: cloning this repo and running `go run ./cmd/violation-service` after `docker compose up` is enough to explore EPIC 7 flows.
