# Pulseboard

Pulseboard is a Go API foundation. This milestone provides PostgreSQL-backed user registration, login, authenticated current-user access, profile management, presence/availability state, lightweight activity tracking, and owner-only boards. Pulses, UI, teams, sharing, and WebSockets are intentionally not implemented yet.

## Run locally

1. Copy `.env.example` to `.env` and replace `JWT_SECRET` with a long random value.
2. Start PostgreSQL and the API with `docker compose up --build`.
3. The API is available at `http://localhost:8080`. Run automated checks with `docker compose --profile test run --rm test`.

The server requires `DATABASE_URL` and `JWT_SECRET`; it applies embedded, versioned SQL migrations before serving requests.

Repository sync is enforced in CI by the script `scripts/check_repo_sync.sh`, which validates that the runtime, Docker, workflow, and documentation files remain aligned.

Public registration is limited to 5 requests per minute per IP and login to 10 requests per minute per IP. A limited request returns `429` with a `Retry-After` header.

## API

`GET /health` checks process health. `GET /ready` checks PostgreSQL connectivity.

`POST /api/auth/register` and `POST /api/auth/login` accept `{"email":"user@example.com","password":"six-or-more-characters"}`. Login returns a JWT. Send it as `Authorization: Bearer <token>` to call `GET /api/me`, `PATCH /api/me`, `DELETE /api/me`, `GET /api/me/activities`, `POST /api/me/activities`, or `POST /api/me/activities/complete`.

Authenticated board endpoints live under `/api/boards`. Create with `POST /api/boards` using `{"title":"Launch Plan","description":"optional notes"}`. Use `GET /api/boards`, `GET /api/boards/{id}`, `PATCH /api/boards/{id}`, and `DELETE /api/boards/{id}` for owner-only board access. Deleted boards are soft-deleted and hidden from API responses.

There is deliberately no public user-list API. Profile updates support a lightweight `display_name`, `status_message`, `presence`, and `availability` contract for future dashboard usage, and activity endpoints let a user manage their current work state.

## API documentation

Fetch the OpenAPI document from `http://localhost:8080/openapi.yaml`. Import `docs/pulseboard.postman_collection.json` into Postman; its login request stores the returned JWT in the collection `token` variable, and its create-board request stores the returned board ID in the collection `boardId` variable.