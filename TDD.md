# PulseBoard — Technical Design / Development Document

**Document Type:** Technical Design / Development (TDD)
**Corresponds to:** TRD.md (architecture) and PRD.md (product, v1.1.0)
**Scope:** How the system is implemented — interfaces, concurrency, protocols, error handling, testing mechanics, coding conventions.
**Explicitly out of scope here:** *why* an architectural choice was made (TRD.md §2's ADRs) — this document assumes those decisions as given and details their execution.

---

## 1. Package Responsibilities — Implementation Contract

Restating TRD §3.1's structure with the *contract* each package must honour, since a layered architecture only stays clean if each layer actually respects its boundary:

```text
handlers/    — Gin-specific. Parses request, calls exactly one service method,
               translates result/error to HTTP response. MUST NOT contain business
               logic (no validation beyond request shape, no direct repository calls).

services/    — Business logic. Framework-agnostic — no *gin.Context, no HTTP status
               codes. Returns domain errors (§6), not HTTP errors. Calls repositories
               via interfaces (§2), not concrete GORM types, so services are unit-
               testable without a database.

repository/  — GORM queries only. One file per aggregate. Returns domain models or
               domain errors (ErrNotFound, etc.) — never a raw gorm.Error leaked
               upward (a handler should never need to know GORM exists).

middleware/  — Cross-cutting: auth verification, rate limiting, request logging.
               Operates on *gin.Context; sits between routing and handlers.

models/      — Plain Go structs (GORM tags included). No behaviour beyond simple
               invariant methods (e.g. Activity.IsActive() bool). Not where
               validation lives (§5).

routes/      — Wiring only: maps HTTP verb+path to handler method, applies
               middleware chains. Zero logic.
```

**Rule with teeth:** if a code review finds a repository call inside a handler, or a `*gin.Context` parameter on a service method, that's a boundary violation and should block merge — not because layering is dogma, but because the entire reason services are unit-testable without spinning up Postgres is that they don't know Gin or GORM exist. Break that once and the next person breaks it twice.

---

## 2. Interfaces — Dependency Direction

### 2.1 Repository Interfaces

Services depend on interfaces, not concrete repository structs, so unit tests can substitute an in-memory fake without a database:

```go
// internal/services/activity_service.go
type ActivityRepository interface {
    GetCurrent(ctx context.Context, userID uint64) (*models.Activity, error)
    Upsert(ctx context.Context, a *models.Activity) error
    Complete(ctx context.Context, userID uint64) error
}

type ActivityEventRepository interface {
    Append(ctx context.Context, e *models.ActivityEvent) error
    Recent(ctx context.Context, limit int) ([]models.ActivityEvent, error)
}

type ActivityService struct {
    repo       ActivityRepository
    eventRepo  ActivityEventRepository
}
```

The concrete GORM implementation (`internal/repository/activity_repo.go`) satisfies `ActivityRepository` structurally (Go's implicit interface satisfaction — no explicit `implements` needed, but name the receiver method set to match exactly). Mirror this for `BoardRepository` today and `UserRepository` — if they don't already follow this pattern, bringing them into alignment is a prerequisite for §11's unit-testing approach, not optional cleanup.

### 2.2 Why This Matters Now, Not Later

V2.0's `ActivityService` needs to call `ActivityEventRepository.Append()` every time activity state changes (§4 below) — and V4.0's WebSocket hub (§9) needs to be notified of that same event to broadcast it. If `ActivityService` depends on concrete types, wiring the hub in later means touching every call site. If it depends on an interface, V4.0 adds a `BroadcastNotifier` interface, and `ActivityService` takes one more constructor argument — additive, not invasive. Design the interface boundary now so V4.0 is a wiring change, not a refactor.

---

## 3. Domain Model — Go Definitions

(Canonical; TRD §5 introduced these at the architecture level — this is the exact implementation.)

```go
// internal/models/activity.go
package models

import "time"

type ActivityStatus string

const (
    ActivityStatusActive    ActivityStatus = "active"
    ActivityStatusCompleted ActivityStatus = "completed"
)

type Activity struct {
    ID          uint64         `gorm:"primaryKey"`
    UserID      uint64         `gorm:"not null;index"`
    Description string         `gorm:"not null"`
    Status      ActivityStatus `gorm:"not null;default:active"`
    StartedAt   time.Time      `gorm:"not null"`
    CompletedAt *time.Time
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

func (a *Activity) IsActive() bool { return a.Status == ActivityStatusActive }
```

```go
// internal/models/activity_event.go
package models

import (
    "encoding/json"
    "time"
)

type ActivityEventType string

const (
    EventUserOnline          ActivityEventType = "USER_ONLINE"
    EventUserOffline         ActivityEventType = "USER_OFFLINE"
    EventAvailabilityChanged ActivityEventType = "AVAILABILITY_CHANGED"
    EventActivityStarted     ActivityEventType = "ACTIVITY_STARTED"
    EventActivityUpdated     ActivityEventType = "ACTIVITY_UPDATED"
    EventActivityCompleted   ActivityEventType = "ACTIVITY_COMPLETED"
    EventUserBecameAway      ActivityEventType = "USER_BECAME_AWAY"
)

type ActivityEvent struct {
    ID        uint64            `gorm:"primaryKey"`
    UserID    uint64            `gorm:"not null;index"`
    Type      ActivityEventType `gorm:"not null"`
    Payload   json.RawMessage   `gorm:"type:jsonb"`
    CreatedAt time.Time         `gorm:"index:idx_created_desc,sort:desc"`
}
```

---

## 4. Service Layer — Activity Lifecycle Implementation

```go
// internal/services/activity_service.go

func (s *ActivityService) Start(ctx context.Context, userID uint64, description string) (*models.Activity, error) {
    if err := validateDescription(description); err != nil {
        return nil, err // ErrValidation — see §6
    }

    existing, err := s.repo.GetCurrent(ctx, userID)
    if err != nil && !errors.Is(err, ErrNotFound) {
        return nil, err
    }

    activity := &models.Activity{
        UserID:      userID,
        Description: description,
        Status:      models.ActivityStatusActive,
        StartedAt:   time.Now().UTC(),
    }

    // NOTE: this is a read-then-write across two statements (GetCurrent, then
    // Upsert) — see §4.1 for the concurrency hazard this creates and why the
    // DB-level unique partial index (TRD §6.3) is the real guard, not this check.
    if existing != nil && existing.IsActive() {
        activity.ID = existing.ID // upsert path
    }

    if err := s.repo.Upsert(ctx, activity); err != nil {
        if isUniqueViolation(err) {
            // Lost the race — see §4.1. Retry once by re-fetching and upserting
            // onto the winner's row, rather than surfacing a 500 for what is a
            // benign concurrent-update race, not a real error.
            return s.retryUpsertOnConflict(ctx, userID, description)
        }
        return nil, err
    }

    eventType := models.EventActivityStarted
    if activity.ID != 0 && existing != nil {
        eventType = models.EventActivityUpdated
    }
    _ = s.eventRepo.Append(ctx, &models.ActivityEvent{
        UserID: userID, Type: eventType,
        Payload: mustJSON(map[string]string{"description": description}),
    })
    // Event append failure is intentionally non-fatal to the primary write —
    // see §6.3 on error-handling philosophy for why.

    return activity, nil
}
```

### 4.1 Concurrency Hazard: Read-Then-Write on `Activity`

`GetCurrent` → `Upsert` is not atomic. Two near-simultaneous `PATCH /activities/current` requests from the same user (double-click, retried request, two browser tabs) can both read "no active activity," both attempt an insert, and race. This is exactly TRD's TD-04 (flagged, not yet tested).

**The fix is at the database layer, not the application layer:** TRD §6.3's `CREATE UNIQUE INDEX idx_activities_one_active_per_user ON activities(user_id) WHERE status = 'active'` is what actually prevents the invariant violation — the loser of the race gets a Postgres unique-constraint error, not silently-corrupted data. The application-level `GetCurrent` check above is an optimisation (avoid the round-trip when there's obviously no conflict) and a UX nicety (upsert onto the existing row rather than erroring), **not** the correctness mechanism. Do not let a future refactor remove the DB constraint on the theory that "the service layer already checks this" — it doesn't, reliably, under concurrency. This is the single most important invariant in the entire V2.0 milestone to get right, because it's the one that fails silently in testing (sequential tests never race) and loudly in production (real users double-click).

**Required test (closes TD-04):** two goroutines calling `Start` concurrently for the same user, asserting exactly one `activities` row exists afterward and the loser either retried-and-succeeded or received a well-defined conflict response — not a 500.

---

## 5. Validation

Validation belongs in the **service layer**, not handlers (per §1's contract) and not models (Go structs shouldn't validate themselves — that couples the data shape to the validation rules, which change independently, e.g. "description max length" is a product decision, not a data-shape fact).

```go
// internal/services/validation.go
const maxActivityDescriptionLen = 280 // [NO PRD ANCHOR — engineering default,
                                        // flag to product if a real limit matters]

func validateDescription(d string) error {
    d = strings.TrimSpace(d)
    if d == "" {
        return fmt.Errorf("%w: description is required", ErrValidation)
    }
    if len(d) > maxActivityDescriptionLen {
        return fmt.Errorf("%w: description exceeds %d characters", ErrValidation, maxActivityDescriptionLen)
    }
    return nil
}
```

Note the `280` is invented — there is no PRD-specified limit anywhere. This is exactly the kind of silent unilateral decision TRD §13 tries to surface explicitly rather than letting a magic number ship undiscussed. Flag it in PR description when V2.0 lands; get one line of product sign-off rather than assuming.

---

## 6. Error Handling Convention

### 6.1 Domain Error Sentinels

```go
// internal/services/errors.go
var (
    ErrNotFound   = errors.New("resource not found")
    ErrValidation = errors.New("validation failed")
    ErrConflict   = errors.New("conflicting state")
    ErrForbidden  = errors.New("not permitted")
)
```

Services return these (wrapped with `fmt.Errorf("%w: ...", ErrX, ...)` for context); handlers map them to HTTP status via a single central function — not ad-hoc `if err != nil { c.JSON(500, ...) }` scattered per handler:

```go
// internal/handlers/errors.go
func respondError(c *gin.Context, err error) {
    switch {
    case errors.Is(err, services.ErrNotFound):
        c.JSON(http.StatusNotFound, errorEnvelope("NOT_FOUND", err))
    case errors.Is(err, services.ErrValidation):
        c.JSON(http.StatusBadRequest, errorEnvelope("VALIDATION_FAILED", err))
    case errors.Is(err, services.ErrConflict):
        c.JSON(http.StatusConflict, errorEnvelope("CONFLICT", err))
    case errors.Is(err, services.ErrForbidden):
        c.JSON(http.StatusForbidden, errorEnvelope("FORBIDDEN", err))
    default:
        slog.Error("unhandled service error", "err", err)
        c.JSON(http.StatusInternalServerError, errorEnvelope("INTERNAL", errors.New("internal server error")))
    }
}
```

This is the implementation of TRD §7.5's extended error envelope. `errorEnvelope` constructs `{"error": {"code": ..., "message": ...}}`. **Never** put the raw `err.Error()` string from an unhandled/internal error into the response — that's a well-worn path to leaking internal detail (table names, query fragments) to clients; log it server-side, return a generic message client-side.

### 6.2 Rule: Every New Error Path Gets a Sentinel, Not a String

If a future service method needs a new failure mode not covered by the four sentinels above, add a new sentinel and a `respondError` case — do not `errors.New("some ad-hoc string")` and let it fall through to the generic 500 case. A 500 for something that is actually a well-defined 409 or 422 is a debugging tax paid by whoever's on call.

### 6.3 Non-Fatal Side Effects: Event Append Failure

§4's `_ = s.eventRepo.Append(...)` deliberately ignores the error. This is a considered choice, not sloppiness: if writing the primary `Activity` row succeeds but the `ActivityEvent` audit-log write fails, the user's actual state change *did* happen — returning a 500 to the client would be lying about what happened (their activity update did take effect) for the sake of an audit trail that's secondary to the core feature. The correct handling is: log the append failure at `slog.Error` level (so it's visible in monitoring) and let the primary operation succeed. If audit-log completeness ever becomes a hard product requirement, this needs revisiting — but PRD §17's language ("users should be able to understand what changed recently") reads as best-effort visibility, not a compliance audit trail, so this trade-off matches product intent as currently specified.

