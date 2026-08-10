# PulseBoard — Technical Requirements Document

**Document Type:** Technical Requirements / Design (TRD)
**Corresponds to Product Doc:** PRD.md (v1.1.0)
**Current Codebase Version:** 1.2 (see §0.3 — version divergence note)
**Status:** Living document; updated as architecture decisions are made
**Scope:** How the system is structured — components, data, APIs, infrastructure, technology decisions.
**Explicitly out of scope here:** package internals, interface signatures, concurrency implementation, testing mechanics — see TDD.md.

---

## 0. Document Conventions

### 0.1 Traceability

Every major section below is annotated with the PRD section(s) it implements, e.g. `[PRD §13]`. If a technical decision has no corresponding PRD section, that is flagged explicitly as **[NO PRD ANCHOR]** — such decisions require either a PRD addendum or justification as pure infrastructure concern (e.g. CI pipeline shape has no product-facing requirement, and that's fine).

### 0.2 ADR Format

Where a genuine technology or architecture choice is being made (not merely inherited from the existing codebase), it is recorded as:

```text
Decision:              <what was chosen>
Rationale:             <why, in terms of constraints>
Alternatives Considered: <what else was viable>
Rejected Because:      <why the alternatives lost>
Status:                Accepted | Provisional | Superseded
```

Where the codebase has already made the choice unilaterally (i.e. this document is catching up to code, not leading it), the ADR is marked `Status: Accepted (retroactive)` — meaning it is not open for re-litigation per PRD intake §11, but the rationale is recorded so a future maintainer doesn't have to reverse-engineer intent from `git blame`.

### 0.3 Version Divergence Note

The PRD's `Current Version` field reads `1.1.0`; the codebase intake identifies itself as `V1.2`. This TRD treats the codebase as ground truth for *implementation state* and the PRD as ground truth for *product scope*. Recommend the PRD's version field be bumped to match on next PRD revision — a mismatched version number between PRD and running code is a small thing that erodes trust in both documents over time, and costs one line to fix.

---

## 1. System Context

### 1.1 What This System Is

A single backend API service (Go/Gin) backed by PostgreSQL, presently exposing authenticated CRUD over two resources: `User` and `Board` (the latter a reference pattern — see §5.2). No frontend exists yet; no real-time transport exists yet. This is the **V1.1–V1.2** state of the roadmap in PRD §26/§41.

### 1.2 Context Diagram

```text
                         ┌─────────────────────┐
                         │   Angular Client      │   (not yet built — V5.0)
                         │   (browser)           │
                         └──────────┬────────────┘
                                    │ HTTPS / JSON
                                    │ (WebSocket — V4.0, not yet built)
                                    ▼
                         ┌─────────────────────┐
                         │   PulseBoard API      │
                         │   (Go / Gin)          │
                         │                       │
                         │  ┌─────────────────┐  │
                         │  │ Middleware        │  │  auth (JWT), rate-limit, logging
                         │  ├─────────────────┤  │
                         │  │ Routes → Handlers │  │
                         │  ├─────────────────┤  │
                         │  │ Services          │  │
                         │  ├─────────────────┤  │
                         │  │ Repositories      │  │  GORM
                         │  └─────────────────┘  │
                         └──────────┬────────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │   PostgreSQL          │
                         └─────────────────────┘
```

### 1.3 Boundaries — What This System Explicitly Does Not Do

Direct inheritance from PRD §5 (Non-Goals). Restated here because a TRD that doesn't restate non-goals tends to accumulate scope silently — an engineer adding a feature reads the TRD, not the PRD, and if the TRD is silent on a boundary, the boundary erodes.

No video, no chat, no file storage, no calendar integration, no screen/keystroke monitoring, no productivity scoring. Concretely: no `internal/` package should ever be named anything resembling `monitoring`, `tracking`, or `scoring` in a way that inspects *how* a user works rather than *what state they've declared*.

---

## 2. Technology Stack

### 2.1 Core Stack — Retroactive ADRs

```text
Decision:               Go 1.25.12, Gin v1.11.0, GORM (postgres driver), PostgreSQL
Rationale:              Statically typed, single-binary deployment, mature ecosystem for
                        REST APIs and WebSocket handling (relevant ahead of V4.0). GORM
                        trades some query-plan control for development velocity, which is
                        appropriate at this product stage (small team, short timeline —
                        PRD §37).
Alternatives Considered: sqlc (compile-time-checked SQL, no ORM), raw database/sql
Rejected Because:       Not rejected outright — flagged as a live trade-off (§2.2), not
                        a closed question. GORM was chosen already in code; this document
                        records why it's defensible, not that it's unimprovable.
Status:                 Accepted (retroactive)
```

### 2.2 Open Trade-off: GORM vs. sqlc — [Provisional, revisit before V3.0]

GORM's reflection-based query building is fine at current scale (§10, low-to-moderate concurrency) but has two failure modes worth naming now rather than discovering at V4.0 under real-time load:

1. **N+1 query risk** grows with relational complexity — and V3.0 (Presence/Availability) plus V4.0 (real-time feed) will introduce exactly that: dashboard summary queries joining users × presence × current activity × recent events. GORM's `Preload` API mitigates but does not eliminate this; it requires discipline that's easy to erode under deadline pressure.
2. **No compile-time query verification** — a typo'd column name in a `Where()` clause fails at runtime, not build time.

**Recommendation, not mandate:** keep GORM for the User/Board/Activity CRUD paths (low query complexity, high development velocity matters more), but write the V3.0 dashboard aggregation query (§6.4 below) as raw SQL via `database/sql` or a lightweight query builder, since that query is both performance-sensitive and complex enough that GORM's ergonomics stop paying for themselves. This is a targeted exception, not a rewrite — don't let this recommendation turn into an ORM-migration side-quest.

### 2.3 Auth Stack

```text
Decision:               JWT via golang-jwt/jwt/v5, stateless bearer tokens
Rationale:              No session store required; horizontally scalable by default
                        (relevant if V7.0 ever needs >1 instance — §9.3); matches
                        PRD §18's requirement for "protected user-specific actions."
Alternatives Considered: Server-side session with Redis/Postgres-backed store
Rejected Because:       Adds an infra dependency (session store) the team does not yet
                        need at MVP scale; JWT statelessness is a genuine simplification
                        here, not premature optimisation, given §10's single-instance
                        acceptance for MVP.
Status:                 Accepted (retroactive)
```

Note for V6.0 (PRD §31, "Protected actions," "User-specific permissions"): JWT's stateless nature means **token revocation is not free**. If PulseBoard ever needs "log out everywhere" or immediate permission downgrade, that requires either short-lived tokens + refresh rotation, or a denylist store — decide *before* V6.0 starts, not during, since retrofitting revocation onto a shipped stateless-JWT system is a much larger change than designing it in from the start.

### 2.4 Logging & Observability

```text
Decision:               log/slog (standard library, Go 1.21+)
Rationale:              No external dependency; structured logging out of the box;
                        sufficient for current single-instance, low-traffic deployment.
Alternatives Considered: zerolog, zap
Rejected Because:       Marginal performance gains from zerolog/zap are irrelevant at
                        current request volume; stdlib reduces dependency surface.
Status:                 Accepted (retroactive)
```

Flag for V7.0 (PRD §32, "Operational monitoring," "Error visibility"): `slog` gives you structured logs, not metrics or traces. Before production readiness, decide explicitly whether "operational monitoring" means log aggregation only (e.g. shipping `slog` JSON output to a log sink) or requires actual metrics (Prometheus-style counters/histograms) and distributed tracing. These are different engineering efforts with different cost, and PRD §32 doesn't specify which — this is a **[NO PRD ANCHOR]** gap worth a one-line PRD addendum before V7.0 planning starts in earnest.

---

## 3. Repository & Package Architecture

### 3.1 Current Structure (Accepted, retroactive)

```text
cmd/server/            — entrypoint, wiring (main.go)
internal/config/       — env/config loading
internal/handlers/     — HTTP handlers (Gin), request/response translation
internal/middleware/   — auth (JWT verify), rate-limiting, logging
internal/migrations/   — versioned .up.sql migration runner
internal/models/       — GORM structs / domain entities
internal/repository/   — data-access layer (GORM queries), one repo per aggregate
internal/routes/       — route registration, wiring handlers to Gin router groups
internal/services/     — business logic layer between handlers and repositories
pkg/utils/             — shared, framework-agnostic helpers
```

This is a conventional **layered architecture** (handler → service → repository), not hexagonal/ports-and-adapters and not a domain-driven "internal/domain" split. That's a legitimate choice at this scale — full hexagonal architecture is overhead the product doesn't need yet — but it is worth naming explicitly so nobody proposes a ports-and-adapters refactor without first asking "does the added indirection buy us anything at 3 domain entities?" It doesn't, yet. Revisit if/when the domain model (§5) grows past ~5–6 aggregates with genuinely divergent persistence needs.

### 3.2 Package Boundary Rule Going Forward

Each of `handlers/`, `services/`, `repository/` should remain organised **per-aggregate**, not per-layer-then-flat — i.e. `board_handler.go`, `board_service.go`, `board_repo.go` (mirrored for `user_*`, and to be mirrored for `activity_*` in V2.0). This is already the apparent convention from the intake (`board_handler.go`, `board_repo.go`); this TRD formalises it as the standing rule rather than an accident of one resource's implementation. Deviating from it (e.g. one giant `handlers.go`) is a code-review-blocking issue, not a style preference — package-per-aggregate is what keeps the V2.0/V3.0 domain additions additive rather than requiring reorganisation each time.

---

## 4. Data Architecture — Overview

Detailed schema and migration sequencing lives in §6. This section covers the conceptual model only.

### 4.1 Aggregate Map (Current + Planned)

```text
User (done, V1.0/V1.1)
  │
  ├── owns → Board  (reference pattern, §5.2 — will be superseded by Activity)
  │
  ├── has-one (current) → Activity  [PRD §8.3, V2.0]
  ├── has-many (history) → ActivityEvent  [PRD §8.4, §17, V2.0/V4.0]
  ├── has-one → PresenceState  [PRD §8.1, V3.0]
  └── has-one → AvailabilityState  [PRD §8.2, V3.0]
```

### 4.2 Key Modelling Decision: Presence/Availability as Columns on User, Not Separate Tables

```text
Decision:               PresenceState and AvailabilityState are enum columns on the
                        `users` table (presence_state, availability_state), not separate
                        joined tables.
Rationale:              Every dashboard read (PRD §15) needs presence + availability +
                        current activity for every visible user, on every poll or
                        real-time push. Denormalising onto `users` avoids a join on the
                        hottest read path in the product. This is the single query the
                        entire dashboard experience depends on (§15.1, §15.2) — it should
                        be the cheapest query in the system, not the most expensive.
Alternatives Considered: Normalised `presence_states` / `availability_states` tables with
                        FKs; a generic `user_state` key-value table.
Rejected Because:       Presence and Availability each have a small, fixed enum domain
                        (§8.1, §8.2) — normalising them buys referential integrity you
                        don't need (the enum is enforced at the application/DB-check-
                        constraint level instead) at the cost of a join on every dashboard
                        read. The generic key-value alternative was rejected for the
                        opposite reason: it removes type safety and makes "last active
                        time per state" queries awkward.
Status:                 Provisional — revisit if a future requirement needs a *history*
                        of presence/availability transitions with timestamps beyond what
                        the ActivityEvent history table (§4.3) already captures.
```

### 4.3 Activity vs. ActivityEvent — Two Different Tables, Not One

PRD §8.3 (current activity, with lifecycle) and §8.4 (activity *history*) are easy to conflate into a single table with a status column, but that's a modelling trap: "what is Rahul doing right now" (a single row, frequently updated) and "what has the team done recently" (an append-only log, frequently inserted, rarely updated) have opposite access patterns and opposite update/insert ratios. Collapsing them into one table means either:

- an append-only table where "current activity" is `SELECT ... ORDER BY created_at DESC LIMIT 1` per user (expensive, and awkward to keep fresh on the hot dashboard path), or
- a mutable table where "history" requires trigger-based or application-level snapshotting on every update (audit-log-via-mutation, a known anti-pattern).

**Decision:** two tables. `activities` (current state, one row per user, upserted) and `activity_events` (append-only, PRD §17's event list: `ACTIVITY_STARTED`, `ACTIVITY_UPDATED`, `ACTIVITY_COMPLETED`, plus the presence/availability events from V3.0). Schema in §6.3.

---

## 5. Domain Model

### 5.1 `User` — Current, Stable

```go
// internal/models — current shape (accepted)
type User struct {
    ID            uint64
    Email         string
    PasswordHash  string
    DisplayName   string
    StatusMessage string
    LastActiveAt  time.Time
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

Note: `StatusMessage` already exists on `User` and will need reconciling against V2.0's `Activity.description` — these are two different concepts (`StatusMessage` reads as free-text status akin to availability annotation; `Activity` is the structured "what I'm working on" per PRD §8.3) but they will look redundant to anyone reading the schema cold. **Recommend**: either fold `StatusMessage` into the `Activity` model at V2.0 (deprecate the column), or explicitly document the distinction (e.g. `StatusMessage` = user-authored freeform note shown alongside availability; `Activity` = structured, lifecycle-tracked work item) before V2.0 implementation begins, so this isn't discovered mid-migration.

### 5.2 `Board` — Reference Pattern, Scheduled for Supersession

Per your clarification: `Board` is the "owned domain entity" template — ownership enforcement, soft delete, repository/handler pairing — that `Activity` should structurally mirror. It is **not** a product concept and has no PRD anchor.

```text
Decision:               Board remains in the codebase through V1.x as a working reference,
                        formally deprecated (not deleted) the moment Activity ships in V2.0.
Rationale:              Deleting it now loses institutional value; it's a working, tested
                        example of the ownership + soft-delete pattern other domain objects
                        should follow. Deleting it after Activity ships removes dead code
                        weight without losing that value, since Activity will by then embody
                        the same pattern.
Status:                 Accepted — tracked as technical debt item TD-01 (§15).
```

Concretely, V2.0 implementation should be done by **structurally cloning** `board_handler.go` / `board_service.go` / `board_repo.go` into `activity_handler.go` / `activity_service.go` / `activity_repo.go`, adapting the domain fields, rather than designing the Activity layer from scratch — this is a deliberate reduction in design risk, not laziness: the Board pattern is already tested and has already worked through the ownership-scoping edge cases (PRD's implicit requirement that a user can only see/edit their own state).

### 5.3 `Activity` — Planned, V2.0 [PRD §13, §27]

```go
type ActivityStatus string

const (
    ActivityStarted   ActivityStatus = "started"
    ActivityActive    ActivityStatus = "active"
    ActivityCompleted ActivityStatus = "completed"
)

type Activity struct {
    ID          uint64
    UserID      uint64         // one active Activity per user — enforced via unique
                                // partial index WHERE status != 'completed'
    Description string         // PRD §13 example: "Implementing WebSocket support"
    Status      ActivityStatus
    StartedAt   time.Time
    CompletedAt *time.Time
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

**Modelling decision requiring your sign-off before implementation:** PRD §8.3's lifecycle diagram (`Started → Active → Completed`) implies three states, but §13 (PR-04/05/06 — Create/Update/Complete Activity) never describes a transition *into* `Active` distinct from `Started`, nor what triggers it. This is a **genuine PRD gap**, not a technical decision I can make unilaterally: either (a) `Started` and `Active` collapse into one state for V2.0 (simplify to `active | completed`, revisit the three-state lifecycle only if a real use case for the distinction emerges), or (b) `Active` is meant to represent "has been running >N minutes" as a derived/computed state rather than a stored one. Recommend (a) for V2.0 — ship the two-state version, since inventing a trigger for a distinction the PRD never operationalises is exactly the kind of unrequested complexity PRD §40 Principle 4 ("every feature should justify its existence") warns against.

### 5.4 `ActivityEvent` — Planned, V2.0/V4.0 [PRD §8.4, §17]

```go
type ActivityEventType string

const (
    EventUserOnline           ActivityEventType = "USER_ONLINE"
    EventUserOffline          ActivityEventType = "USER_OFFLINE"
    EventAvailabilityChanged  ActivityEventType = "AVAILABILITY_CHANGED"
    EventActivityStarted      ActivityEventType = "ACTIVITY_STARTED"
    EventActivityUpdated      ActivityEventType = "ACTIVITY_UPDATED"
    EventActivityCompleted    ActivityEventType = "ACTIVITY_COMPLETED"
    EventUserBecameAway       ActivityEventType = "USER_BECAME_AWAY"
)

type ActivityEvent struct {
    ID        uint64
    UserID    uint64
    Type      ActivityEventType
    Payload   json.RawMessage // event-specific detail, e.g. {"description": "..."}
    CreatedAt time.Time
}
```

Append-only; no update path. Retention policy per PRD §22 ("reasonable retention policies") is a **[NO PRD ANCHOR]** open question — PRD says retention should be "reasonable" but doesn't define a number. Recommend a default of 30-day retention with a scheduled cleanup job, stated explicitly here so it isn't silently decided by "whatever the query happens to `LIMIT`."

### 5.5 `PresenceState` / `AvailabilityState` — Planned, V3.0 [PRD §8.1, §8.2]

Per §4.2's decision, these are enum columns on `users`, not tables:

```go
type PresenceState string

const (
    PresenceOnline  PresenceState = "online"
    PresenceAway    PresenceState = "away"
    PresenceOffline PresenceState = "offline"
)

type AvailabilityState string

const (
    AvailabilityAvailable    AvailabilityState = "available"
    AvailabilityBusy         AvailabilityState = "busy"
    AvailabilityInMeeting    AvailabilityState = "in_meeting"
    AvailabilityDoNotDisturb AvailabilityState = "do_not_disturb"
)
```

Added to `User`:

```go
PresenceState     PresenceState
AvailabilityState AvailabilityState
```

---

## 6. Database Schema

### 6.1 Current — `users` (Accepted)

```sql
-- 000001_create_users.up.sql (existing)
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name    TEXT NOT NULL DEFAULT '',
    status_message  TEXT NOT NULL DEFAULT '',
    last_active_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

*(Reconciled against the Go struct in §5.1 — the intake's raw SQL paste predates the display_name/status_message/last_active_at columns that the Go model and route list already assume exist; confirm this migration has in fact been updated, or there is drift between `000001_create_users.up.sql` and the live schema that needs a follow-up migration before V2.0 work starts.)*

### 6.2 Current — `boards` (Accepted, scheduled for deprecation per §5.2)

```sql
-- 000002_create_boards.up.sql (existing)
CREATE TABLE boards (
    id          BIGSERIAL PRIMARY KEY,
    owner_id    BIGINT NOT NULL REFERENCES users(id),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);
CREATE INDEX idx_boards_owner_id ON boards(owner_id) WHERE deleted_at IS NULL;
```

### 6.3 Planned — V2.0 Migrations

```sql
-- 000003_create_activities.up.sql
CREATE TABLE activities (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id),
    description  TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'active'
                 CHECK (status IN ('active', 'completed')), -- see §5.3 open question
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- enforce "one active Activity per user" per §5.3
CREATE UNIQUE INDEX idx_activities_one_active_per_user
    ON activities(user_id) WHERE status = 'active';

-- 000004_create_activity_events.up.sql
CREATE TABLE activity_events (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id),
    event_type TEXT NOT NULL,
    payload    JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_activity_events_created_at ON activity_events(created_at DESC);
```

### 6.4 Planned — V3.0 Migration

```sql
-- 000005_add_presence_availability_to_users.up.sql
ALTER TABLE users
    ADD COLUMN presence_state     TEXT NOT NULL DEFAULT 'offline'
        CHECK (presence_state IN ('online','away','offline')),
    ADD COLUMN availability_state TEXT NOT NULL DEFAULT 'available'
        CHECK (availability_state IN ('available','busy','in_meeting','do_not_disturb'));
```

### 6.5 Dashboard Aggregation Query — the Hot Path (§2.2)

```sql
-- Single-query dashboard fetch — candidate for hand-written SQL per §2.2,
-- bypassing GORM's N+1-prone Preload chain.
SELECT
    u.id, u.display_name, u.presence_state, u.availability_state, u.last_active_at,
    a.description AS current_activity, a.started_at AS activity_started_at
FROM users u
LEFT JOIN activities a ON a.user_id = u.id AND a.status = 'active'
ORDER BY u.display_name;
```

This single query answers PRD §15.1 and §15.2 in full. Worth benchmarking against `EXPLAIN ANALYZE` once V3.0 data exists — at the concurrency levels stated in intake §10 (low-to-moderate), this will not be a bottleneck, but it's the query to watch if the product ever grows past "small team."

---

## 7. API Design

### 7.1 Current Endpoints (Accepted)

```text
GET    /health
GET    /ready
POST   /api/auth/register
POST   /api/auth/login
GET    /api/me
PATCH  /api/me
DELETE /api/me
POST   /api/boards
GET    /api/boards
GET    /api/boards/:id
PATCH  /api/boards/:id
DELETE /api/boards/:id
```

### 7.2 Versioning — [NO PRD ANCHOR, Recommendation]

```text
Decision:               Introduce /api/v1/ prefix before V2.0 ships new endpoints, rather
                        than adding versioning retroactively later.
Rationale:              The cost of adding a version prefix now (rename routes, update
                        client base URL — there is no client yet) is near zero. The cost
                        of adding it after V5.0's Angular client exists and is hardcoding
                        unversioned paths is a coordinated multi-repo change. This is a
                        classic "cheap now, expensive later" call.
Alternatives Considered: Defer versioning indefinitely (YAGNI); header-based versioning
                        (Accept: application/vnd.pulseboard.v1+json)
Rejected Because:       Deferring costs nothing today but compounds; header-based
                        versioning is more RESTfully "correct" but adds cognitive
                        overhead disproportionate to a small-team MVP (PRD §37) — path
                        versioning is what nearly every consumer of this API will expect
                        by default.
Status:                 Provisional — flagging for your decision, not unilaterally
                        implemented, since it touches every existing route.
```

### 7.3 Planned Endpoints — V2.0 (Activity)

```text
POST   /api/v1/activities            — create/start current activity   [PR-04]
GET    /api/v1/activities/current    — get own current activity
PATCH  /api/v1/activities/current    — update current activity         [PR-05]
POST   /api/v1/activities/current/complete — mark complete             [PR-06]
GET    /api/v1/activities/recent     — recent activity feed            [PRD §17]
```

Deliberately **not** `POST/GET/PATCH/DELETE /api/v1/activities/:id` generic CRUD — per §5.3's one-active-Activity-per-user constraint, the "current activity" is a singleton resource from the client's perspective, not a collection member the client addresses by ID. Modelling it as `/activities/current` rather than `/activities/:id` avoids an entire class of bugs where a client caches or hardcodes an activity ID that's since rotated.

### 7.4 Planned Endpoints — V3.0 (Presence/Availability)

```text
PATCH  /api/v1/me/availability       — set availability state
GET    /api/v1/dashboard             — team summary + member cards     [PRD §15]
```

Presence itself (`online`/`away`/`offline`) is **not** a client-settable field via PATCH — per PRD §8.1/§14, presence is server-derived (connection state + inactivity detection), and exposing a PATCH endpoint for it would let a client lie about being online. Availability *is* client-settable (§8.2, it's a deliberate user signal). This distinction should be enforced at the API surface, not just convention — don't add `presence_state` to any request DTO.

### 7.5 Error Response Contract

Current shape (`{"error": "..."}`) is minimally viable but has no machine-readable error code, which will start to hurt once the Angular client (V5.0) needs to distinguish "validation failed on field X" from "unauthorized" from "not found" programmatically rather than by parsing English strings.

```text
Decision:               Extend error envelope before V2.0 to:
                        {"error": {"code": "VALIDATION_FAILED", "message": "...",
                                   "fields": {"description": "required"}}}
                        while keeping the top-level "error" key for backward
                        compatibility with any existing manual testing/tooling.
Status:                 Provisional — low-risk, recommend adopting alongside the
                        /v1/ prefix change (§7.2) since both touch every handler anyway.
```

---

## 8. Real-Time Architecture [PRD §14, V4.0]

Not yet implemented (accepted current state — intake §5). This section is forward design, to be treated as provisional until V4.0 work actually starts, since requirements may shift.

```text
Decision:               WebSocket (not SSE, not polling) via gorilla/websocket or
                        nhooyr.io/websocket, single-instance hub-and-broadcast pattern.
Rationale:              PRD §14 requires bidirectional-feeling low-latency updates
                        (client changes state → others see it "almost immediately," no
                        manual refresh). SSE is simpler and would suffice for the
                        server→client push half of this, but the client also needs to
                        send state changes, which SSE doesn't cover — you'd need SSE
                        for the feed plus REST for writes, which is two transports for
                        one feature. WebSocket unifies both directions.
Alternatives Considered: SSE + REST writes; long-polling; a message broker
                        (Redis pub/sub, NATS) from day one.
Rejected Because:       SSE+REST is a reasonable alternative, not obviously wrong — but
                        adds transport-splitting complexity for no real gain here, since
                        writes are already going through REST regardless (state changes
                        are validated, persisted mutations — you want that on REST with
                        normal request/response semantics, not WebSocket frames). A
                        message broker is deferred per intake §5/§10: single-instance is
                        explicitly acceptable for MVP, and an in-process hub (a single
                        Go struct holding a map of user-ID → connection, guarded by a
                        mutex, broadcasting via channels) is sufficient until horizontal
                        scaling is actually needed. Introducing Redis pub/sub now would
                        be solving a scaling problem you don't have yet, at the cost of
                        an operational dependency you'd then have to run, monitor, and
                        version — pure premature optimisation.
Status:                 Provisional pending V4.0 kickoff; concurrency-level implementation
                        (hub struct, goroutine-per-connection, channel design) belongs in
                        TDD.md §9, not here.
```

**Explicit scaling trapdoor:** the in-process hub design works only as long as PulseBoard runs as one instance (true today per intake §5, §10). If PRD adoption ever requires horizontal scaling *before* a message-broker migration happens, WebSocket broadcasts silently stop reaching clients connected to a different instance than the one that received the state change — this fails silently, not loudly, which makes it dangerous. Record this as TD-02 (§15) now so it's a known trade-off, not a production incident later.

---

## 9. Deployment Architecture [PRD §32, V7.0]

### 9.1 Current (Accepted)

```text
Docker container, built via Dockerfile, orchestrated locally via compose.yaml.
Config via environment variables (os.Getenv) with .env support (godotenv) and
.env.example as the documented contract. GitHub Actions runs gofmt check, go vet,
tests, and Docker image build/publish on CI.
```

### 9.2 Config Convention Going Forward

As new config values are needed (JWT secret rotation, WebSocket-related tunables, retention-policy cutoffs per §5.4), they should be added to `.env.example` in the same commit that introduces the code reading them — an env var with no corresponding `.env.example` entry is effectively undocumented config, and will cost someone a debugging session eventually.

### 9.3 Single-Instance Assumption — Explicit Statement

Per intake §5/§10: single-instance deployment is accepted for MVP and V4.0's real-time hub design (§8) depends on this assumption holding. This TRD flags it once, here, as the canonical statement — any future change to "we need multiple instances" is not a deployment-config change, it's an architecture change that reopens §8's ADR.

---

## 10. Non-Functional Requirements

| Concern | Current stance | Source |
|---|---|---|
| Concurrency | Low-to-moderate, small-team scale | Intake §10 |
| Latency | "Near-instant" desired, no strict SLA defined | PRD §14, Intake §10 — **[NO PRD ANCHOR: numeric target]** |
| Availability | No stated uptime target | **[NO PRD ANCHOR]** |
| Data retention | "Reasonable" per PRD §22, no number given | PRD §22 — recommend 30 days for `activity_events` (§5.4) |
| Deployment environment | Standard containerised, no on-prem/air-gap constraint | Intake §10 |

The absence of a numeric latency/uptime target isn't a defect in the PRD — PRD §34 (Success Metrics) is deliberately qualitative ("almost immediately," "within a few seconds"), which is appropriate for a product-level document. But *someone* needs to pick a number before it can be tested against, and that someone is this TRD: recommend **p95 dashboard-update latency under 500ms** (client state change → other connected clients see it) as the working engineering target, revisited if real usage data suggests otherwise.

---

## 11. Testing Strategy — Overview

(Mechanics — table-driven tests, fixture setup, mocking conventions — in TDD.md §11.)

Current: Go stdlib `testing` + `httptest`, integration tests against a real Postgres instance gated by `TEST_DATABASE_URL` (accepted, sound approach — it catches GORM-query and migration-drift bugs that pure mocking would miss). Coverage currently spans auth, profile management, board ownership/soft-delete, rate-limiting.

**Gap to close before V2.0 ships:** no load/concurrency test exists yet for the "one active Activity per user" unique-index constraint (§6.3) under concurrent PATCH requests — this is exactly the kind of invariant that passes every sequential test and then breaks under two near-simultaneous requests in production. Recommend one explicit concurrency test here, not deferred to "we'll add it if it breaks."

---

## 12. Architecture Roadmap — Mapped to PRD Versions

| PRD Version | Architectural Work | TRD Section |
|---|---|---|
| V1.1 (current) | Auth, user CRUD, Board reference pattern | §5.1, §5.2 |
| V2.0 | Activity + ActivityEvent tables, `/activities/*` endpoints, Board→Activity pattern clone | §5.3–5.4, §6.3, §7.3 |
| V3.0 | Presence/Availability columns, dashboard aggregation query | §5.5, §6.4, §6.5, §7.4 |
| V4.0 | WebSocket hub, real-time broadcast | §8 |
| V5.0 | Angular client (no backend architecture change expected beyond CORS/API-consumption concerns) | — |
| V6.0 | Token revocation strategy decision (§2.3 flag) | §2.3 |
| V7.0 | Observability decision (metrics/tracing vs. logs-only, §2.4), single-vs-multi-instance re-evaluation (§9.3) | §2.4, §9.3 |

---

## 13. Open Decisions Requiring Your Sign-Off

Consolidated from throughout this document — these are the points where I've made a recommendation but stopped short of treating it as accepted, because each one is either irreversible-ish or touches product semantics rather than pure implementation:

1. **§5.3** — Collapse `Activity` lifecycle to two states (`active`/`completed`) for V2.0, deferring the `started`/`active` distinction. *Recommended, needs your confirmation — this is a PRD-level gap, not just a technical one.*
2. **§5.1** — Reconcile `StatusMessage` vs. `Activity.description` overlap before V2.0. *Needs a decision: deprecate, or document distinction.*
3. **§7.2** — Introduce `/api/v1/` prefix before V2.0 ships. *Recommended, cheap now/expensive later.*
4. **§7.5** — Extend error envelope with machine-readable `code`. *Recommended, low-risk.*
5. **§5.4** — 30-day retention default for `activity_events`. *Recommended default, PRD doesn't specify.*
6. **§10** — p95 500ms latency as the working NFR target. *Recommended default, PRD doesn't specify a number.*

---

## 14. Technical Debt Register

| ID | Item | Introduced | Resolve By |
|---|---|---|---|
| TD-01 | `Board` resource is a reference pattern with no product meaning; deprecate once `Activity` ships | V1.x (deliberate) | V2.0 completion |
| TD-02 | In-process WebSocket hub does not survive horizontal scaling; silent broadcast failure if ever multi-instance | V4.0 design | Before any move to >1 instance |
| TD-03 | `000001_create_users.up.sql` may be stale relative to live schema (display_name/status_message/last_active_at) — verify | Pre-existing | Before V2.0 migrations are written on top of it |
| TD-04 | No concurrency test for "one active Activity per user" unique constraint | To be introduced at V2.0 | V2.0 completion |

---

*Companion document: TDD.md covers package-level interfaces, concurrency implementation, WebSocket message protocol, and testing mechanics for everything designed at the architecture level above.*