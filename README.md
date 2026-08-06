# Pulseboard

Pulseboard is a Go API foundation. This milestone provides PostgreSQL-backed user registration, login, and authenticated current-user access. Boards, pulses, UI, and WebSockets are intentionally not implemented yet.

## Run locally

1. Copy `.env.example` to `.env` and replace `JWT_SECRET` with a long random value.
2. Start PostgreSQL and the API with `docker compose up --build`.
3. The API is available at `http://localhost:8080`. Run automated checks with `docker compose --profile test run --rm test`.

The server requires `DATABASE_URL` and `JWT_SECRET`; it applies embedded, versioned SQL migrations before serving requests.

Public registration is limited to 5 requests per minute per IP and login to 10 requests per minute per IP. A limited request returns `429` with a `Retry-After` header.

## API

`GET /health` checks process health. `GET /ready` checks PostgreSQL connectivity.

`POST /api/auth/register` and `POST /api/auth/login` accept `{"email":"user@example.com","password":"six-or-more-characters"}`. Login returns a JWT. Send it as `Authorization: Bearer <token>` to call `GET /api/me`.

There is deliberately no public user-list API.

## API documentation

Fetch the OpenAPI document from `http://localhost:8080/openapi.yaml`. Import `docs/pulseboard.postman_collection.json` into Postman; its login request stores the returned JWT in the collection `token` variable for the authenticated profile request.