---

## 7. Request/Response DTOs

Handlers translate between wire format and domain models via explicit DTOs — never bind a request body directly onto a GORM model (couples wire format to schema; a column rename becomes a breaking API change).

```go
// internal/handlers/dto/activity.go
type StartActivityRequest struct {
    Description string `json:"description" binding:"required"`
}

type ActivityResponse struct {
    ID          uint64  `json:"id"`
    Description string  `json:"description"`
    Status      string  `json:"status"`
    StartedAt   string  `json:"started_at"` // RFC3339
    CompletedAt *string `json:"completed_at,omitempty"`
}

func toActivityResponse(a *models.Activity) ActivityResponse {
    resp := ActivityResponse{
        ID: a.ID, Description: a.Description, Status: string(a.Status),
        StartedAt: a.StartedAt.Format(time.RFC3339),
    }
    if a.CompletedAt != nil {
        s := a.CompletedAt.Format(time.RFC3339)
        resp.CompletedAt = &s
    }
    return resp
}
```

---

## 8. Middleware — Auth

Existing JWT middleware (accepted) verifies bearer token, extracts `user_id` claim, sets it on `gin.Context` via `c.Set("user_id", id)`. Convention for V2.0+ handlers:

```go
func (h *ActivityHandler) StartActivity(c *gin.Context) {
    userID := c.MustGet("user_id").(uint64) // panics if middleware not applied —
                                              // deliberate: a route missing auth
                                              // middleware should fail loudly at
                                              // first request, not silently serve
                                              // with userID=0.
    var req dto.StartActivityRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        respondError(c, fmt.Errorf("%w: %v", services.ErrValidation, err))
        return
    }
    activity, err := h.service.Start(c.Request.Context(), userID, req.Description)
    if err != nil {
        respondError(c, err)
        return
    }
    c.JSON(http.StatusOK, toActivityResponse(activity))
}
```

