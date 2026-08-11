#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

check_file() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "Missing required file: $path"
    exit 1
  fi
}

check_contains() {
  local file="$1"
  local needle="$2"
  if ! grep -Fq "$needle" "$file"; then
    echo "Expected to find '$needle' in $file"
    exit 1
  fi
}

check_file Dockerfile
check_file compose.yaml
check_file .env.example
check_file README.md
check_file docs/openapi.yaml
check_file .github/workflows/ci.yml
check_file .github/workflows/docker-publish.yml

check_contains compose.yaml 'ports: ["8080:8080"]'
check_contains compose.yaml 'JWT_SECRET: ${JWT_SECRET:?JWT_SECRET must be set}'
check_contains compose.yaml 'wget -qO- http://localhost:8080/health'
check_contains .github/workflows/ci.yml 'go vet ./...'
check_contains .github/workflows/ci.yml 'go test ./...'
check_contains .github/workflows/ci.yml 'docker build -t pulseboard .'
check_contains .github/workflows/docker-publish.yml 'docker/build-push-action@v6'
check_contains .github/workflows/docker-publish.yml 'context: .'
check_contains Dockerfile 'EXPOSE 8080'
check_contains .env.example 'PORT=8080'
check_contains README.md 'docker compose'

openapi_version=$(grep -E '^  version: ' docs/openapi.yaml | head -n 1 | tr -d '[:space:]' | cut -d: -f2)
if [[ -z "$openapi_version" ]]; then
  echo "Could not determine OpenAPI version from docs/openapi.yaml"
  exit 1
fi

check_contains docs/openapi.yaml "version: $openapi_version"

echo "Repository sync checks passed."
