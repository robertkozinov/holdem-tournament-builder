-include .env
export

.PHONY: migrate-up migrate-down migrate-test-up migrate-test-down test-storage test-unit run

migrate-up:
	@test -n "${DATABASE_URL}" || (printf '%s\n' 'DATABASE_URL is required' >&2; exit 1)
	@migrate -path migrations -database "${DATABASE_URL}" up

migrate-down:
	@test -n "${DATABASE_URL}" || (printf '%s\n' 'DATABASE_URL is required' >&2; exit 1)
	@migrate -path migrations -database "${DATABASE_URL}" down 1

migrate-test-up:
	@test -n "${TEST_DATABASE_URL}" || (printf '%s\n' 'TEST_DATABASE_URL is required' >&2; exit 1)
	@migrate -path migrations -database "${TEST_DATABASE_URL}" up

migrate-test-down:
	@test -n "${TEST_DATABASE_URL}" || (printf '%s\n' 'TEST_DATABASE_URL is required' >&2; exit 1)
	@migrate -path migrations -database "${TEST_DATABASE_URL}" down 1

test-storage:
	@test -n "${TEST_DATABASE_URL}" || (printf '%s\n' 'TEST_DATABASE_URL is required' >&2; exit 1)
	@test -n "${TEST_REDIS_URL}" || (printf '%s\n' 'TEST_REDIS_URL is required' >&2; exit 1)
	go test ./internal/storage/postgres ./internal/storage/redis -race -count=1

test-unit:
	env -u TEST_DATABASE_URL -u TEST_REDIS_URL go test ./... -race -count=1

run:
	go run ./cmd/server
