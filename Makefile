include .env
export

migrate-up:
	migrate -path migrations -database "${DATABASE_URL}" up

migrate-down:
	migrate -path migrations -database "${DATABASE_URL}" down 1

migrate-test-up:
	migrate -path migrations -database "${TEST_DATABASE_URL}" up

migrate-test-down:
	migrate -path migrations -database "${TEST_DATABASE_URL}" down 1

test-storage:
	go test ./internal/storage/postgres -v