---

## 9. Real-Time — Concurrency & Protocol Design [V4.0, provisional per TRD §8]

This is forward design — not to be implemented until V4.0 starts, recorded now so the API/service layer built in V2.0/V3.0 doesn't foreclose it.

### 9.1 Hub Structure

```go
// internal/realtime/hub.go
type Hub struct {
    mu          sync.RWMutex
    connections map[uint64]map[*Connection]struct{} // userID -> set of conns
                                                       // (multiple tabs/devices)
    broadcast   chan Event
    register    chan *Connection
    unregister  chan *Connection
}

type Connection struct {
    userID uint64
    send   chan []byte // buffered; one writer goroutine per connection
    conn   *websocket.Conn
}

type Event struct {
    Type    models.ActivityEventType `json:"type"`
    UserID  uint64                   `json:"user_id"`
    Payload json.RawMessage          `json:"payload"`
}
```

### 9.2 Concurrency Model — Why This Shape

```text
Decision:               Single Hub goroutine owns the connections map and drains
                        register/unregister/broadcast channels in one select loop.
                        Each Connection gets exactly two goroutines: one blocking
                        read (for ping/pong + detecting client disconnect), one
                        blocking write draining Connection.send.
Rationale:              This is the standard gorilla/websocket chat-server pattern
                        for good reason: it confines all map mutation to a single
                        goroutine (the Hub loop), eliminating the need for the
                        sync.RWMutex on the hot broadcast path entirely — the mutex
                        above is only needed if something outside the Hub loop reads
                        `connections` (e.g. a metrics endpoint reporting connection
                        count); if nothing does, drop the mutex and rely purely on
                        channel-confinement. Don't add locking you don't need.
Alternatives Considered: sync.Map for connections; one goroutine per connection pair
                        doing direct writes without a Hub intermediary.
Rejected Because:       sync.Map optimises for a read-heavy, rarely-mutated key set —
                        wrong shape for connections churning as users connect/
                        disconnect. Direct writes-without-a-hub means N-to-M fan-out
                        logic (broadcast an event to all of a user's team) gets
                        duplicated at every write site instead of centralised once.
Status:                 Provisional — implement at V4.0 kickoff, re-validate this
                        design against gorilla/websocket vs nhooyr.io/websocket API
                        shape at that time (library choice itself still open per
                        TRD §8).
```

