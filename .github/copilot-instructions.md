# Pulseboard repository instructions

When making changes in this repository, treat every code change as a full-stack change.

Always review and update the relevant project infrastructure and documentation so the repository stays consistent:

- Dockerfile and compose.yaml for runtime/container changes
- .github/workflows/ci.yml and .github/workflows/docker-publish.yml for build, test, or deployment changes
- .env.example and README.md for environment or setup changes
- docs and OpenAPI-related files when APIs or request/response behavior change

If a change introduces a new dependency, env var, service, port, startup command, or binary path, update the related config and docs immediately.

For this project, keep the following expectations in mind:
- It is a Go API service.
- The server is built from cmd/server.
- Local development uses docker compose.
- CI validates formatting, vetting, tests, and Docker build health.
- The app depends on PostgreSQL and exposes port 8080.

When you modify code, prefer changes that keep local development, tests, and deployment aligned without manual follow-up.

Strict rule: do not change runtime behavior, dependencies, ports, env vars, startup commands, or API contracts without updating the related Docker, Compose, CI, and documentation files in the same change.