### 9.3 Broadcast Fan-Out — Team-Wide, Not Point-to-Point

PRD §14/§15.3: a state change from one user must reach *all other connected users' dashboards*, not just that user's own other tabs. This means `Hub.broadcast` receiving an `Event` fans out to every connection in `connections`, filtered only by "not the originating connection" if you want to avoid echoing a user's own change back to themselves redundantly (harmless either way, but wasteful). There is **no team/room partitioning** in V4.0's initial scope — PRD §21 (Teams and Groups) is explicitly out of MVP scope, so every connected user currently sees every other connected user's updates. This is correct *for now* but is a design point that must be revisited the moment PRD §21 ships — team-scoped broadcast is a materially different fan-out design (partition `connections` by team, not one global set), not an incremental tweak.

### 9.4 Message Protocol — Wire Format

```json
// Server → Client (broadcast)
{
  "type": "ACTIVITY_STARTED",
  "user_id": 42,
  "payload": { "description": "Implementing WebSocket support" },
  "ts": "2026-08-10T14:32:00Z"
}
```

Client → Server messages are **not** used for state changes (those remain REST — TRD §8's rationale). The only client→server WebSocket traffic is protocol-level: `ping`/`pong` frames for liveness (standard `gorilla/websocket` control frames, not application-level JSON) and connection close. This keeps the WebSocket message schema deliberately minimal — one direction, one shape — rather than growing into a second parallel API surface that duplicates REST semantics.

### 9.5 Reconnection & State Recovery [PRD §14: "disconnected clients should recover their current state when reconnecting"]

On reconnect, the client does **not** receive replayed missed events over the socket (no event buffering/replay queue in V4.0 scope — that's a materially larger feature, effectively at-least-once delivery guarantees, which PRD doesn't ask for). Instead: reconnection triggers a fresh `GET /api/v1/dashboard` REST call (§7.4) to resynchronise full state, and the WebSocket resumes providing incremental updates from that point forward. This satisfies PRD §14's requirement ("recover their current state") without building a message-replay system — full-resync-on-reconnect is the simpler design and is indistinguishable from replay to the end user, since the dashboard is small (bounded by team size) and cheap to refetch wholesale.

---

## 10. Migration & Refactor Plan — Board → Activity Pattern Clone

Concrete execution steps for TRD §5.2's "structural clone" recommendation:

1. Copy `board_handler.go` → `activity_handler.go`; copy `board_service.go` → `activity_service.go`; copy `board_repo.go` → `activity_repo.go`.
2. Replace `Board`-specific fields (`title`, `owner_id`) with `Activity` fields (§3). Retain the ownership-scoping pattern verbatim — `WHERE user_id = ?` scoping is identical in shape to `Board`'s `WHERE owner_id = ?`.
3. Replace generic CRUD routes with the singleton-resource routes from TRD §7.3 (`/activities/current`, not `/activities/:id`) — this is the one structural deviation from the Board pattern, per TRD §7.3's reasoning.
4. Add the one-active-per-user unique index (§4.1) — Board has no equivalent constraint; this is new.
5. Wire `ActivityEventRepository.Append` calls at each state transition (§4, §6.3).
6. Write the concurrency test closing TD-04 (§4.1) before merging.
7. Mark `board_*.go` files with a `// Deprecated: reference pattern superseded by Activity. See TRD.md TD-01.` comment — do not delete yet (TRD §5.2/§14).

---

## 11. Testing Mechanics

### 11.1 Unit Tests — Service Layer

Services depend on interfaces (§2) — unit tests substitute in-memory fakes, no database:

```go
type fakeActivityRepo struct {
    current map[uint64]*models.Activity
}
func (f *fakeActivityRepo) GetCurrent(ctx context.Context, userID uint64) (*models.Activity, error) {
    a, ok := f.current[userID]
    if !ok { return nil, ErrNotFound }
    return a, nil
}
// ... Upsert, Complete similarly
```

Table-driven, per Go convention:

```go
func TestActivityService_Start(t *testing.T) {
    cases := []struct{
        name        string
        description string
        wantErr     error
    }{
        {"valid description", "Writing tests", nil},
        {"empty description", "", ErrValidation},
        {"description too long", strings.Repeat("a", 281), ErrValidation},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) { /* ... */ })
    }
}
```

### 11.2 Integration Tests — Repository/Route Layer

Existing convention (accepted): `httptest` + real Postgres gated by `TEST_DATABASE_URL`, migrations run against a test database, fixtures seeded per test. Extend this pattern for `activities`/`activity_events` tables — do not introduce a second, different integration-test approach for the new domain; consistency across test suites matters more than any marginal improvement a new approach might offer.

### 11.3 Concurrency Test — Closing TD-04

```go
func TestActivityService_Start_ConcurrentCreate(t *testing.T) {
    // requires TEST_DATABASE_URL — exercises the real unique partial index,
    // not the fake repo, since the fake can't reproduce a DB-level race.
    var wg sync.WaitGroup
    results := make([]error, 2)
    for i := 0; i < 2; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            _, results[i] = service.Start(ctx, testUserID, fmt.Sprintf("activity %d", i))
        }(i)
    }
    wg.Wait()
    // Assert: both calls succeed (one via insert, one via retry-upsert-on-conflict
    // per §4), and exactly one active `activities` row exists for testUserID
    // afterward — not that one call errors, since §4's retry logic is designed
    // to make the race invisible to the caller.
}
```

### 11.4 WebSocket Tests [V4.0]

Use `httptest.NewServer` + a real `websocket.Dial` client in-process (standard approach for `gorilla/websocket`/`nhooyr.io/websocket` testing — avoid mocking the socket itself, since the value of these tests is exercising the actual upgrade handshake and frame handling). Minimum coverage before V4.0 is considered done: connect/register, broadcast reaches a second connected client, disconnect cleanly removes from `Hub.connections`, reconnect triggers correct dashboard resync per §9.5.

---

## 12. CI/CD — Additions for V2.0+

Existing GitHub Actions pipeline (gofmt, vet, tests, Docker build/publish) needs one addition once `TEST_DATABASE_URL`-gated tests grow to include the concurrency test (§11.3): confirm the CI Postgres service container is provisioned with enough connection headroom that a concurrency test's deliberate simultaneous connections don't get starved by CI's own connection pool limits — a flaky concurrency test in CI is worse than no concurrency test, because it teaches the team to ignore red builds.

---

## 13. Coding Conventions

- Package-per-aggregate (TRD §3.2) — enforced at review, not just documented.
- Error sentinels, not ad-hoc strings (§6.2).
- No `*gin.Context` below the handler layer (§1).
- All timestamps stored and transmitted as UTC; `RFC3339` on the wire (§7).
- Every new env var gets a `.env.example` entry in the same commit (TRD §9.2).
- Every new domain invariant enforced by application logic that *can* race (§4.1's pattern) must also be enforced at the database level — the application check is UX, the DB constraint is correctness.

---

## 14. Open Items Requiring Sign-Off (Mirrors TRD §13, Implementation-Level Additions)

1. **§5** — `maxActivityDescriptionLen = 280` is an invented default; needs product sign-off before V2.0 ships, not after.
2. **§9.3** — Global (non-team-scoped) broadcast is correct for MVP but is explicitly named as needing redesign the moment PRD §21 (Teams) is scheduled — flagging now so it's not rediscovered as a surprise then.
3. **§9.5** — Full-resync-on-reconnect (vs. event replay) as the reconnection strategy — recommended, not yet validated against real usage patterns.

---

*Companion documents: PRD.md (product scope and rationale), TRD.md (architecture and technology decisions this document implements).*